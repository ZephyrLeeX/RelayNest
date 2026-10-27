package messages

import (
	"database/sql"
	"errors"
	"reflect"
	"testing"
)

func TestReplyDomainAndNaturalDeletion(t *testing.T) {
	f := setupMessages(t)
	source := historySeed(t, f, 1)[0]
	if err := f.r.ValidateReplyTx(messageContext, nil, f.a.Bot.ID, source.ID); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if err := f.s.Write(messageContext, func(tx *sql.Tx) error {
		if err := f.r.ValidateReplyTx(messageContext, tx, f.a.Bot.ID, source.ID); err != nil {
			return err
		}
		for _, args := range [][2]string{{f.bbot.Bot.ID, source.ID}, {f.a.Bot.ID, "missing"}, {"missing", source.ID}} {
			if err := f.r.ValidateReplyTx(messageContext, tx, args[0], args[1]); !errors.Is(err, ErrReplyUnavailable) {
				t.Fatal("reference domain leak", err)
			}
		}
		// 本叶只准备仓储；用真实 SQL fixture 验证既有触发器，不放开创建入口。
		_, err := tx.Exec(`INSERT INTO messages(id,bot_id,bot_seq,sender,revision,format,text,buttons_json,reply_to_message_id,original_reply_to_message_id,created_at,updated_at) VALUES('22222222-2222-4222-8222-222222222222',?,2,'administrator',1,'plain','reply','[]',?,?,?,?)`, f.a.Bot.ID, source.ID, source.ID, source.CreatedAt.Format("2006-01-02T15:04:05.999999999Z07:00"), source.UpdatedAt.Format("2006-01-02T15:04:05.999999999Z07:00"))
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if err := f.s.Write(messageContext, func(tx *sql.Tx) error {
		if _, err := tx.Exec(`UPDATE messages SET bot_id=? WHERE id='22222222-2222-4222-8222-222222222222'`, f.bbot.Bot.ID); err == nil {
			t.Fatal("cross Bot update trigger bypass")
		}
		if _, err := tx.Exec(`INSERT INTO messages SELECT '33333333-3333-4333-8333-333333333333',?,3,created_change_seq,sender,revision,format,text,buttons_json,reply_to_message_id,original_reply_to_message_id,created_at,updated_at FROM messages WHERE id='22222222-2222-4222-8222-222222222222'`, f.bbot.Bot.ID); err == nil {
			t.Fatal("cross Bot insert trigger bypass")
		}
		if _, err := tx.Exec(`UPDATE messages SET original_reply_to_message_id='changed' WHERE id='22222222-2222-4222-8222-222222222222'`); err == nil {
			t.Fatal("original reference mutable")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	execMessages(t, f.s, `DELETE FROM messages WHERE id=?`, source.ID)
	if err := f.s.Read(messageContext, func(tx *sql.Tx) error {
		m, err := f.r.GetHistoryMessageTx(messageContext, tx, f.a.Bot.ID, "22222222-2222-4222-8222-222222222222")
		if err != nil {
			return err
		}
		if m.ReplyToMessageID != nil || m.OriginalReplyToMessageID == nil || *m.OriginalReplyToMessageID != source.ID || m.Text != "reply" {
			t.Fatal("expired reference lost original or leaked body")
		}
		m, err = f.r.GetHistoryMessageTx(messageContext, tx, f.a.Bot.ID, source.ID)
		if !errors.Is(err, ErrHistoryNotFound) || !reflect.DeepEqual(m, Message{}) {
			t.Fatal("deleted body visible")
		}
		if err := f.r.ValidateReplyTx(messageContext, tx, f.a.Bot.ID, source.ID); !errors.Is(err, ErrReplyUnavailable) {
			t.Fatal("deleted source accepted")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
