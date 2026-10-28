package httpapi

import (
	"encoding/base64"
	"strconv"
	"testing"
	"time"

	"relaynest/server/internal/messages"
)

func TestHistoryCursorDomainAndLimit(t *testing.T) {
	const bot = "00000000-0000-4000-8000-000000000001"
	valid := encodeHistoryCursor(bot, 9223372036854775807)
	query, err := historyQuery("limit=999999&cursor="+valid, bot)
	if err != nil || query.Limit != 200 || query.BeforeBotSeq == nil || *query.BeforeBotSeq != 9223372036854775807 {
		t.Fatalf("%+v %v", query, err)
	}
	for _, seq := range []int64{1, 2, 9223372036854775807} {
		cursor := encodeHistoryCursor(bot, seq)
		query, err := historyQuery("cursor="+cursor, bot)
		if err != nil || query.BeforeBotSeq == nil || *query.BeforeBotSeq != seq || query.Limit != 50 {
			t.Fatalf("seq %d %+v %v", seq, query, err)
		}
	}
	for _, raw := range []string{"0", "-1", "01", "+1", "9223372036854775808", "1e2"} {
		cursor := base64.RawURLEncoding.EncodeToString([]byte("history:1:" + bot + ":" + raw))
		if _, err := historyQuery("cursor="+cursor, bot); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	if _, err := historyQuery("cursor="+valid, "00000000-0000-4000-8000-000000000002"); err == nil {
		t.Fatal("跨Bot游标被接受")
	}
}

func TestHistoryProjectionRejectsCorruptSnapshot(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	m := messages.Message{ID: "00000000-0000-4000-8000-000000000001", BotID: "00000000-0000-4000-8000-000000000002", BotSeq: 9223372036854775807, Revision: 2, Sender: messages.Application, Format: "plain", Text: "内容", ButtonsJSON: []byte("[]"), CreatedAt: now, UpdatedAt: now}
	out, err := projectHistoryMessage(m)
	if err != nil || out.BotSeq != strconv.FormatInt(m.BotSeq, 10) || out.Revision != 2 || out.Attachments == nil || out.Buttons == nil {
		t.Fatalf("%+v %v", out, err)
	}
	for _, buttons := range []string{"null", "[null]", "[[]]", "[{}]", `[[{"id":"a","label":"b"}]]`, `[[{"id":"a","label":"b","url":"https://example.test","callback_data":"x"}]]`, `[[{"id":"a","label":"b","url":"relative"}]]`, `[[{"id":"a","label":"b","url":"https://example.test","unexpected":true}]]`, "[] []"} {
		t.Run(buttons, func(t *testing.T) {
			m.ButtonsJSON = []byte(buttons)
			if _, err := projectHistoryMessage(m); err == nil {
				t.Fatalf("accepted %s", buttons)
			}
		})
	}
}
