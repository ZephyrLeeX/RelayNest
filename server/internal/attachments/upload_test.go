package attachments

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"relaynest/server/internal/bots"
	"relaynest/server/internal/storage"
)

type fixture struct {
	s            *storage.Store
	r            *Repository
	f            *Files
	u            *Uploader
	path, volume string
}

func setup(t *testing.T) *fixture {
	t.Helper()
	dir := t.TempDir()
	ctx := context.Background()
	s, e := storage.Open(ctx, storage.Config{Path: filepath.Join(dir, "db.sqlite")})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close() })
	e = s.Write(ctx, func(tx *sql.Tx) error {
		_, e := tx.Exec(`INSERT INTO bots(id,name,enabled,can_send,can_receive,version) VALUES('bot','test',1,1,1,1)`)
		return e
	})
	if e != nil {
		t.Fatal(e)
	}
	volume := filepath.Join(dir, "volume")
	f, e := OpenFiles(volume)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { f.Close() })
	r := New(s)
	u, e := NewUploader(r, f, DefaultLimits(), s.Write)
	if e != nil {
		t.Fatal(e)
	}
	return &fixture{s, r, f, u, filepath.Join(dir, "db.sqlite"), volume}
}
func input() Input {
	return Input{"bot:bot", uuid.NewString(), "bot", "file", "../../display.txt", "text/plain"}
}
func counts(t *testing.T, x *fixture) (int, int) {
	t.Helper()
	var staging, ready int
	e := x.s.Read(context.Background(), func(tx *sql.Tx) error {
		return tx.QueryRow(`SELECT count(*) FILTER(WHERE state='staging'),count(*) FILTER(WHERE state='ready') FROM attachments`).Scan(&staging, &ready)
	})
	if e != nil {
		t.Fatal(e)
	}
	return staging, ready
}
func TestUploadReplayConflictAndReopen(t *testing.T) {
	x := setup(t)
	ctx := context.Background()
	in := input()
	a, e := x.u.Upload(ctx, in, strings.NewReader("payload"))
	if e != nil {
		t.Fatal(e)
	}
	if a.State != "ready" || a.Size != 7 || a.Filename != in.Filename {
		t.Fatalf("bad result %+v", a)
	}
	file, e := x.f.Open(a.StorageKey)
	if e != nil {
		t.Fatal(e)
	}
	b, e := io.ReadAll(file)
	file.Close()
	if e != nil || string(b) != "payload" {
		t.Fatal(string(b), e)
	}
	reader := &countReader{r: strings.NewReader("payload")}
	again, e := x.u.Upload(ctx, in, reader)
	if e != nil || again.ID != a.ID || reader.n != 7 {
		t.Fatal(again, e, reader.n)
	}
	out, e := x.u.Upload(ctx, in, strings.NewReader("PAYLOAD"))
	if !errors.Is(e, ErrConflict) || out.ID != "" {
		t.Fatal(out, e)
	}
	if e = x.s.Close(); e != nil {
		t.Fatal(e)
	}
	s, e := storage.Open(ctx, storage.Config{Path: x.path})
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	u, e := NewUploader(New(s), x.f, DefaultLimits(), s.Write)
	if e != nil {
		t.Fatal(e)
	}
	again, e = u.Upload(ctx, in, strings.NewReader("payload"))
	if e != nil || again.ID != a.ID {
		t.Fatal(again, e)
	}
}

type countReader struct {
	r io.Reader
	n int
}

func (c *countReader) Read(p []byte) (int, error) { n, e := c.r.Read(p); c.n += n; return n, e }

type gateReader struct {
	entered, release chan struct{}
	data             string
}

func (g *gateReader) Read(p []byte) (int, error) {
	close(g.entered)
	<-g.release
	return copy(p, g.data), io.EOF
}
func TestConcurrentSameKeyAndDifferentContent(t *testing.T) {
	for _, body := range []string{"first", "other"} {
		t.Run(body, func(t *testing.T) {
			x := setup(t)
			in := input()
			gate := &gateReader{make(chan struct{}), make(chan struct{}), "first"}
			done := make(chan error, 1)
			go func() { _, e := x.u.Upload(context.Background(), in, gate); done <- e }()
			<-gate.entered
			_, e := x.u.Upload(context.Background(), in, strings.NewReader(body))
			if !errors.Is(e, ErrActive) {
				t.Fatal(e)
			}
			close(gate.release)
			if e = <-done; e != nil {
				t.Fatal(e)
			}
			_, e = x.u.Upload(context.Background(), in, strings.NewReader(body))
			if body == "first" && e != nil {
				t.Fatal(e)
			}
			if body != "first" && !errors.Is(e, ErrConflict) {
				t.Fatal(e)
			}
			_, ready := counts(t, x)
			if ready != 1 {
				t.Fatal(ready)
			}
		})
	}
}

type brokenReader struct{ n int }

func (b *brokenReader) Read(p []byte) (int, error) {
	if b.n == 0 {
		b.n++
		copy(p, "half")
		return 4, nil
	}
	return 0, io.ErrUnexpectedEOF
}
func TestHalfFileAndCancellation(t *testing.T) {
	x := setup(t)
	out, e := x.u.Upload(context.Background(), input(), &brokenReader{})
	if !errors.Is(e, io.ErrUnexpectedEOF) || out.ID != "" {
		t.Fatal(out, e)
	}
	staging, ready := counts(t, x)
	if staging != 1 || ready != 0 {
		t.Fatal(staging, ready)
	}
	matches, e := filepath.Glob(filepath.Join(x.volume, "*.part"))
	if e != nil || len(matches) != 1 {
		t.Fatal(matches, e)
	}
	b, e := os.ReadFile(matches[0])
	if e != nil || string(b) != "half" {
		t.Fatal(string(b), e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	gate := &gateReader{make(chan struct{}), make(chan struct{}), "cancelled"}
	done := make(chan error, 1)
	go func() {
		out, e := x.u.Upload(ctx, input(), gate)
		if out.ID != "" {
			done <- errors.New("published cancelled result")
			return
		}
		done <- e
	}()
	<-gate.entered
	cancel()
	close(gate.release)
	if e = <-done; !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	_, ready = counts(t, x)
	if ready != 0 {
		t.Fatal(ready)
	}
}
func TestBeginAndFinalAuthorization(t *testing.T) {
	x := setup(t)
	denied := errors.New("revoked")
	r := &countReader{r: strings.NewReader("secret")}
	x.u.write = func(context.Context, func(*sql.Tx) error) error { return denied }
	out, e := x.u.Upload(context.Background(), input(), r)
	if !errors.Is(e, denied) || out.ID != "" || r.n != 0 {
		t.Fatal(out, e, r.n)
	}
	var calls int
	x.u.write = func(ctx context.Context, fn func(*sql.Tx) error) error {
		calls++
		if calls == 2 {
			return denied
		}
		return x.s.Write(ctx, fn)
	}
	out, e = x.u.Upload(context.Background(), input(), strings.NewReader("persisted"))
	if !errors.Is(e, denied) || out.ID != "" {
		t.Fatal(out, e)
	}
	staging, ready := counts(t, x)
	if staging != 1 || ready != 0 {
		t.Fatal(staging, ready)
	}
	matches, _ := filepath.Glob(filepath.Join(x.volume, "*.ready"))
	if len(matches) != 1 {
		t.Fatal(matches)
	}
}
func TestRealCommitFailureAfterRename(t *testing.T) {
	x := setup(t)
	ctx := context.Background()
	e := x.s.Write(ctx, func(tx *sql.Tx) error {
		_, e := tx.Exec(`CREATE TABLE commit_parent(id INTEGER PRIMARY KEY); CREATE TABLE commit_child(id INTEGER REFERENCES commit_parent(id) DEFERRABLE INITIALLY DEFERRED)`)
		return e
	})
	if e != nil {
		t.Fatal(e)
	}
	calls := 0
	x.u.write = func(ctx context.Context, fn func(*sql.Tx) error) error {
		calls++
		return x.s.Write(ctx, func(tx *sql.Tx) error {
			if e := fn(tx); e != nil {
				return e
			}
			if calls == 2 {
				_, e := tx.Exec(`INSERT INTO commit_child VALUES(123)`)
				return e
			}
			return nil
		})
	}
	out, e := x.u.Upload(ctx, input(), strings.NewReader("durable"))
	if e == nil || out.ID != "" {
		t.Fatal(out, e)
	}
	staging, ready := counts(t, x)
	if staging != 1 || ready != 0 {
		t.Fatal(staging, ready)
	}
	var child int
	e = x.s.Read(ctx, func(tx *sql.Tx) error { return tx.QueryRow(`SELECT count(*) FROM commit_child`).Scan(&child) })
	if e != nil || child != 0 {
		t.Fatal(child, e)
	}
	matches, _ := filepath.Glob(filepath.Join(x.volume, "*.ready"))
	if len(matches) != 1 {
		t.Fatal(matches)
	}
	b, _ := os.ReadFile(matches[0])
	if string(b) != "durable" {
		t.Fatal(string(b))
	}
}
func TestLeaseRenewalWhileStreamBlocked(t *testing.T) {
	x := setup(t)
	x.r.lease = 300 * time.Millisecond
	gate := &gateReader{make(chan struct{}), make(chan struct{}), "data"}
	done := make(chan error, 1)
	in := input()
	go func() { _, e := x.u.Upload(context.Background(), in, gate); done <- e }()
	<-gate.entered
	// 多次实际续租后再检查仍有活动保护，时间由数据库观测而非固定睡眠断言。
	deadline := time.After(3 * time.Second)
	var first int64
	var renewed atomic.Bool
	for !renewed.Load() {
		var until int64
		e := x.s.Read(context.Background(), func(tx *sql.Tx) error { return tx.QueryRow(`SELECT lease_until FROM attachment_uploads`).Scan(&until) })
		if e != nil {
			t.Fatal(e)
		}
		if first == 0 {
			first = until
		}
		if until > first+300 {
			renewed.Store(true)
			break
		}
		select {
		case <-deadline:
			t.Fatal("lease not renewed")
		case <-time.After(20 * time.Millisecond):
		}
	}
	_, e := x.u.Upload(context.Background(), in, bytes.NewBufferString("data"))
	if !errors.Is(e, ErrActive) {
		t.Fatal(e)
	}
	close(gate.release)
	if e = <-done; e != nil {
		t.Fatal(e)
	}
}

func TestBeginCommitFailureDoesNotRead(t *testing.T) {
	x := setup(t)
	ctx := context.Background()
	e := x.s.Write(ctx, func(tx *sql.Tx) error {
		_, e := tx.Exec(`CREATE TABLE parent(id INTEGER PRIMARY KEY); CREATE TABLE child(id INTEGER REFERENCES parent(id) DEFERRABLE INITIALLY DEFERRED)`)
		return e
	})
	if e != nil {
		t.Fatal(e)
	}
	x.u.write = func(ctx context.Context, fn func(*sql.Tx) error) error {
		return x.s.Write(ctx, func(tx *sql.Tx) error {
			if e := fn(tx); e != nil {
				return e
			}
			_, e := tx.Exec(`INSERT INTO child VALUES(1)`)
			return e
		})
	}
	r := &countReader{r: strings.NewReader("unread")}
	out, e := x.u.Upload(ctx, input(), r)
	if e == nil || out.ID != "" || r.n != 0 {
		t.Fatal(out, e, r.n)
	}
	staging, ready := counts(t, x)
	if staging != 0 || ready != 0 {
		t.Fatal(staging, ready)
	}
}
func TestRenameThenSQLFailure(t *testing.T) {
	x := setup(t)
	ctx := context.Background()
	e := x.s.Write(ctx, func(tx *sql.Tx) error {
		_, e := tx.Exec(`CREATE TRIGGER fail_ready BEFORE UPDATE OF state ON attachments WHEN NEW.state='ready' BEGIN SELECT RAISE(ABORT,'injected ready failure'); END`)
		return e
	})
	if e != nil {
		t.Fatal(e)
	}
	out, e := x.u.Upload(ctx, input(), strings.NewReader("recoverable"))
	if e == nil || out.ID != "" {
		t.Fatal(out, e)
	}
	staging, ready := counts(t, x)
	if staging != 1 || ready != 0 {
		t.Fatal(staging, ready)
	}
	matches, _ := filepath.Glob(filepath.Join(x.volume, "*.ready"))
	if len(matches) != 1 {
		t.Fatal(matches)
	}
	var token string
	e = x.s.Read(ctx, func(tx *sql.Tx) error { return tx.QueryRow(`SELECT token FROM attachment_attempts`).Scan(&token) })
	if e != nil || filepath.Base(matches[0]) != token+".ready" {
		t.Fatal(token, e)
	}
}

// 真实 Bot 授权闭包证明该接缝可以复用现有不可伪造 Identity，非 HTTP 验收。
func TestControlledBotAuthorizationRevokedDuringStream(t *testing.T) {
	x := setup(t)
	ctx := context.Background()
	secret := strings.Repeat("a", 64) + "." + strings.Repeat("b", 64)
	digest := sha256.Sum256([]byte(secret))
	e := x.s.Write(ctx, func(tx *sql.Tx) error {
		_, e := tx.Exec(`INSERT INTO bot_credentials(bot_id,kind,credential_id,generation,digest) VALUES('bot','api_token',?,1,?)`, strings.Repeat("a", 64), digest[:])
		return e
	})
	if e != nil {
		t.Fatal(e)
	}
	identity, e := bots.New(x.s).Authenticate(ctx, "bot", bots.APIToken, secret)
	if e != nil {
		t.Fatal(e)
	}
	in := input()
	in.Identity = "bot:" + identity.BotID()
	x.u.write = func(ctx context.Context, fn func(*sql.Tx) error) error {
		return x.s.Write(ctx, func(tx *sql.Tx) error {
			if e := bots.AuthorizeTx(ctx, tx, identity, in.BotID, bots.Upload); e != nil {
				return e
			}
			return fn(tx)
		})
	}
	gate := &gateReader{make(chan struct{}), make(chan struct{}), "data"}
	done := make(chan error, 1)
	go func() {
		out, e := x.u.Upload(ctx, in, gate)
		if out.ID != "" {
			done <- errors.New("published revoked upload")
			return
		}
		done <- e
	}()
	<-gate.entered
	// 能在 reader 阻塞时提交撤销，证明流式过程没有占用 SQLite 写事务。
	e = x.s.Write(ctx, func(tx *sql.Tx) error {
		_, e := tx.Exec(`UPDATE bot_credentials SET generation=generation+1 WHERE bot_id='bot'`)
		return e
	})
	if e != nil {
		t.Fatal(e)
	}
	close(gate.release)
	if e = <-done; !errors.Is(e, bots.ErrUnauthorized) {
		t.Fatal(e)
	}
	staging, ready := counts(t, x)
	if staging != 1 || ready != 0 {
		t.Fatal(staging, ready)
	}
}

func TestCancellationClosesBlockedPipe(t *testing.T) {
	x := setup(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()
	done := make(chan error, 1)
	go func() {
		out, e := x.u.Upload(ctx, input(), reader)
		if out.ID != "" {
			done <- errors.New("published cancelled pipe")
			return
		}
		done <- e
	}()
	// 写入成功表明上传已开始读流，随后阻塞在下一次 Read。
	if _, e := writer.Write([]byte("half")); e != nil {
		t.Fatal(e)
	}
	cancel()
	select {
	case e := <-done:
		if !errors.Is(e, context.Canceled) {
			t.Fatal(e)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("cancelled pipe upload did not return")
	}
	_, ready := counts(t, x)
	if ready != 0 {
		t.Fatal(ready)
	}
}

func TestKeyValidationAndCaseNormalization(t *testing.T) {
	x := setup(t)
	ctx := context.Background()
	in := input()
	a, e := x.u.Upload(ctx, in, strings.NewReader("body"))
	if e != nil {
		t.Fatal(e)
	}
	in.Key = strings.ToUpper(in.Key)
	b, e := x.u.Upload(ctx, in, strings.NewReader("body"))
	if e != nil || b.ID != a.ID {
		t.Fatal(b, e)
	}
	for _, key := range []string{"00000000-0000-0000-0000-000000000000", "bad", "12345678-1234-1234-9234-123456789abc"} {
		in.Key = key
		r := &countReader{r: strings.NewReader("unread")}
		if _, e = x.u.Upload(ctx, in, r); !errors.Is(e, ErrInvalid) || r.n != 0 {
			t.Fatal(e, r.n)
		}
	}
}
