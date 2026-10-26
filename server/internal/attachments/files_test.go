package attachments

import (
	"context"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestLimitsActualBytesAndMaxInt64(t *testing.T) {
	for _, tc := range []struct {
		limit    int64
		body     string
		tooLarge bool
	}{{3, "abc", false}, {3, "abcd", true}, {math.MaxInt64, "payload", false}} {
		x := setup(t)
		x.u.limits = Limits{tc.limit, tc.limit}
		out, e := x.u.Upload(context.Background(), input(), strings.NewReader(tc.body))
		if tc.tooLarge {
			if !errors.Is(e, ErrTooLarge) || out.ID != "" {
				t.Fatal(out, e)
			}
		} else if e != nil || out.Size != int64(len(tc.body)) {
			t.Fatal(out, e)
		}
	}
	for _, limits := range []Limits{{0, 1}, {1, 0}, {-1, 1}} {
		x := setup(t)
		if _, e := NewUploader(x.r, x.f, limits, x.s.Write); !errors.Is(e, ErrInvalid) {
			t.Fatal(e)
		}
	}
	d := DefaultLimits()
	if d.Image != 10485760 || d.File != 52428800 {
		t.Fatal(d)
	}
}
func TestFilePathAndSymlinkSafety(t *testing.T) {
	x := setup(t)
	for _, key := range []string{"../outside", "/tmp/outside", "https://example.com/file", uuid.NewString() + "/x.part", "random.ready"} {
		if _, e := x.f.Open(key); !errors.Is(e, ErrInvalid) {
			t.Fatal(key, e)
		}
	}
	outside := filepath.Join(t.TempDir(), "outside")
	if e := os.WriteFile(outside, []byte("unchanged"), 0600); e != nil {
		t.Fatal(e)
	}
	token := uuid.NewString()
	if e := os.Symlink(outside, filepath.Join(x.volume, token+".part")); e != nil {
		t.Fatal(e)
	}
	if _, _, e := x.f.write(context.Background(), token, strings.NewReader("overwrite"), 100); e == nil {
		t.Fatal("followed symlink")
	}
	if _, e := x.f.Open(token + ".part"); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	if e := x.f.publish(token); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	b, _ := os.ReadFile(outside)
	if string(b) != "unchanged" {
		t.Fatal(string(b))
	}
	link := filepath.Join(t.TempDir(), "volume-link")
	if e := os.Symlink(x.volume, link); e != nil {
		t.Fatal(e)
	}
	if f, e := OpenFiles(link); e == nil {
		f.Close()
		t.Fatal("accepted symlink volume")
	}
	// 独立随机尝试无法覆盖已有最终文件，原名不会被拼接到物理路径。
	token = uuid.NewString()
	_, _, e := x.f.write(context.Background(), token, strings.NewReader("fresh"), 100)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(x.volume, token+".ready"), []byte("existing"), 0600); e != nil {
		t.Fatal(e)
	}
	if e = x.f.publish(token); !errors.Is(e, ErrConflict) {
		t.Fatal(e)
	}
	file, e := x.f.Open(token + ".ready")
	if e != nil {
		t.Fatal(e)
	}
	b, e = io.ReadAll(file)
	file.Close()
	if e != nil || string(b) != "existing" {
		t.Fatal(string(b), e)
	}
}

func TestAncestorSymlinkRejectedBeforeCreatingVolume(t *testing.T) {
	target := t.TempDir()
	link := filepath.Join(t.TempDir(), "link")
	if e := os.Symlink(target, link); e != nil {
		t.Fatal(e)
	}
	if f, e := OpenFiles(filepath.Join(link, "new-volume")); e == nil {
		f.Close()
		t.Fatal("accepted ancestor symlink")
	}
	if _, e := os.Stat(filepath.Join(target, "new-volume")); !errors.Is(e, os.ErrNotExist) {
		t.Fatal("created directory through symlink", e)
	}
}
