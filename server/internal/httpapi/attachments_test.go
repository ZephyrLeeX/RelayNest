package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"relaynest/server/internal/attachments"
	"relaynest/server/internal/health"
)

type attachmentReader struct{ remaining, maxRead, reads int64 }

func (r *attachmentReader) Read(p []byte) (int, error) {
	r.reads++
	if int64(len(p)) > r.maxRead {
		r.maxRead = int64(len(p))
	}
	if r.remaining == 0 {
		return 0, io.EOF
	}
	n := int64(len(p))
	if n > r.remaining {
		n = r.remaining
	}
	for i := range p[:int(n)] {
		p[i] = 'x'
	}
	r.remaining -= n
	return int(n), nil
}
func (*attachmentReader) Close() error { return nil }

func TestAttachmentBoundedReadAndStagingBeforeBody(t *testing.T) {
	f := setupMessageHTTP(t)
	files, err := attachments.OpenFiles(filepath.Join(t.TempDir(), "files"))
	if err != nil {
		t.Fatal(err)
	}
	defer files.Close()
	h := NewAttachmentHandler(f.store, files, attachments.Limits{Image: 4 << 20, File: 4 << 20}, nil)
	body := &attachmentReader{remaining: 4 << 20}
	req := httptest.NewRequest("POST", "/api/v1/attachments?"+url.Values{"bot_id": {f.first.Bot.ID}, "kind": {"file"}, "filename": {"a"}, "media_type": {"text/plain"}}.Encode(), body)
	req.Header.Set("Authorization", "Bearer "+f.first.APIToken.Token)
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Idempotency-Key", messageKey)
	rec := httptest.NewRecorder()
	h.UploadAttachment(rec, req)
	if rec.Code != 201 || body.remaining != 0 || body.maxRead > 32<<10 || body.reads < 128 {
		t.Fatalf("status=%d remaining=%d buffer=%d reads=%d", rec.Code, body.remaining, body.maxRead, body.reads)
	}
}

func TestAttachmentMetadataRejectedWithoutBody(t *testing.T) {
	f := setupMessageHTTP(t)
	files, err := attachments.OpenFiles(filepath.Join(t.TempDir(), "files"))
	if err != nil {
		t.Fatal(err)
	}
	defer files.Close()
	h := NewAttachmentHandler(f.store, files, attachments.DefaultLimits(), nil)
	base := url.Values{"bot_id": {f.first.Bot.ID}, "kind": {"file"}, "filename": {"a"}, "media_type": {"text/plain"}}
	tests := []struct {
		name, query, content, key string
		status                    int
	}{
		{"duplicate", base.Encode() + "&kind=file", "application/octet-stream", messageKey, 400},
		{"unknown", base.Encode() + "&extra=x", "application/octet-stream", messageKey, 400},
		{"encoding", base.Encode() + "&x=%zz", "application/octet-stream", messageKey, 400},
		{"missing", "kind=file", "application/octet-stream", messageKey, 422},
		{"bad-kind", strings.Replace(base.Encode(), "kind=file", "kind=video", 1), "application/octet-stream", messageKey, 422},
		{"bad-mime", strings.Replace(base.Encode(), "text%2Fplain", "bad%2Fno+type", 1), "application/octet-stream", messageKey, 422},
		{"bad-key", base.Encode(), "application/octet-stream", "not-uuid", 400},
		{"json", base.Encode(), "application/json", messageKey, 400},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			body := &attachmentReader{remaining: 1}
			req := httptest.NewRequest("POST", "/api/v1/attachments?"+test.query, body)
			req.Header.Set("Authorization", "Bearer "+f.first.APIToken.Token)
			req.Header.Set("Content-Type", test.content)
			req.Header.Set("Idempotency-Key", test.key)
			rec := httptest.NewRecorder()
			h.UploadAttachment(rec, req)
			if rec.Code != test.status || body.reads != 0 {
				t.Fatalf("%d %s reads=%d", rec.Code, rec.Body.String(), body.reads)
			}
		})
	}
}

type attachmentRouteStub struct {
	routeProbe
	calls int
}

func (s *attachmentRouteStub) UploadAttachment(w http.ResponseWriter, r *http.Request) {
	s.calls++
	w.WriteHeader(201)
}

func TestAttachmentExactRoute(t *testing.T) {
	stub := &attachmentRouteStub{}
	router := New(health.New(nil), stub)
	for _, test := range []struct {
		method, path string
		want         int
	}{{"POST", "/api/v1/attachments", 201}, {"GET", "/api/v1/attachments", 405}, {"POST", "/api/v1/attachments/id", 404}, {"POST", "/api/v1/attachments/", 404}} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(test.method, test.path, nil))
		if rec.Code != test.want {
			t.Fatalf("%s %s: %d", test.method, test.path, rec.Code)
		}
	}
	if stub.calls != 1 {
		t.Fatal("unexpected upload calls")
	}
}

// 短事务提交之前 ReadyTx 必须拒绝 staging，避免上传前缀被后续关联消费。
func TestAttachmentStagingNotReady(t *testing.T) {
	f := setupMessageHTTP(t)
	repo := attachments.New(f.store)
	var attempt attachments.Attempt
	err := f.store.Write(context.Background(), func(tx *sql.Tx) error {
		var err error
		attempt, err = repo.BeginTx(context.Background(), tx, attachments.Input{Identity: "bot:" + f.first.Bot.ID, Key: messageKey, BotID: f.first.Bot.ID, Kind: "file", Filename: "a", MediaType: "text/plain"})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	err = f.store.Read(context.Background(), func(tx *sql.Tx) error {
		_, err := repo.ReadyTx(context.Background(), tx, attempt.Attachment.ID)
		return err
	})
	if !errors.Is(err, attachments.ErrNotReady) {
		t.Fatalf("staging ready: %v", err)
	}
}
