package push

import (
	"context"
	"time"
)

// OPushAlertWorker 只执行一个原资格完整 opush 提醒候选；没有后台循环、并发调度或自动重试。
// 仓储固定为真实实现，避免装配一个在事务提交前发布票据的替身。
type OPushAlertWorker struct {
	repository *JobRepository
	clock      func() time.Time
	ttl        time.Duration
	dispatch   OPushAlertDispatch
}

func NewOPushAlertWorker(repository *JobRepository, clock func() time.Time, ttl time.Duration, dispatch OPushAlertDispatch) *OPushAlertWorker {
	return &OPushAlertWorker{repository: repository, clock: clock, ttl: ttl, dispatch: dispatch}
}

// OPushAlertStepResult 不携带租约或尝试票据。Dispatched 表示调用曾开始；
// Applied 为 false 时不得声称结果已持久化，也不能据此立即再次调用 adapter。
type OPushAlertStepResult struct {
	Dispatched bool
	Applied    bool
}

// Step 串行组合已提交的短事务与一次外部调用。Check 返回后仍有 TOCTOU，
// 不能撤回已开始的派发；Apply 的 fencing 仅能隔离迟到结果。
func (w *OPushAlertWorker) Step(ctx context.Context) (result OPushAlertStepResult, err error) {
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
	held, err := w.repository.ReserveOPushAlerts(ctx, now, w.ttl, 1)
	if err != nil || len(held) == 0 {
		return result, err
	}
	now, err = nextNow()
	if err != nil {
		return result, err
	}
	ticket, err := w.repository.BeginAlert(ctx, held[0].Lease, now)
	if err != nil {
		return result, err
	}
	if ticket == nil {
		// Begin 拒绝只补做源／已读终止；用新时刻和完整原快照在独立事务重核。
		// 其他拒绝仍由既有到期恢复处理，不释放租约或立即重试。
		now, err = nextNow()
		if err != nil {
			return result, err
		}
		_, err = w.repository.DropAlertSourceRead(ctx, held[0].Lease, held[0].Job.Snapshot, now)
		return result, err
	}
	now, err = nextNow()
	if err != nil {
		return result, err
	}
	current, err := w.repository.CheckAlertAttempt(ctx, held[0].Lease, *ticket, now)
	if err != nil || !current {
		return result, err
	}
	// Check 的只读事务已完成；adapter 不持有任何数据库事务或票据。
	// Check 拒绝不补做 Drop；Begin 后的 DND／共享已读变化仍沿既有到期恢复。
	if err = ctx.Err(); err != nil {
		return result, err
	}
	j := ticket.snapshot
	result.Dispatched = true
	outcome, dispatchErr := w.dispatch.Dispatch(ctx, OPushAlertEnvelope{
		JobKey: j.ID, Purpose: "alert", Provider: "opush", ExpiresAt: alertAttemptExpiry(j),
	})
	// 原始 adapter 错误可能携带凭据，不能包装、记录或向外传播。
	if dispatchErr != nil || !outcome.valid() {
		outcome = AlertUnknownDelivery
	}
	now, err = nextNow()
	if err != nil {
		return result, err
	}
	// 不主动释放或续租；取消、失败与 no-op 均留给既有到期恢复规则。
	result.Applied, err = w.repository.ApplyAlertResult(ctx, *ticket, outcome, now)
	return result, err
}
