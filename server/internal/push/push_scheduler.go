package push

import (
	"context"
	"sync/atomic"
	"time"
)

// PushScheduler 只负责两个既有单步 worker 的内部串行周期；不装配生产后台或平台。
// 每个实例至多一个 Run，使用后不得复制；跨实例排他仍由数据库租约保证。
type PushScheduler struct {
	sync     *SyncWorker
	opush    *OPushAlertWorker
	interval time.Duration
	running  atomic.Bool
}

// NewPushScheduler 只保存可信内部依赖，不启动循环或选择默认参数。
func NewPushScheduler(sync *SyncWorker, opush *OPushAlertWorker, interval time.Duration) *PushScheduler {
	return &PushScheduler{sync: sync, opush: opush, interval: interval}
}

// Run 首周期立即执行，之后从两步正常完成时等待完整间隔，不追赶 tick 或自行重试。
// 在途 adapter 若忽略取消，必须等原 Step 返回；这里不启动可遗留外发的后台 goroutine。
func (s *PushScheduler) Run(ctx context.Context) error {
	if ctx == nil || s == nil || s.sync == nil || s.opush == nil || s.interval <= 0 {
		return ErrJobInput
	}
	// 两个依赖都须先完整装配，避免先发送 sync 后才发现 opush 装配无效。
	a, b := s.sync, s.opush
	if a.repository == nil || a.repository.store == nil || a.clock == nil || a.dispatch == nil || a.ttl <= 0 ||
		b.repository == nil || b.repository.store == nil || b.clock == nil || b.dispatch == nil || b.ttl <= 0 {
		return ErrJobInput
	}
	if !s.running.CompareAndSwap(false, true) {
		return ErrJobInput
	}
	// panic 沿调用栈传播；占用与 timer 仍须释放，不能恢复后继续发送。
	defer s.running.Store(false)
	var timer *time.Timer
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if _, err := a.Step(ctx); err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if _, err := b.Step(ctx); err != nil {
			return err
		}
		if timer == nil {
			timer = time.NewTimer(s.interval)
		} else {
			timer.Reset(s.interval)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		}
	}
}
