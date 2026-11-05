package push

import (
	"context"
	"time"
)

// SyncWorker 只执行一个同步候选；没有后台循环、并发调度或自动重试。
// 仓储固定为真实实现，避免装配一个在事务提交前发布票据的替身。
type SyncWorker struct {
	repository *JobRepository
	clock      func() time.Time
	ttl        time.Duration
	dispatch   SyncDispatch
}

func NewSyncWorker(repository *JobRepository, clock func() time.Time, ttl time.Duration, dispatch SyncDispatch) *SyncWorker {
	return &SyncWorker{repository: repository, clock: clock, ttl: ttl, dispatch: dispatch}
}

// SyncStepResult 不携带租约或尝试票据。Dispatched 表示调用曾开始；
// Applied 为 false 时不得声称结果已持久化，也不能据此立即再次调用 adapter。
type SyncStepResult struct {
	Dispatched bool
	Applied    bool
}

// Step 串行组合已提交的短事务与一次外部调用。Check 返回后仍有 TOCTOU，
// 不能撤回已开始的派发；Apply 的 fencing 仅能隔离迟到结果。
func (w *SyncWorker) Step(ctx context.Context) (result SyncStepResult, err error) {
	if ctx == nil || w == nil || w.repository == nil || w.repository.store == nil || w.clock == nil || w.dispatch == nil || w.ttl <= 0 {
		return result, ErrJobInput
	}
	// 每阶段读取新时刻；无效或倒退即失败关闭，不能夹紧到旧值后继续发送。
	var previous time.Time
	nextNow := func() (time.Time, error) {
		if err := ctx.Err(); err != nil {
			return time.Time{}, err
		}
		now := w.clock().UTC()
		if !jobTimeValid(now) || (!previous.IsZero() && now.Before(previous)) {
			return time.Time{}, ErrJobInput
		}
		previous = now
		return now, ctx.Err()
	}
	now, err := nextNow()
	if err != nil {
		return result, err
	}
	until := now.Add(w.ttl)
	if !jobTimeValid(until) || !until.After(now) {
		return result, ErrJobInput
	}
	held, err := w.repository.ReserveSync(ctx, now, w.ttl, 1)
	if err != nil || len(held) == 0 {
		return result, err
	}
	now, err = nextNow()
	if err != nil {
		return result, err
	}
	ticket, err := w.repository.BeginSync(ctx, held[0].Lease, now)
	if err != nil || ticket == nil {
		return result, err
	}
	now, err = nextNow()
	if err != nil {
		return result, err
	}
	current, err := w.repository.CheckSyncAttempt(ctx, held[0].Lease, *ticket, now)
	if err != nil || !current {
		return result, err
	}
	// Check 的只读事务已完成；adapter 不持有任何数据库事务或票据。
	if err = ctx.Err(); err != nil {
		return result, err
	}
	j := ticket.snapshot
	result.Dispatched = true
	outcome, dispatchErr := w.dispatch.Dispatch(ctx, SyncEnvelope{
		JobKey: j.ID, Purpose: "sync", Provider: "fcm", TargetChangeSeq: j.TargetChangeSeq, ExpiresAt: j.ExpiresAt,
	})
	// 原始 adapter 错误可能携带凭据，不能包装、记录或向外传播。
	if dispatchErr != nil || !outcome.valid() {
		outcome = SyncUnknownDelivery
	}
	now, err = nextNow()
	if err != nil {
		return result, err
	}
	// 不主动释放或续租；取消、失败与 no-op 均留给既有到期恢复规则。
	result.Applied, err = w.repository.ApplySyncResult(ctx, *ticket, outcome, now)
	return result, err
}
