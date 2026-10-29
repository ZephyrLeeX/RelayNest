package messages

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"relaynest/server/internal/bots"
	"relaynest/server/internal/storage"
	read "relaynest/server/internal/sync"
)

// 测试中的可信事务拥有者只在真实COMMIT成功后发布，绝非新增生产授权入口。
func editOwned(ctx context.Context, s *storage.Store, r *Repository, i bots.Identity, in EditTextButtonsInput) (Edited, error) {
	var candidate Edited
	err := s.Write(ctx, func(tx *sql.Tx) error {
		var err error
		candidate, err = r.EditApplicationTx(ctx, tx, i, in)
		return err
	})
	if err != nil {
		return Edited{}, err
	}
	return candidate, nil
}

func editSeed(t *testing.T, f *fixture) (Created, EditTextButtonsInput) {
	t.Helper()
	out, err := f.r.CreateIdempotentApplication(messageContext, f.identity, idemKey, request(), nil)
	if err != nil {
		t.Fatal(err)
	}
	return out, EditTextButtonsInput{BotID: f.a.Bot.ID, MessageID: out.Message.ID, ExpectedRevision: 1, Format: "plain", Text: "updated"}
}

// 同一只读Tx覆盖全部持久表及sqlite_sequence；不只用计数掩盖正文/代次/快照变化。
func editSnapshot(t *testing.T, s *storage.Store) map[string][]string {
	t.Helper()
	out := make(map[string][]string)
	err := s.Read(messageContext, func(tx *sql.Tx) error {
		rows, err := tx.Query(`SELECT name FROM sqlite_schema WHERE type='table' ORDER BY name`)
		if err != nil {
			return err
		}
		var tables []string
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				rows.Close()
				return err
			}
			tables = append(tables, name)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		if err := rows.Close(); err != nil {
			return err
		}
		for _, table := range tables {
			rows, err := tx.Query(`SELECT * FROM "` + strings.ReplaceAll(table, `"`, `""`) + `"`)
			if err != nil {
				return err
			}
			cols, err := rows.Columns()
			if err != nil {
				rows.Close()
				return err
			}
			out[table] = []string{}
			for rows.Next() {
				values, ptrs := make([]any, len(cols)), make([]any, len(cols))
				for n := range values {
					ptrs[n] = &values[n]
				}
				if err := rows.Scan(ptrs...); err != nil {
					rows.Close()
					return err
				}
				data, err := json.Marshal(values)
				if err != nil {
					rows.Close()
					return err
				}
				out[table] = append(out[table], string(data))
			}
			if err := rows.Err(); err != nil {
				rows.Close()
				return err
			}
			if err := rows.Close(); err != nil {
				return err
			}
			sort.Strings(out[table])
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func editReadState(t *testing.T, f *fixture) read.ReadState {
	t.Helper()
	var out read.ReadState
	if err := f.other.Read(messageContext, func(tx *sql.Tx) error {
		var err error
		out, err = read.GetReadStateTx(messageContext, tx, f.a.Bot.ID)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestEditTextButtonsAtomicPreservation(t *testing.T) {
	f := setupMessages(t)
	source, _ := editSeed(t, f)
	inCreate := replyRequest(source.Message.ID)
	reply, err := f.r.CreateIdempotentApplication(messageContext, f.identity, "11111111-1111-4111-8111-111111111111", inCreate, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := idemAdmin(f, f.a.Bot.ID, "22222222-2222-4222-8222-222222222222", replyRequest(reply.Message.ID)); err != nil {
		t.Fatal(err)
	}
	const aid = "33333333-3333-4333-8333-333333333333"
	attachmentRefSeed(t, f, aid, f.a.Bot.ID, "file")
	attachmentRefLink(t, f, reply.Message.ID, aid, "file", 7)
	// 保留已过期原引用，不能编辑时恢复FK或查询已不存在源正文。
	execMessages(t, f.s, `DELETE FROM messages WHERE id=?`, source.Message.ID)
	before := editSnapshot(t, f.other)
	unread := editReadState(t, f)
	callback, link := "  α\ncommand / untouched  ", "custom+scheme:opaque?x=1"
	in := EditTextButtonsInput{BotID: f.a.Bot.ID, MessageID: reply.Message.ID, ExpectedRevision: 1,
		Format: "markdown", Text: "  new **正文**\n", Buttons: [][]Button{{{ID: "c", Label: "执行", CallbackData: &callback}, {ID: "u", Label: "跳转", URL: &link}}}}
	stamp := time.Date(2026, 10, 30, 15, 1, 2, 123, time.FixedZone("offset", 8*3600))
	f.r.now = func() time.Time { return stamp }
	out, err := editOwned(messageContext, f.s, f.r, f.identity, in)
	if err != nil {
		t.Fatal(err)
	}
	m := out.Message
	if m.ID != reply.Message.ID || m.BotSeq != reply.Message.BotSeq || m.CreatedAt != reply.Message.CreatedAt ||
		!reflect.DeepEqual(m.CreatedChangeSeq, reply.Message.CreatedChangeSeq) || m.Sender != Application ||
		m.Revision != 2 || m.Format != in.Format || m.Text != in.Text || !m.UpdatedAt.Equal(stamp) || m.UpdatedAt.Location() != time.UTC ||
		m.ReplyToMessageID != nil || m.OriginalReplyToMessageID == nil || *m.OriginalReplyToMessageID != source.Message.ID ||
		!reflect.DeepEqual(m.AttachmentRefs, []AttachmentReference{{ID: aid, Kind: "file"}}) || out.LastChangeSeq <= reply.LastChangeSeq {
		t.Fatalf("edit changed immutable state or lost current content: %+v", out)
	}
	var gotButtons [][]Button
	if err := json.Unmarshal(m.ButtonsJSON, &gotButtons); err != nil || !reflect.DeepEqual(gotButtons, in.Buttons) {
		t.Fatal("button bytes changed", err)
	}
	after := editSnapshot(t, f.other)
	for table, rows := range before {
		if table != "messages" && table != "changes" && table != "sqlite_sequence" && !reflect.DeepEqual(rows, after[table]) {
			t.Fatalf("unexpected table mutation %s", table)
		}
	}
	if !reflect.DeepEqual(unread, editReadState(t, f)) {
		t.Fatal("edit increased unread/changed read state")
	}
	if err := f.other.Read(messageContext, func(tx *sql.Tx) error {
		current, err := f.r.GetHistoryMessageTx(messageContext, tx, f.a.Bot.ID, m.ID)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(current, m) {
			t.Fatal("current history differs from committed edit")
		}
		var typ, id string
		var rev int64
		if err := tx.QueryRow(`SELECT type,entity_id,revision FROM changes WHERE change_seq=?`, out.LastChangeSeq).Scan(&typ, &id, &rev); err != nil {
			return err
		}
		if typ != string("message.upsert") || id != m.ID || rev != 2 {
			t.Fatal("incorrect edit metadata")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	// 原幂等创建结果仍旧，不因当前编辑重新生成receipt。
	replay, err := f.r.CreateIdempotentApplication(messageContext, f.identity, "11111111-1111-4111-8111-111111111111", inCreate, nil)
	if err != nil || !reflect.DeepEqual(replay, reply) {
		t.Fatal("edit rewrote creation receipt", err)
	}
	// 同值成功CAS也提升revision，不能偷偷当作no-op。
	in.ExpectedRevision = 2
	same, err := editOwned(messageContext, f.s, f.r, f.identity, in)
	if err != nil || same.Message.Revision != 3 || same.LastChangeSeq <= out.LastChangeSeq {
		t.Fatal("same-value CAS not versioned", err)
	}
}

func TestEditTextButtonsContentVariants(t *testing.T) {
	for _, kind := range []string{"buttons-only", "attachment-only", "empty"} {
		t.Run(kind, func(t *testing.T) {
			f := setupMessages(t)
			_, in := editSeed(t, f)
			in.Text = ""
			switch kind {
			case "buttons-only":
				empty := ""
				in.Buttons = [][]Button{{{ID: "callback", Label: "empty data allowed", CallbackData: &empty}}}
			case "attachment-only":
				const aid = "44444444-4444-4444-8444-444444444444"
				attachmentRefSeed(t, f, aid, f.a.Bot.ID, "image")
				attachmentRefLink(t, f, in.MessageID, aid, "image", 0)
			}
			before := editSnapshot(t, f.other)
			out, err := editOwned(messageContext, f.s, f.r, f.identity, in)
			if kind == "empty" {
				if !errors.Is(err, ErrInvalid) || !reflect.DeepEqual(out, Edited{}) || !reflect.DeepEqual(before, editSnapshot(t, f.other)) {
					t.Fatal("accepted empty content", err)
				}
			} else if err != nil || out.Message.Text != "" || out.Message.Revision != 2 {
				t.Fatal("valid non-text rejected", err)
			}
		})
	}
}

func TestEditTextButtonsRevisionAndCorruption(t *testing.T) {
	f := setupMessages(t)
	_, in := editSeed(t, f)
	execMessages(t, f.s, `UPDATE messages SET revision=? WHERE id=?`, int64(math.MaxInt64-1), in.MessageID)
	in.ExpectedRevision = math.MaxInt64 - 1
	out, err := editOwned(messageContext, f.s, f.r, f.identity, in)
	if err != nil || out.Message.Revision != math.MaxInt64 {
		t.Fatal("last revision failed", err)
	}
	before := editSnapshot(t, f.other)
	for _, expected := range []int64{1, math.MaxInt64 - 1, math.MaxInt64} {
		in.ExpectedRevision = expected
		out, err = editOwned(messageContext, f.s, f.r, f.identity, in)
		want := ErrRevisionConflict
		if expected == math.MaxInt64 {
			want = ErrRevisionExhausted
		}
		if !errors.Is(err, want) || !reflect.DeepEqual(out, Edited{}) || !reflect.DeepEqual(before, editSnapshot(t, f.other)) {
			t.Fatal("revision mismatch/overflow wrote", err)
		}
	}
	for _, mode := range []string{"creation-seq", "created-utc", "updated-utc", "clock"} {
		t.Run(mode, func(t *testing.T) {
			f := setupMessages(t)
			_, in := editSeed(t, f)
			switch mode {
			case "creation-seq":
				// SQL允许创建事务的临时NULL，损坏的已提交行必须失败关闭。
				execMessages(t, f.s, `DROP TRIGGER messages_creation_seq_immutable;UPDATE messages SET created_change_seq=NULL WHERE id=?`, in.MessageID)
			case "created-utc":
				execMessages(t, f.s, `UPDATE messages SET created_at='9999-12-31T23:59:59-08:00' WHERE id=?`, in.MessageID)
			case "updated-utc":
				execMessages(t, f.s, `UPDATE messages SET updated_at='9999-12-31T23:59:59-08:00' WHERE id=?`, in.MessageID)
			case "clock":
				f.r.now = func() time.Time { return time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC) }
			}
			before := editSnapshot(t, f.other)
			out, err := editOwned(messageContext, f.s, f.r, f.identity, in)
			if err == nil || !reflect.DeepEqual(out, Edited{}) || !reflect.DeepEqual(before, editSnapshot(t, f.other)) {
				t.Fatal("corrupt state accepted", err)
			}
		})
	}
}

func TestEditTextButtonsTwoStoreCAS(t *testing.T) {
	f := setupMessages(t)
	_, in := editSeed(t, f)
	before := editSnapshot(t, f.other)
	const count = 12
	start := make(chan struct{})
	type result struct {
		out Edited
		err error
	}
	results := make(chan result, count)
	var wg sync.WaitGroup
	for n := 0; n < count; n++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			<-start
			s, r := f.s, f.r
			if n%2 == 1 {
				s, r = f.other, New(f.other)
			}
			candidate := in
			candidate.Text = fmt.Sprintf("candidate %d", n)
			out, err := editOwned(messageContext, s, r, f.identity, candidate)
			results <- result{out, err}
		}(n)
	}
	close(start)
	wg.Wait()
	close(results)
	winners, conflicts := 0, 0
	for got := range results {
		if got.err == nil {
			winners++
			if got.out.Message.Revision != 2 {
				t.Fatal("wrong winner")
			}
		} else {
			if !errors.Is(got.err, ErrRevisionConflict) || !reflect.DeepEqual(got.out, Edited{}) {
				t.Fatal("nonconflict or leaked candidate", got.err)
			}
			conflicts++
		}
	}
	after := editSnapshot(t, f.other)
	if winners != 1 || conflicts != count-1 || len(after["changes"]) != len(before["changes"])+1 {
		t.Fatal("CAS did not serialize", winners, conflicts)
	}
	for table, rows := range before {
		if table != "messages" && table != "changes" && table != "sqlite_sequence" && !reflect.DeepEqual(rows, after[table]) {
			t.Fatal("CAS affected", table)
		}
	}
}
