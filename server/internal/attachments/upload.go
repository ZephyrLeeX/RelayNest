package attachments

import (
	"context"
	"database/sql"
	"io"
	"time"
)

// Uploader 不解释 HTTP/Bearer/实例头。write 每次须在同事务复核真实授权并提交。
type Uploader struct {
	repo   *Repository
	files  *Files
	limits Limits
	write  AuthorizedWrite
}

func NewUploader(repo *Repository, files *Files, limits Limits, write AuthorizedWrite) (*Uploader, error) {
	if repo == nil || repo.store == nil || files == nil || files.root == nil || write == nil {
		return nil, ErrInvalid
	}
	if _, e := limits.limit("file"); e != nil {
		return nil, e
	}
	return &Uploader{repo, files, limits, write}, nil
}
func (u *Uploader) Upload(ctx context.Context, in Input, src io.Reader) (Attachment, error) {
	if !in.valid() || src == nil {
		return Attachment{}, ErrInvalid
	}
	limit, e := u.limits.limit(in.Kind)
	if e != nil {
		return Attachment{}, e
	}
	var a Attempt
	// 初次授权与 staging 提交成功后才允许读取上传流。
	e = u.write(ctx, func(tx *sql.Tx) error { var err error; a, err = u.repo.BeginTx(ctx, tx, in); return err })
	if e != nil {
		return Attachment{}, e
	}
	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	// 对 HTTP Body/管道等可关闭流，取消或续租失败时主动解除阻塞读取。
	// 普通 io.Reader 无通用中断协议，调用方须保证其 Read 最终返回。
	stopRead := context.AfterFunc(streamCtx, func() {
		if closer, ok := src.(io.Closer); ok {
			_ = closer.Close()
		}
	})
	defer stopRead()
	stop := make(chan struct{})
	done := make(chan error, 1)
	// 流式期间不持事务；reader 暂停时仍续租，任意 Reader 的阻塞由调用方关闭来解除。
	go func() {
		interval := u.repo.lease / 3
		if interval <= 0 {
			interval = time.Millisecond
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				done <- nil
				return
			case <-streamCtx.Done():
				done <- streamCtx.Err()
				return
			case <-ticker.C:
				if err := u.repo.Renew(streamCtx, a); err != nil {
					cancel()
					done <- err
					return
				}
			}
		}
	}()
	size, digest, streamErr := u.files.write(streamCtx, a.Token, src, limit)
	close(stop)
	renewErr := <-done
	stopRead()
	cancel()
	if renewErr != nil {
		return Attachment{}, renewErr
	}
	if streamErr != nil {
		return Attachment{}, streamErr
	}
	if e = ctx.Err(); e != nil {
		return Attachment{}, e
	}
	if !a.Replay {
		if e = u.files.publish(a.Token); e != nil {
			return Attachment{}, e
		}
	}
	var candidate Attachment
	// 文件持久化后再授权并标 ready；DB 失败保留 staging 与文件恢复证据。
	e = u.write(ctx, func(tx *sql.Tx) error {
		var err error
		candidate, err = u.repo.CompleteTx(ctx, tx, a, size, digest)
		return err
	})
	if e != nil {
		return Attachment{}, e
	}
	if a.Replay {
		_ = u.files.root.Remove(a.Token + ".part")
	}
	return candidate, nil
}
