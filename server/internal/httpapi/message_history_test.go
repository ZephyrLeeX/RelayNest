package httpapi

import (
	"encoding/base64"
	"encoding/json"
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

func TestHistoryProjectionUTCTimeBoundaries(t *testing.T) {
	for _, timestamp := range []struct {
		value string
		valid bool
	}{
		{"9999-12-31T23:59:59-08:00", false},
		{"0000-01-01T00:00:00+08:00", false},
		{"9999-12-31T23:59:59Z", true},
		{"0000-01-01T00:00:00Z", true},
		{"9999-12-31T23:59:59+08:00", true},
		{"0000-01-01T00:00:00-08:00", true},
	} {
		for _, field := range []string{"created_at", "updated_at"} {
			t.Run(field+"/"+timestamp.value, func(t *testing.T) {
				parsed, err := time.Parse(time.RFC3339Nano, timestamp.value)
				if err != nil {
					t.Fatal(err)
				}
				now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
				m := messages.Message{ID: "00000000-0000-4000-8000-000000000001", BotID: "00000000-0000-4000-8000-000000000002", BotSeq: 1, Revision: 1, Sender: messages.Application, Format: "plain", Text: "时间边界", ButtonsJSON: []byte("[]"), CreatedAt: now, UpdatedAt: now}
				if field == "created_at" {
					m.CreatedAt = parsed
				} else {
					m.UpdatedAt = parsed
				}
				out, err := projectHistoryMessage(m)
				if !timestamp.valid {
					if err == nil {
						t.Fatal("接受了 UTC 转换后无法序列化的时间")
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				if out.CreatedAt.Location() != time.UTC || out.UpdatedAt.Location() != time.UTC || !out.CreatedAt.Equal(m.CreatedAt) || !out.UpdatedAt.Equal(m.UpdatedAt) {
					t.Fatalf("UTC 投影改变时间: %+v", out)
				}
				if _, err := json.Marshal(out); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
