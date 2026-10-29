package messages

import (
	"database/sql"
	"encoding/json"
	"reflect"
	"testing"

	"relaynest/server/internal/storage"
)

func attachmentRefSeed(t *testing.T, f *fixture, id, bot, kind string) {
	t.Helper()
	execMessages(t, f.s, `INSERT INTO attachments(id,bot_id,kind,filename,media_type,state,created_at) VALUES(?,?,?,'private-filename','application/octet-stream','missing','2026-01-01T00:00:00Z')`, id, bot, kind)
}
func attachmentRefLink(t *testing.T, f *fixture, message, id, kind string, position int) {
	t.Helper()
	execMessages(t, f.s, `INSERT INTO message_attachments(message_id,attachment_id,kind,position) VALUES(?,?,?,?)`, message, id, kind, position)
}
func TestMessageAttachmentRefsStableReadAndCascade(t *testing.T) {
	f := setupMessages(t)
	ms := historySeed(t, f, 3)
	const image = "11111111-1111-4111-8111-111111111111"
	const file = "22222222-2222-4222-8222-222222222222"
	attachmentRefSeed(t, f, image, f.a.Bot.ID, "image")
	attachmentRefSeed(t, f, file, f.a.Bot.ID, "file")
	// 故意以相反顺序插入；missing没有物理文件，仍是历史公开引用。
	attachmentRefLink(t, f, ms[1].ID, file, "file", 7)
	attachmentRefLink(t, f, ms[1].ID, image, "image", 2)
	want := []AttachmentReference{{image, "image"}, {file, "file"}}
	page := historyPage(t, f, f.a.Bot.ID, nil, 2)
	if len(page.Items) != 2 || !page.HasMore || page.Items[0].AttachmentRefs != nil || !reflect.DeepEqual(page.Items[1].AttachmentRefs, want) {
		t.Fatalf("page: %+v", page)
	}
	before := persisted(t, f.s)
	reopened, err := storage.Open(messageContext, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if err := reopened.Read(messageContext, func(tx *sql.Tx) error {
		m, e := New(reopened).GetHistoryMessageByIDTx(messageContext, tx, ms[1].ID)
		if e == nil && !reflect.DeepEqual(m.AttachmentRefs, want) {
			t.Fatal(m)
		}
		return e
	}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, persisted(t, f.s)) {
		t.Fatal("read wrote business state")
	}
	// 关系只属读模型，不成为创建结果JSON的新字段或附件输入能力。
	raw, err := json.Marshal(ms[1])
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err = json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	if _, ok := fields["AttachmentRefs"]; ok {
		t.Fatal("read field leaked into receipt")
	}
	if err := f.s.Write(messageContext, func(tx *sql.Tx) error { _, e := tx.Exec(`DELETE FROM attachments WHERE id=?`, file); return e }); err == nil {
		t.Fatal("live reference FK did not protect attachment row")
	}
	execMessages(t, f.s, `DELETE FROM messages WHERE id=?`, ms[1].ID)
	if err := f.s.Read(messageContext, func(tx *sql.Tx) error {
		var n int
		if e := tx.QueryRow(`SELECT count(*) FROM message_attachments`).Scan(&n); e != nil {
			return e
		}
		if n != 0 {
			t.Fatal("message cascade")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
func TestMessageAttachmentRefsCorruptionFailsWholeCandidate(t *testing.T) {
	for _, kind := range []string{"cross-bot", "kind", "missing-table"} {
		t.Run(kind, func(t *testing.T) {
			f := setupMessages(t)
			ms := historySeed(t, f, 2)
			const id = "11111111-1111-4111-8111-111111111111"
			bot := f.a.Bot.ID
			if kind == "cross-bot" {
				bot = f.bbot.Bot.ID
			}
			attachmentRefSeed(t, f, id, bot, "file")
			linkKind := "file"
			if kind == "kind" {
				linkKind = "image"
			}
			attachmentRefLink(t, f, ms[0].ID, id, linkKind, 0)
			if kind == "missing-table" {
				execMessages(t, f.s, `DROP TABLE message_attachments`)
			}
			if err := f.s.Read(messageContext, func(tx *sql.Tx) error {
				p, e := f.r.ListHistoryTx(messageContext, tx, f.a.Bot.ID, HistoryQuery{Limit: 2})
				if e == nil || !reflect.DeepEqual(p, HistoryPage{}) {
					t.Fatal("partial page", p, e)
				}
				m, e := f.r.GetHistoryMessageByIDTx(messageContext, tx, ms[0].ID)
				if e == nil || !reflect.DeepEqual(m, Message{}) {
					t.Fatal("partial detail", m, e)
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		})
	}
}
func TestMessageAttachmentRefsConstraints(t *testing.T) {
	f := setupMessages(t)
	m := historySeed(t, f, 1)[0]
	const id = "11111111-1111-4111-8111-111111111111"
	attachmentRefSeed(t, f, id, f.a.Bot.ID, "file")
	for _, args := range [][]any{{m.ID, id, "bad", 0}, {m.ID, id, "file", -1}, {m.ID, id, "file", 10}, {"missing", id, "file", 0}, {m.ID, "missing", "file", 0}} {
		if err := f.s.Write(messageContext, func(tx *sql.Tx) error {
			_, e := tx.Exec(`INSERT INTO message_attachments VALUES(?,?,?,?)`, args...)
			return e
		}); err == nil {
			t.Fatal("invalid relation accepted", args)
		}
	}
	attachmentRefLink(t, f, m.ID, id, "file", 0)
	if err := f.s.Write(messageContext, func(tx *sql.Tx) error {
		_, e := tx.Exec(`INSERT INTO message_attachments VALUES(?,?,?,1)`, m.ID, id, "file")
		return e
	}); err == nil {
		t.Fatal("duplicate attachment")
	}
	const other = "22222222-2222-4222-8222-222222222222"
	attachmentRefSeed(t, f, other, f.a.Bot.ID, "file")
	if err := f.s.Write(messageContext, func(tx *sql.Tx) error {
		_, e := tx.Exec(`INSERT INTO message_attachments VALUES(?,?,?,0)`, m.ID, other, "file")
		return e
	}); err == nil {
		t.Fatal("duplicate position")
	}
}
