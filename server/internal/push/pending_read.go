package push

import (
	"context"
	"database/sql"
	"time"

	"relaynest/server/internal/changes"
)

// ReadPendingTx 仅供可信服务端调用方传入现有事务，调用方负责事务最终成功后发布
// 候选；此函数不鉴权调用方、不开启或提交事务、不领取或修改待办。
// limit 按会话计数；按 device_id/session_id 升序截取，选中会话的全部到期待办
// 都参与最大水位归并。读取时间须能用认证存储的 Unix 纳秒无损表示。
func ReadPendingTx(ctx context.Context, tx *sql.Tx, now time.Time, limit int) ([]PendingDevice, error) {
	if tx == nil || ctx == nil || now.IsZero() || !time.Unix(0, now.UnixNano()).Equal(now) || limit < 1 || limit > MaxPendingSessions {
		return nil, ErrPendingInput
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// 同一语句、同一事务快照校验身份；不读取凭据或消息内容。
	rows, err := tx.QueryContext(ctx, `SELECT j.device_id,j.session_id,j.target_change_seq,j.next_attempt_at
 FROM push_jobs j JOIN sessions s ON s.id=j.session_id AND s.device_id=j.device_id
 JOIN administrator a ON a.id=1 AND a.auth_generation=s.auth_generation
 WHERE j.state='pending' AND s.client_type='android' AND s.revoked=0
 AND s.idle_expires_at>? AND s.absolute_expires_at>?
 ORDER BY j.device_id,j.session_id,j.id`, now.UnixNano(), now.UnixNano())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]PendingDevice, 0)
	count := 0
	for rows.Next() {
		var device, session, timestamp string
		var seq changes.Sequence
		if err := rows.Scan(&device, &session, &seq, &timestamp); err != nil {
			return nil, err
		}
		due, err := time.Parse(time.RFC3339Nano, timestamp)
		if err != nil || device == "" || session == "" || seq < 1 {
			return nil, ErrPendingData
		}
		// RFC3339Nano 的小数位长度可变，不能按字符串比较到期时间；SQLite 日期函数
		// 也不能保留认证所需的纳秒边界。这里流式判断，结果内存有界，扫描量可能较大。
		if due.After(now) {
			continue
		}
		last := len(result) - 1
		if last >= 0 && result[last].DeviceID == device {
			sessions := result[last].Sessions
			tail := len(sessions) - 1
			if sessions[tail].SessionID == session {
				if seq > sessions[tail].TargetChangeSeq {
					result[last].Sessions[tail].TargetChangeSeq = seq
				}
				continue
			}
		}
		// 不按 job 截断，否则一个会话的高水位可能在截断处丢失。
		if count == limit {
			break
		}
		candidate := PendingSession{SessionID: session, TargetChangeSeq: seq}
		if last >= 0 && result[last].DeviceID == device {
			result[last].Sessions = append(result[last].Sessions, candidate)
		} else {
			result = append(result, PendingDevice{DeviceID: device, Sessions: []PendingSession{candidate}})
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
