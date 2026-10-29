package httpapi

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"relaynest/server/internal/messages"
)

func TestHistoryAttachmentProjectionContract(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	m := messages.Message{ID: "11111111-1111-4111-8111-111111111111", BotID: "22222222-2222-4222-8222-222222222222", BotSeq: 1, Sender: messages.Application, Revision: 1, Format: "plain", Text: "text", ButtonsJSON: []byte("[]"), CreatedAt: now, UpdatedAt: now}
	refs := []messages.AttachmentReference{{ID: "33333333-3333-4333-8333-333333333333", Kind: "image"}, {ID: "44444444-4444-4444-8444-444444444444", Kind: "file"}}
	m.AttachmentRefs = refs
	out, e := projectHistoryMessage(m)
	if e != nil || !reflect.DeepEqual(out.Attachments, refs) {
		t.Fatal(out, e)
	}
	raw, e := json.Marshal(out)
	if e != nil {
		t.Fatal(e)
	}
	var doc struct {
		Attachments []map[string]any `json:"attachments"`
	}
	if e = json.Unmarshal(raw, &doc); e != nil {
		t.Fatal(e)
	}
	for _, ref := range doc.Attachments {
		if len(ref) != 2 || ref["id"] == nil || ref["kind"] == nil {
			t.Fatal(ref)
		}
	}
	m.AttachmentRefs = nil
	out, e = projectHistoryMessage(m)
	if e != nil || out.Attachments == nil || len(out.Attachments) != 0 {
		t.Fatal("empty attachments", out, e)
	}
	for _, bad := range [][]messages.AttachmentReference{{{ID: "bad", Kind: "file"}}, {{ID: refs[0].ID, Kind: "bad"}}, {refs[0], refs[0]}} {
		m.AttachmentRefs = bad
		if _, e := projectHistoryMessage(m); e == nil {
			t.Fatal("corrupt refs accepted", bad)
		}
	}
	m.AttachmentRefs = refs
	m.Text = ""
	if _, e := projectHistoryMessage(m); e != nil {
		t.Fatal("attachment-only history rejected", e)
	}
	// 保留原修复：候选附件存在也不能绕过授权Tx内UTC JSON年界验证。
	m.CreatedAt = time.Date(9999, 12, 31, 23, 59, 59, 0, time.FixedZone("west", -8*3600))
	if _, e := projectHistoryMessage(m); e == nil {
		t.Fatal("UTC overflow accepted")
	}
}
