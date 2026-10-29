package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"relaynest/server/internal/attachments"
	"relaynest/server/internal/messages"
)

// 先走既有上传生产handler和真实卷；关系只由受信测试事务建立，不声称绑定写HTTP。
func attachmentHistoryUpload(t *testing.T, f *uploadFixture, kind, key string) messages.AttachmentReference {
	t.Helper()
	body := "attachment-private-bytes"
	media := "text/plain"
	if kind == "image" {
		body = "\x89PNG\r\n\x1a\n" + strings.Repeat("\x00", 64)
		media = "image/png"
	}
	path := "/api/v1/attachments?" + url.Values{"bot_id": {f.bot.Bot.ID}, "kind": {kind}, "filename": {"private-upload.txt"}, "media_type": {media}}.Encode()
	status, raw, h := f.request("POST", path, body, "android", f.android.Token, map[string]string{"Content-Type": "application/octet-stream", "Idempotency-Key": key})
	if status != 201 || h.Get("Cache-Control") != "no-store" {
		t.Fatalf("upload %d %s", status, raw)
	}
	var got struct{ ID, Kind, State, SHA256 string }
	if e := json.Unmarshal([]byte(raw), &got); e != nil {
		t.Fatal(e)
	}
	if got.State != "ready" || got.Kind != kind || got.SHA256 != fmt.Sprintf("%x", sha256.Sum256([]byte(body))) {
		t.Fatal("upload baseline", raw)
	}
	var stored string
	if e := f.store.Read(context.Background(), func(tx *sql.Tx) error {
		return tx.QueryRow(`SELECT storage_key FROM attachments WHERE id=?`, got.ID).Scan(&stored)
	}); e != nil {
		t.Fatal(e)
	}
	data, e := os.ReadFile(filepath.Join(f.directory, stored))
	if e != nil || string(data) != body {
		t.Fatal("physical upload mismatch", e)
	}
	return messages.AttachmentReference{ID: got.ID, Kind: kind}
}
func attachmentHistoryLink(t *testing.T, f *uploadFixture, m historyMessage, ref messages.AttachmentReference, position int) {
	t.Helper()
	f.sql(`INSERT INTO message_attachments(message_id,attachment_id,kind,position) VALUES(?,?,?,?)`, m.ID, ref.ID, ref.Kind, position)
}
func attachmentHistorySnapshot(t *testing.T, f *uploadFixture) [32]byte {
	t.Helper()
	all := []any{f.snapshot()}
	if e := f.store.Read(context.Background(), func(tx *sql.Tx) error {
		for _, table := range []string{"messages", "attachments", "message_attachments", "bot_read_states", "push_jobs", "deliveries", "idempotency"} {
			rows, e := tx.Query(`SELECT * FROM ` + table + ` ORDER BY 1`)
			if e != nil {
				return e
			}
			cols, e := rows.Columns()
			if e != nil {
				rows.Close()
				return e
			}
			for rows.Next() {
				v := make([]any, len(cols))
				p := make([]any, len(cols))
				for i := range v {
					p[i] = &v[i]
				}
				if e = rows.Scan(p...); e != nil {
					rows.Close()
					return e
				}
				all = append(all, v)
			}
			e = rows.Err()
			rows.Close()
			if e != nil {
				return e
			}
		}
		return nil
	}); e != nil {
		t.Fatal(e)
	}
	raw, e := json.Marshal(all)
	if e != nil {
		t.Fatal(e)
	}
	return sha256.Sum256(raw)
}
func TestAssembledHistoryAttachmentsFromTrustedRelations(t *testing.T) {
	f := uploadHTTP(t, attachments.DefaultLimits())
	image := attachmentHistoryUpload(t, f, "image", "11111111-1111-4111-8111-111111111111")
	file := attachmentHistoryUpload(t, f, "file", "22222222-2222-4222-8222-222222222222")
	unrelated := attachmentHistoryUpload(t, f, "file", "33333333-3333-4333-8333-333333333333")
	first := historyCreate(t, f.botHTTPFixture, f.bot.Bot.ID, "attachment-first", "first-private-message", nil)
	reply := historyCreate(t, f.botHTTPFixture, f.bot.Bot.ID, "attachment-reply", "reply-private-message", &first.ID)
	newest := historyCreate(t, f.botHTTPFixture, f.bot.Bot.ID, "attachment-empty", "empty-reference-message", nil)
	attachmentHistoryLink(t, f, first, file, 7)
	attachmentHistoryLink(t, f, first, image, 2)
	attachmentHistoryLink(t, f, reply, file, 0)
	base := "/api/v1/client/bots/" + f.bot.Bot.ID + "/messages"
	page := historyReadPage(t, f.botHTTPFixture, base+"?limit=2")
	if len(page.Items) != 2 || page.Items[0].ID != newest.ID || page.Items[0].Attachments == nil || len(page.Items[0].Attachments) != 0 || !reflect.DeepEqual(page.Items[1].Attachments, []messages.AttachmentReference{file}) || page.Next == nil {
		t.Fatal(page)
	}
	tail := historyReadPage(t, f.botHTTPFixture, base+"?limit=2&cursor="+url.QueryEscape(*page.Next))
	want := []messages.AttachmentReference{image, file}
	if len(tail.Items) != 1 || tail.Items[0].ID != first.ID || !reflect.DeepEqual(tail.Items[0].Attachments, want) {
		t.Fatal(tail)
	}
	raw := historyGet(t, f.botHTTPFixture, "/api/v1/client/messages/"+first.ID)
	detail := assertHistoryMessage(t, raw)
	if !reflect.DeepEqual(detail.Attachments, want) {
		t.Fatal(detail)
	}
	var contract struct {
		Attachments []map[string]any `json:"attachments"`
	}
	if e := json.Unmarshal([]byte(raw), &contract); e != nil {
		t.Fatal(e)
	}
	for _, ref := range contract.Attachments {
		if len(ref) != 2 {
			t.Fatal("extra reference field", ref)
		}
	}
	var key, digest string
	if e := f.store.Read(context.Background(), func(tx *sql.Tx) error {
		return tx.QueryRow(`SELECT storage_key,lower(hex(sha256)) FROM attachments WHERE id=?`, file.ID).Scan(&key, &digest)
	}); e != nil {
		t.Fatal(e)
	}
	for _, secret := range []string{key, digest, f.directory, f.android.Token, "private-upload.txt", unrelated.ID} {
		if strings.Contains(raw, secret) {
			t.Fatal("attachment secret/unrelated upload exposed")
		}
	}
	// 卷丢失与状态变化不应静默抹掉当前关系；下载／修复属于后包。
	if e := os.Remove(filepath.Join(f.directory, key)); e != nil {
		t.Fatal(e)
	}
	f.sql(`UPDATE attachments SET state='missing' WHERE id=?`, file.ID)
	f.sql(`UPDATE bots SET enabled=0 WHERE id=?`, f.bot.Bot.ID)
	got := assertHistoryMessage(t, historyGet(t, f.botHTTPFixture, "/api/v1/client/messages/"+reply.ID))
	if !reflect.DeepEqual(got.Attachments, []messages.AttachmentReference{file}) || got.Reply == nil || *got.Reply != first.ID {
		t.Fatal(got)
	}
	f.sql(`DELETE FROM messages WHERE id=?`, first.ID)
	status, raw, h := f.admin("GET", "/api/v1/client/messages/"+first.ID, "")
	if status != 404 || strings.Contains(raw, first.Text) || h.Get("Cache-Control") != "no-store" {
		t.Fatal(status, raw)
	}
	got = assertHistoryMessage(t, historyGet(t, f.botHTTPFixture, "/api/v1/client/messages/"+reply.ID))
	if got.Reply == nil || *got.Reply != first.ID || !reflect.DeepEqual(got.Attachments, []messages.AttachmentReference{file}) {
		t.Fatal(got)
	}
}
func TestAssembledHistoryAttachmentsCorruptRelationAtomic(t *testing.T) {
	for _, kind := range []string{"cross-bot", "mismatch", "bad-kind"} {
		t.Run(kind, func(t *testing.T) {
			f := uploadHTTP(t, attachments.DefaultLimits())
			ref := attachmentHistoryUpload(t, f, "file", "11111111-1111-4111-8111-111111111111")
			m := historyCreate(t, f.botHTTPFixture, f.bot.Bot.ID, "attachment-corrupt", "attachment-private-message", nil)
			historyCreate(t, f.botHTTPFixture, f.bot.Bot.ID, "attachment-valid", "another-private-message", nil)
			attachmentHistoryLink(t, f, m, ref, 0)
			switch kind {
			case "cross-bot":
				other := clientMessageBot(t, f.botHTTPFixture)
				f.sql(`UPDATE attachments SET bot_id=? WHERE id=?`, other.Bot.ID, ref.ID)
			case "mismatch":
				f.sql(`UPDATE message_attachments SET kind='image' WHERE message_id=?`, m.ID)
			case "bad-kind":
				if e := f.store.Write(context.Background(), func(tx *sql.Tx) error {
					if _, e := tx.Exec(`PRAGMA ignore_check_constraints=ON`); e != nil {
						return e
					}
					_, e := tx.Exec(`UPDATE message_attachments SET kind='corrupt' WHERE message_id=?`, m.ID)
					_, reset := tx.Exec(`PRAGMA ignore_check_constraints=OFF`)
					if e != nil {
						return e
					}
					return reset
				}); e != nil {
					t.Fatal(e)
				}
			}
			before := attachmentHistorySnapshot(t, f)
			for _, path := range []string{"/api/v1/client/bots/" + f.bot.Bot.ID + "/messages", "/api/v1/client/messages/" + m.ID} {
				status, raw, h := f.admin("GET", path, "")
				if status != 503 || !strings.Contains(raw, "SERVICE_UNAVAILABLE") || strings.Contains(raw, "private-message") || strings.Contains(raw, ref.ID) || h.Get("Cache-Control") != "no-store" || attachmentHistorySnapshot(t, f) != before {
					t.Fatalf("partial corrupt response %d %s", status, raw)
				}
			}
		})
	}
}
func TestAssembledHistoryAttachmentsFinalAuthorizationAtomic(t *testing.T) {
	for _, mutation := range []string{"revoked", "device", "generation", "instance"} {
		for _, detail := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/detail=%v", mutation, detail), func(t *testing.T) {
				f := uploadHTTP(t, attachments.DefaultLimits())
				ref := attachmentHistoryUpload(t, f, "file", "11111111-1111-4111-8111-111111111111")
				m := historyCreate(t, f.botHTTPFixture, f.bot.Bot.ID, "attachment-final", "attachment-private-message", nil)
				attachmentHistoryLink(t, f, m, ref, 0)
				before := attachmentHistorySnapshot(t, f)
				historyIntercept(f.botHTTPFixture, nil, func(tx *sql.Tx) error {
					var e error
					switch mutation {
					case "revoked":
						_, e = tx.Exec(`UPDATE sessions SET revoked=1 WHERE id=?`, f.androidID)
					case "device":
						_, e = tx.Exec(`UPDATE sessions SET device_id='00000000-0000-4000-8000-000000000099' WHERE id=?`, f.androidID)
					case "generation":
						_, e = tx.Exec(`UPDATE administrator SET auth_generation=auth_generation+1`)
					case "instance":
						_, e = tx.Exec(`UPDATE instance_identity SET instance_id='00000000-0000-4000-8000-000000000099' WHERE id=1`)
					}
					return e
				})
				path := "/api/v1/client/bots/" + f.bot.Bot.ID + "/messages"
				if detail {
					path = "/api/v1/client/messages/" + m.ID
				}
				status, raw, h := f.admin("GET", path, "")
				want := 401
				if mutation == "instance" {
					want = 409
				}
				if status != want || strings.Contains(raw, m.Text) || strings.Contains(raw, ref.ID) || h.Get("Cache-Control") != "no-store" || attachmentHistorySnapshot(t, f) != before {
					t.Fatalf("final identity published or renewed %d %s", status, raw)
				}
			})
		}
	}
}
func TestAssembledHistoryAttachmentsCreationStillUnsupported(t *testing.T) {
	f := uploadHTTP(t, attachments.DefaultLimits())
	ref := attachmentHistoryUpload(t, f, "file", "11111111-1111-4111-8111-111111111111")
	before := attachmentHistorySnapshot(t, f)
	body := fmt.Sprintf(`{"format":"plain","text":"not-new-binding","attachments":[{"id":%q,"kind":"file"}]}`, ref.ID)
	status, _, _ := clientMessageRequest(f.botHTTPFixture, f.bot.Bot.ID, "22222222-2222-4222-8222-222222222222", body)
	if status != 422 || attachmentHistorySnapshot(t, f) != before {
		t.Fatal("nonempty attachment creation changed", status)
	}
}
