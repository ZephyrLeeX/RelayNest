package messages

import (
	"database/sql"
	"errors"
	"reflect"
	"testing"
)

func editSnapshotSeed(t *testing.T, f *fixture) (EditTextButtonsInput, string) {
	t.Helper()
	source, _ := editSeed(t, f)
	reply, err := f.r.CreateIdempotentApplication(messageContext, f.identity, "77777777-7777-4777-8777-777777777777", replyRequest(source.Message.ID), nil)
	if err != nil {
		t.Fatal(err)
	}
	const aid = "88888888-8888-4888-8888-888888888888"
	attachmentRefSeed(t, f, aid, f.a.Bot.ID, "image")
	attachmentRefLink(t, f, reply.Message.ID, aid, "image", 4)
	url := "custom:opaque"
	return EditTextButtonsInput{BotID: f.a.Bot.ID, MessageID: reply.Message.ID, ExpectedRevision: 1, Format: "markdown", Text: "requested snapshot",
		Buttons: [][]Button{{{ID: "url", Label: "open", URL: &url}}}}, source.Message.ID
}

// BE04-CAS-STORAGE-A02-01：同样的typed Scan强转问题不限于审计中的两个
// 水位列；同时覆盖revision、保留文本元数据和附件位置的真实SQLite存储类。
func TestEditTextButtonsStoredTypes(t *testing.T) {
	for _, column := range []string{"bot_seq", "created_change_seq", "revision", "sender", "format", "text", "buttons_json", "created_at", "updated_at", "original_reply_to_message_id", "attachment-position"} {
		t.Run(column, func(t *testing.T) {
			f := setupMessages(t)
			in, _ := editSnapshotSeed(t, f)
			if column == "created_change_seq" {
				execMessages(t, f.s, `DROP TRIGGER messages_creation_seq_immutable`)
			}
			if column == "original_reply_to_message_id" {
				execMessages(t, f.s, `DROP TRIGGER messages_original_reply_immutable`)
			}
			query := `SELECT typeof(` + column + `) FROM messages WHERE id=?`
			if column == "attachment-position" {
				execMessages(t, f.s, `PRAGMA ignore_check_constraints=ON;UPDATE message_attachments SET position=CAST(position AS BLOB) WHERE message_id=?;PRAGMA ignore_check_constraints=OFF`, in.MessageID)
				query = `SELECT typeof(position) FROM message_attachments WHERE message_id=?`
			} else {
				execMessages(t, f.s, `PRAGMA ignore_check_constraints=ON;UPDATE messages SET `+column+`=CAST(`+column+` AS BLOB) WHERE id=?;PRAGMA ignore_check_constraints=OFF`, in.MessageID)
			}
			var storedType string
			if err := f.other.Read(messageContext, func(tx *sql.Tx) error {
				return tx.QueryRow(query, in.MessageID).Scan(&storedType)
			}); err != nil || storedType != "blob" {
				t.Fatal("bad type fixture was not persisted", storedType, err)
			}
			before := editSnapshot(t, f.other)
			var candidate Edited
			err := f.s.Write(messageContext, func(tx *sql.Tx) error {
				var err error
				candidate, err = f.r.EditApplicationTx(messageContext, tx, f.identity, in)
				return err
			})
			if err == nil || !reflect.DeepEqual(candidate, Edited{}) || !reflect.DeepEqual(before, editSnapshot(t, f.other)) {
				t.Fatal("invalid authoritative storage type succeeded or changed state", column, err, candidate)
			}
		})
	}
}

// BE04-CAS-STORAGE-A02-02：同一根因覆盖本次允许修改的全部列、保留列及
// 附件关联，不能只识别审计示例中的text值或某个触发器名字。
func TestEditTextButtonsPostAppendSnapshot(t *testing.T) {
	cases := map[string]string{
		"text":           `UPDATE messages SET text='different content' WHERE id=NEW.entity_id;`,
		"format":         `UPDATE messages SET format='plain' WHERE id=NEW.entity_id;`,
		"buttons":        `UPDATE messages SET buttons_json='[]' WHERE id=NEW.entity_id;`,
		"revision":       `UPDATE messages SET revision=revision+1 WHERE id=NEW.entity_id;`,
		"updated_at":     `UPDATE messages SET updated_at='2026-01-01T00:00:00Z' WHERE id=NEW.entity_id;`,
		"bot_seq":        `UPDATE messages SET bot_seq=bot_seq+20 WHERE id=NEW.entity_id;`,
		"created_at":     `UPDATE messages SET created_at='2026-01-01T00:00:00Z' WHERE id=NEW.entity_id;`,
		"created_seq":    `UPDATE messages SET created_change_seq=created_change_seq+1 WHERE id=NEW.entity_id;`,
		"reply":          `UPDATE messages SET reply_to_message_id=NULL WHERE id=NEW.entity_id;`,
		"original":       `UPDATE messages SET reply_to_message_id=NULL,original_reply_to_message_id='99999999-9999-4999-8999-999999999999' WHERE id=NEW.entity_id;`,
		"delete-link":    `DELETE FROM message_attachments WHERE message_id=NEW.entity_id;`,
		"link-position":  `UPDATE message_attachments SET position=position+1 WHERE message_id=NEW.entity_id;`,
		"binding-kind":   `UPDATE attachments SET kind='file' WHERE id IN (SELECT attachment_id FROM message_attachments WHERE message_id=NEW.entity_id);UPDATE message_attachments SET kind='file' WHERE message_id=NEW.entity_id;`,
		"sender":         `UPDATE messages SET sender='administrator' WHERE id=NEW.entity_id;`,
		"raw-created-at": `UPDATE messages SET created_at='2026-01-01T08:00:00.000+08:00' WHERE id=NEW.entity_id;`,
	}
	for mode, body := range cases {
		t.Run(mode, func(t *testing.T) {
			f := setupMessages(t)
			in, _ := editSnapshotSeed(t, f)
			if mode == "created_seq" {
				execMessages(t, f.s, `DROP TRIGGER messages_creation_seq_immutable`)
			}
			if mode == "original" {
				execMessages(t, f.s, `DROP TRIGGER messages_original_reply_immutable`)
			}
			if mode == "raw-created-at" {
				execMessages(t, f.s, `UPDATE messages SET created_at='2026-01-01T00:00:00Z' WHERE id=?`, in.MessageID)
			}
			execMessages(t, f.s, `CREATE TRIGGER edit_snapshot_fault AFTER INSERT ON changes WHEN NEW.type='message.upsert' BEGIN `+body+` END`)
			before := editSnapshot(t, f.other)
			var candidate Edited
			err := f.s.Write(messageContext, func(tx *sql.Tx) error {
				var err error
				candidate, err = f.r.EditApplicationTx(messageContext, tx, f.identity, in)
				return err
			})
			if !errors.Is(err, ErrAffectedRows) || !reflect.DeepEqual(candidate, Edited{}) || !reflect.DeepEqual(before, editSnapshot(t, f.other)) {
				t.Fatal("post-Append mismatch did not fail atomically", mode, err, candidate)
			}
			execMessages(t, f.s, `DROP TRIGGER edit_snapshot_fault`)
			out, err := editOwned(messageContext, f.s, f.r, f.identity, in)
			if err != nil || out.Message.Text != in.Text || out.Message.Format != in.Format || out.Message.Revision != 2 {
				t.Fatal("edit did not recover after fault removal", err)
			}
			if err := f.other.Read(messageContext, func(tx *sql.Tx) error {
				current, err := f.r.GetHistoryMessageTx(messageContext, tx, in.BotID, in.MessageID)
				if err != nil {
					return err
				}
				if !reflect.DeepEqual(current, out.Message) {
					t.Fatal("successful candidate differs from committed current message")
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestEditTextButtonsRawRetentionAndNulls(t *testing.T) {
	f := setupMessages(t)
	in, source := editSnapshotSeed(t, f)
	execMessages(t, f.s, `DELETE FROM messages WHERE id=?`, source)
	const rawCreated = "2026-01-01T08:00:00.000+08:00"
	execMessages(t, f.s, `UPDATE messages SET created_at=?,buttons_json='[ ]' WHERE id=?`, rawCreated, in.MessageID)
	in.Buttons = nil
	before := editSnapshot(t, f.other)
	out, err := editOwned(messageContext, f.s, f.r, f.identity, in)
	if err != nil || out.Message.ReplyToMessageID != nil || out.Message.OriginalReplyToMessageID == nil || *out.Message.OriginalReplyToMessageID != source || string(out.Message.ButtonsJSON) != "[]" {
		t.Fatal("valid expired reference or button normalization changed", err, out)
	}
	var persistedCreated string
	if err := f.other.Read(messageContext, func(tx *sql.Tx) error {
		return tx.QueryRow(`SELECT created_at FROM messages WHERE id=?`, in.MessageID).Scan(&persistedCreated)
	}); err != nil || persistedCreated != rawCreated {
		t.Fatal("edit rewrote raw retained creation time", persistedCreated, err)
	}
	after := editSnapshot(t, f.other)
	for table, rows := range before {
		if table != "messages" && table != "changes" && table != "sqlite_sequence" && !reflect.DeepEqual(rows, after[table]) {
			t.Fatal("edit changed a retained relation/read/receipt/delivery/push table", table)
		}
	}
}
