package messages

import (
	"database/sql"
	"errors"
	"reflect"
	"testing"
	"time"

	"relaynest/server/internal/changes"
)

// BE04-CAS-STORAGE-A01-01：插入被忽略或插入行随后消失/失配，都不能
// 把连接上的旧LastInsertId或触发器副作用当作本次编辑的同步变更。
func TestEditTextButtonsChangeConfirmation(t *testing.T) {
	cases := map[string]string{
		"ignored":   `CREATE TRIGGER edit_change_fault BEFORE INSERT ON changes BEGIN SELECT RAISE(IGNORE);END`,
		"deleted":   `CREATE TRIGGER edit_change_fault AFTER INSERT ON changes BEGIN DELETE FROM changes WHERE change_seq=NEW.change_seq;END`,
		"rewritten": `CREATE TRIGGER edit_change_fault AFTER INSERT ON changes BEGIN UPDATE changes SET revision=revision+1 WHERE change_seq=NEW.change_seq;END`,
	}
	for name, ddl := range cases {
		t.Run(name, func(t *testing.T) {
			f := setupMessages(t)
			_, in := editSeed(t, f)
			execMessages(t, f.s, ddl)
			before := editSnapshot(t, f.other)
			var candidate Edited
			err := f.s.Write(messageContext, func(tx *sql.Tx) error {
				var err error
				candidate, err = f.r.EditApplicationTx(messageContext, tx, f.identity, in)
				return err
			})
			if !errors.Is(err, ErrAffectedRows) || !reflect.DeepEqual(candidate, Edited{}) || !reflect.DeepEqual(before, editSnapshot(t, f.other)) {
				t.Fatal("missing/mismatched edit change returned success or persisted partial state", err, candidate)
			}
			execMessages(t, f.s, `DROP TRIGGER edit_change_fault`)
			out, err := editOwned(messageContext, f.s, f.r, f.identity, in)
			if err != nil || out.Message.Revision != 2 || out.Message.Text != in.Text {
				t.Fatal("edit did not recover after change fault", err)
			}
			if len(editSnapshot(t, f.other)["changes"]) != len(before["changes"])+1 {
				t.Fatal("normal edit did not append exactly one change")
			}
			if err := f.other.Read(messageContext, func(tx *sql.Tx) error {
				var typ, id, botID, stamp string
				var revision int64
				if err := tx.QueryRow(`SELECT type,entity_id,revision,bot_id,created_at FROM changes WHERE change_seq=?`, out.LastChangeSeq).Scan(&typ, &id, &revision, &botID, &stamp); err != nil {
					return err
				}
				if typ != string(changes.MessageUpsert) || id != out.Message.ID || revision != 2 || botID != in.BotID || stamp != out.Message.UpdatedAt.UTC().Format(time.RFC3339Nano) {
					t.Fatal("edit returned a change unrelated to the current revision")
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// BE04-CAS-STORAGE-A01-02：显式模拟可信坏库，不能用正常immutable触发器
// 会阻止写坏来替代损坏验收；只拒绝坏引用，绝不修复、清空或恢复原引用。
func TestEditTextButtonsRetainedReplyValidation(t *testing.T) {
	for _, original := range []string{"invalid-reference", "", "11111111-1111-1111-8111-111111111111", "ABCDEF01-2345-4678-8ABC-ABCDEF012345"} {
		t.Run(original, func(t *testing.T) {
			f := setupMessages(t)
			_, in := editSeed(t, f)
			execMessages(t, f.s, `DROP TRIGGER messages_original_reply_immutable;UPDATE messages SET original_reply_to_message_id=? WHERE id=?`, original, in.MessageID)
			before := editSnapshot(t, f.other)
			var candidate Edited
			err := f.s.Write(messageContext, func(tx *sql.Tx) error {
				var err error
				candidate, err = f.r.EditApplicationTx(messageContext, tx, f.identity, in)
				return err
			})
			if !errors.Is(err, ErrInvalid) || !reflect.DeepEqual(candidate, Edited{}) || !reflect.DeepEqual(before, editSnapshot(t, f.other)) {
				t.Fatal("invalid retained reply was committed or returned as a candidate", err, candidate)
			}
		})
	}
}
