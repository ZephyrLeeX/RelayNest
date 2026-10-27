package messages

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"reflect"
	"testing"

	"relaynest/server/internal/storage"
)

func historySeed(t *testing.T, f *fixture, count int) []Message {
	t.Helper()
	var messages []Message
	for i := 0; i < count; i++ {
		in := f.input()
		in.PushTargets = nil
		out, err := f.r.CreateApplication(messageContext, f.identity, in)
		if err != nil {
			t.Fatal(err)
		}
		messages = append(messages, out.Message)
	}
	return messages
}

func historyPage(t *testing.T, f *fixture, botID string, before *int64, limit int) HistoryPage {
	t.Helper()
	var page HistoryPage
	if err := f.s.Read(messageContext, func(tx *sql.Tx) error {
		var err error
		page, err = f.r.ListHistoryTx(messageContext, tx, botID, HistoryQuery{before, limit})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return page
}

func assertHistorySeq(t *testing.T, page HistoryPage, more bool, seqs ...int64) {
	t.Helper()
	got := make([]int64, 0, len(page.Items))
	for _, item := range page.Items {
		got = append(got, item.BotSeq)
	}
	if !reflect.DeepEqual(got, seqs) || page.HasMore != more {
		t.Fatalf("page %v more=%v; want %v more=%v", got, page.HasMore, seqs, more)
	}
}

func TestHistoryPaginationAndCurrentContent(t *testing.T) {
	f := setupMessages(t)
	messages := historySeed(t, f, 7)
	assertHistorySeq(t, historyPage(t, f, f.bbot.Bot.ID, nil, 2), false, []int64{}...)
	// 另一 Bot 有相同序号，不能混入当前分页。
	execMessages(t, f.s, `INSERT INTO messages(id,bot_id,bot_seq,sender,revision,format,text,buttons_json,created_at,updated_at) VALUES('11111111-1111-4111-8111-111111111111',?,7,'application',1,'plain','other','[]',?,?)`, f.bbot.Bot.ID, messages[0].CreatedAt.Format("2006-01-02T15:04:05.999999999Z07:00"), messages[0].UpdatedAt.Format("2006-01-02T15:04:05.999999999Z07:00"))
	page := historyPage(t, f, f.a.Bot.ID, nil, 2)
	assertHistorySeq(t, page, true, 7, 6)
	if !reflect.DeepEqual(page.Items[0], messages[6]) {
		t.Fatal("persistent fields lost")
	}
	// 翻页期间新增、编辑、删除；边界取上一页最后一项，旧消息不重复。
	historySeed(t, f, 1)
	execMessages(t, f.s, `UPDATE messages SET text='edited',revision=2 WHERE id=?`, messages[4].ID)
	execMessages(t, f.s, `DELETE FROM messages WHERE id=?`, messages[3].ID)
	before := int64(6)
	page = historyPage(t, f, f.a.Bot.ID, &before, 2)
	assertHistorySeq(t, page, true, 5, 3)
	if page.Items[0].Text != "edited" || page.Items[0].Revision != 2 {
		t.Fatal("edit missing")
	}
	before = 3
	assertHistorySeq(t, historyPage(t, f, f.a.Bot.ID, &before, 2), false, 2, 1)
	before = 4 // 边界本身已经删除，仍可找到更旧消息。
	assertHistorySeq(t, historyPage(t, f, f.a.Bot.ID, &before, 2), true, 3, 2)
	before = 1
	assertHistorySeq(t, historyPage(t, f, f.a.Bot.ID, &before, 2), false, []int64{}...)
	before = math.MaxInt64
	assertHistorySeq(t, historyPage(t, f, f.a.Bot.ID, &before, 1), true, 8)
	execMessages(t, f.s, `UPDATE bots SET enabled=0 WHERE id=?`, f.a.Bot.ID)
	reopened, err := storage.Open(messageContext, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if err := reopened.Read(messageContext, func(tx *sql.Tx) error {
		m, err := New(reopened).GetHistoryMessageTx(messageContext, tx, f.a.Bot.ID, messages[4].ID)
		if err == nil && (m.Text != "edited" || m.BotSeq != 5) {
			t.Fatal("reopen/disabled detail")
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
}

func TestHistoryInvalidAndFailuresReturnNoCandidate(t *testing.T) {
	f := setupMessages(t)
	messages := historySeed(t, f, 3)
	assertHistorySeq(t, historyPage(t, f, f.a.Bot.ID, nil, MaxHistoryLimit), false, 3, 2, 1)
	for _, query := range []HistoryQuery{{Limit: 0}, {Limit: -1}, {Limit: MaxHistoryLimit + 1}, {Limit: math.MaxInt}, {BeforeBotSeq: new(int64), Limit: 1}} {
		if err := f.s.Read(messageContext, func(tx *sql.Tx) error {
			page, err := f.r.ListHistoryTx(messageContext, tx, f.a.Bot.ID, query)
			if !errors.Is(err, ErrInvalid) || !reflect.DeepEqual(page, HistoryPage{}) {
				t.Fatal("invalid query accepted", err)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	if p, err := f.r.ListHistoryTx(messageContext, nil, f.a.Bot.ID, HistoryQuery{Limit: 1}); !errors.Is(err, ErrInvalid) || !reflect.DeepEqual(p, HistoryPage{}) {
		t.Fatal("nil transaction")
	}
	if m, err := f.r.GetHistoryMessageTx(messageContext, nil, f.a.Bot.ID, messages[0].ID); !errors.Is(err, ErrInvalid) || !reflect.DeepEqual(m, Message{}) {
		t.Fatal("nil detail")
	}
	if err := f.s.Read(messageContext, func(tx *sql.Tx) error {
		for _, bot := range []string{f.bbot.Bot.ID, "missing"} {
			m, err := f.r.GetHistoryMessageTx(messageContext, tx, bot, messages[0].ID)
			if !errors.Is(err, ErrHistoryNotFound) || !reflect.DeepEqual(m, Message{}) {
				t.Fatal("cross Bot detail", err)
			}
		}
		p, err := f.r.ListHistoryTx(messageContext, tx, "missing", HistoryQuery{Limit: 1})
		if !errors.Is(err, ErrHistoryNotFound) || !reflect.DeepEqual(p, HistoryPage{}) {
			t.Fatal("missing Bot")
		}
		ctx, cancel := context.WithCancel(messageContext)
		cancel()
		p, err = f.r.ListHistoryTx(ctx, tx, f.a.Bot.ID, HistoryQuery{Limit: 1})
		if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(p, HistoryPage{}) {
			t.Fatal("SQL cancellation", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	// SQLite 时间列允许旧数据损坏；第二行解析失败时不能泄漏第一行候选。
	execMessages(t, f.s, `UPDATE messages SET updated_at='bad-time' WHERE id=?`, messages[1].ID)
	if err := f.s.Read(messageContext, func(tx *sql.Tx) error {
		p, err := f.r.ListHistoryTx(messageContext, tx, f.a.Bot.ID, HistoryQuery{Limit: 3})
		if err == nil || !reflect.DeepEqual(p, HistoryPage{}) {
			t.Fatal("partial scan result", err)
		}
		m, err := f.r.GetHistoryMessageTx(messageContext, tx, f.a.Bot.ID, messages[1].ID)
		if err == nil || !reflect.DeepEqual(m, Message{}) {
			t.Fatal("partial detail result", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	// 真实 SQLite 类型损坏导致 Scan 失败；已扫描行也不能作为部分列表返回。
	if err := f.s.Write(messageContext, func(tx *sql.Tx) error {
		if _, err := tx.Exec(`PRAGMA ignore_check_constraints=ON`); err != nil {
			return err
		}
		defer tx.Exec(`PRAGMA ignore_check_constraints=OFF`)
		if _, err := tx.Exec(`UPDATE messages SET updated_at=?,revision='broken' WHERE id=?`, messages[1].UpdatedAt.Format("2006-01-02T15:04:05.999999999Z07:00"), messages[1].ID); err != nil {
			return err
		}
		p, err := f.r.ListHistoryTx(messageContext, tx, f.a.Bot.ID, HistoryQuery{Limit: 3})
		if err == nil || !reflect.DeepEqual(p, HistoryPage{}) {
			t.Fatal("partial SQL scan result", err)
		}
		return errors.New("rollback scan fixture")
	}); err == nil {
		t.Fatal("scan fixture committed")
	}
	// 真实 SQLite SQL 故障，不为生产代码加测试钩子。
	if err := f.s.Write(messageContext, func(tx *sql.Tx) error {
		if _, err := tx.Exec(`ALTER TABLE messages RENAME COLUMN text TO broken_text`); err != nil {
			return err
		}
		p, err := f.r.ListHistoryTx(messageContext, tx, f.a.Bot.ID, HistoryQuery{Limit: 3})
		if err == nil || !reflect.DeepEqual(p, HistoryPage{}) {
			t.Fatal("SQL failure returned candidate")
		}
		return errors.New("rollback fixture")
	}); err == nil {
		t.Fatal("fixture committed")
	}
}

func TestHistorySameTransactionSnapshot(t *testing.T) {
	f := setupMessages(t)
	messages := historySeed(t, f, 2)
	if err := f.s.Read(messageContext, func(tx *sql.Tx) error {
		// 先固定读快照；另一 Store 删除 Bot 及其消息后，同事务仍完整读取旧快照。
		if err := historyBotTx(messageContext, tx, f.a.Bot.ID); err != nil {
			return err
		}
		execMessages(t, f.other, `DELETE FROM bots WHERE id=?`, f.a.Bot.ID)
		p, err := f.r.ListHistoryTx(messageContext, tx, f.a.Bot.ID, HistoryQuery{Limit: 2})
		if err != nil {
			return err
		}
		assertHistorySeq(t, p, false, 2, 1)
		m, err := f.r.GetHistoryMessageTx(messageContext, tx, f.a.Bot.ID, messages[0].ID)
		if err == nil && m.ID != messages[0].ID {
			t.Fatal("snapshot detail")
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if err := f.s.Read(messageContext, func(tx *sql.Tx) error {
		_, err := f.r.ListHistoryTx(messageContext, tx, f.a.Bot.ID, HistoryQuery{Limit: 2})
		if !errors.Is(err, ErrHistoryNotFound) {
			t.Fatal("new snapshot still exists", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
