package attachments

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"relaynest/server/internal/storage"
)

type Repository struct {
	store *storage.Store
	now   func() time.Time
	lease time.Duration
}

func New(store *storage.Store) *Repository { return &Repository{store, time.Now, defaultLease} }

const columns = `id,bot_id,kind,filename,media_type,coalesce(storage_key,''),state,coalesce(size,0),sha256,created_at`

func scan(row *sql.Row) (Attachment, error) {
	var a Attachment
	err := row.Scan(&a.ID, &a.BotID, &a.Kind, &a.Filename, &a.MediaType, &a.StorageKey, &a.State, &a.Size, &a.SHA256, &a.CreatedAt)
	return a, err
}

// ReadyTx 不开启/提交事务，也不授权；下载和消息关联须在真实授权事务中使用。
func (r *Repository) ReadyTx(ctx context.Context, tx *sql.Tx, id string) (Attachment, error) {
	if tx == nil {
		return Attachment{}, ErrInvalid
	}
	a, e := scan(tx.QueryRowContext(ctx, `SELECT `+columns+` FROM attachments WHERE id=? AND state='ready'`, id))
	if errors.Is(e, sql.ErrNoRows) {
		e = ErrNotReady
	}
	return a, e
}

// BeginTx 返回未提交候选；调用者必须传播错误，提交成功后才可开始读流。
func (r *Repository) BeginTx(ctx context.Context, tx *sql.Tx, in Input) (Attempt, error) {
	if tx == nil || !in.valid() || r.lease <= 0 {
		return Attempt{}, ErrInvalid
	}
	in.Key = strings.ToLower(in.Key)
	now := r.now()
	var a Attempt
	var until int64
	err := tx.QueryRowContext(ctx, `SELECT attachment_id,generation,active_token,lease_until FROM attachment_uploads WHERE identity=? AND key=?`, in.Identity, in.Key).Scan(&a.Attachment.ID, &a.Generation, &a.Token, &until)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return Attempt{}, err
	}
	if err == nil {
		a.Attachment, err = scan(tx.QueryRowContext(ctx, `SELECT `+columns+` FROM attachments WHERE id=?`, a.Attachment.ID))
		if err != nil {
			return Attempt{}, err
		}
		old := a.Attachment
		if old.BotID != in.BotID || old.Kind != in.Kind || old.Filename != in.Filename || old.MediaType != in.MediaType {
			return Attempt{}, ErrConflict
		}
		if old.State == "ready" {
			a.Replay = true
			token, randomErr := uuid.NewRandom()
			if randomErr != nil {
				return Attempt{}, randomErr
			}
			a.Token = token.String()
			_, err = tx.ExecContext(ctx, `INSERT INTO attachment_attempts(token,attachment_id,generation,created_at) VALUES(?,?,?,?)`, a.Token, a.Attachment.ID, a.Generation, now.UTC().Format(time.RFC3339Nano))
			return a, err
		}
		if old.State != "staging" {
			return Attempt{}, ErrNotReady
		}
		if until > now.UnixMilli() {
			return Attempt{}, ErrActive
		}
		if a.Generation == math.MaxInt64 {
			return Attempt{}, ErrFenced
		}
		a.Generation++
	} else {
		a.Generation = 1
		id, randomErr := uuid.NewRandom()
		if randomErr != nil {
			return Attempt{}, randomErr
		}
		a.Attachment = Attachment{ID: id.String(), BotID: in.BotID, Kind: in.Kind, Filename: in.Filename, MediaType: in.MediaType, State: "staging", CreatedAt: now.UTC().Format(time.RFC3339Nano)}
		_, err = tx.ExecContext(ctx, `INSERT INTO attachments(id,bot_id,kind,filename,media_type,state,created_at) VALUES(?,?,?,?,?,'staging',?)`, a.Attachment.ID, in.BotID, in.Kind, in.Filename, in.MediaType, a.Attachment.CreatedAt)
		if err != nil {
			return Attempt{}, err
		}
	}
	token, randomErr := uuid.NewRandom()
	if randomErr != nil {
		return Attempt{}, randomErr
	}
	a.Token = token.String()
	_, err = tx.ExecContext(ctx, `INSERT INTO attachment_uploads(identity,key,attachment_id,generation,active_token,lease_until) VALUES(?,?,?,?,?,?) ON CONFLICT(identity,key) DO UPDATE SET generation=excluded.generation,active_token=excluded.active_token,lease_until=excluded.lease_until`, in.Identity, in.Key, a.Attachment.ID, a.Generation, a.Token, now.Add(r.lease).UnixMilli())
	if err != nil {
		return Attempt{}, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO attachment_attempts(token,attachment_id,generation,created_at) VALUES(?,?,?,?)`, a.Token, a.Attachment.ID, a.Generation, now.UTC().Format(time.RFC3339Nano))
	return a, err
}
func one(res sql.Result, err error) error {
	if err != nil {
		return err
	}
	n, e := res.RowsAffected()
	if e != nil {
		return e
	}
	if n != 1 {
		return ErrFenced
	}
	return nil
}

// Renew 仅延长仍活动的同一代尝试；过期租约不能复活。
func (r *Repository) Renew(ctx context.Context, a Attempt) error {
	if a.Replay {
		return nil
	}
	return r.store.Write(ctx, func(tx *sql.Tx) error {
		now := r.now()
		return one(tx.ExecContext(ctx, `UPDATE attachment_uploads SET lease_until=? WHERE attachment_id=? AND generation=? AND active_token=? AND lease_until>? AND EXISTS(SELECT 1 FROM attachments WHERE id=? AND state='staging')`, now.Add(r.lease).UnixMilli(), a.Attachment.ID, a.Generation, a.Token, now.UnixMilli(), a.Attachment.ID))
	})
}

// CompleteTx 仅在文件持久化后调用；不提交，结果仍为候选。
func (r *Repository) CompleteTx(ctx context.Context, tx *sql.Tx, a Attempt, size int64, digest []byte) (Attachment, error) {
	if tx == nil || size < 0 || len(digest) != 32 {
		return Attachment{}, ErrInvalid
	}
	if a.Replay {
		old, e := r.ReadyTx(ctx, tx, a.Attachment.ID)
		if e != nil {
			return Attachment{}, e
		}
		if old.Size != size || !bytes.Equal(old.SHA256, digest) {
			return Attachment{}, ErrConflict
		}
		return old, nil
	}
	if err := one(tx.ExecContext(ctx, `UPDATE attachments SET state='ready',storage_key=?,size=?,sha256=? WHERE id=? AND state='staging' AND EXISTS(SELECT 1 FROM attachment_uploads WHERE attachment_id=? AND generation=? AND active_token=? AND lease_until>?)`, a.Token+".ready", size, digest, a.Attachment.ID, a.Attachment.ID, a.Generation, a.Token, r.now().UnixMilli())); err != nil {
		return Attachment{}, err
	}
	return r.ReadyTx(ctx, tx, a.Attachment.ID)
}
