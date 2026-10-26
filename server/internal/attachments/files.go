package attachments

import (
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// Files 持有独立附件卷句柄，子路径仅允许单段随机键。卷与父目录须由服务账号独占写入。
type Files struct{ root *os.Root }

func OpenFiles(path string) (*Files, error) {
	abs, e := filepath.Abs(path)
	if e != nil {
		return nil, e
	}
	// 逐级创建前先拒绝符号链接，避免经过父目录链接创建卷外子目录。
	current := string(filepath.Separator)
	for _, component := range strings.Split(strings.TrimPrefix(abs, current), string(filepath.Separator)) {
		if component == "" {
			continue
		}
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			if err = os.Mkdir(current, 0700); err != nil && !errors.Is(err, os.ErrExist) {
				return nil, err
			}
			info, err = os.Lstat(current)
		}
		if err != nil {
			return nil, err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return nil, ErrInvalid
		}
	}
	info, e := os.Lstat(abs)
	if e != nil {
		return nil, e
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, ErrInvalid
	}
	root, e := os.OpenRoot(abs)
	if e != nil {
		return nil, e
	}
	return &Files{root}, nil
}
func (f *Files) Close() error { return f.root.Close() }
func validFile(key string) bool {
	if len(key) != 42 && len(key) != 41 {
		return false
	}
	ext := filepath.Ext(key)
	if ext != ".part" && ext != ".ready" {
		return false
	}
	token := key[:len(key)-len(ext)]
	id, e := uuid.Parse(token)
	return e == nil && id.String() == token
}

// Open 仅用于已授权下载；拒绝路径穿越、符号链接和文件替换。
func (f *Files) Open(key string) (*os.File, error) {
	if !validFile(key) {
		return nil, ErrInvalid
	}
	info, e := f.root.Lstat(key)
	if e != nil {
		return nil, e
	}
	if !info.Mode().IsRegular() {
		return nil, ErrInvalid
	}
	file, e := f.root.Open(key)
	if e != nil {
		return nil, e
	}
	actual, e := file.Stat()
	if e != nil || !os.SameFile(info, actual) {
		file.Close()
		return nil, ErrInvalid
	}
	return file, nil
}

// write 使用固定缓冲；最大 int64 限制采用单字节超限探测，不计算 limit+1。
// 失败保留半文件供 BE-05-04 恢复，绝不据半文件报告成功。
func (f *Files) write(ctx context.Context, token string, src io.Reader, limit int64) (int64, []byte, error) {
	if src == nil || limit <= 0 || !validFile(token+".part") {
		return 0, nil, ErrInvalid
	}
	file, e := f.root.OpenFile(token+".part", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return 0, nil, e
	}
	defer file.Close()
	h := sha256.New()
	buf := make([]byte, 32*1024)
	var total int64
	empty := 0
	for {
		if e = ctx.Err(); e != nil {
			return total, nil, e
		}
		want := int64(len(buf))
		remaining := limit - total
		if remaining < want {
			want = remaining
		}
		if want == 0 {
			want = 1
		}
		n, readErr := src.Read(buf[:int(want)])
		if n < 0 || n > int(want) {
			return total, nil, ErrInvalid
		}
		if int64(n) > remaining {
			return total, nil, ErrTooLarge
		}
		if n > 0 {
			empty = 0
			if _, e = file.Write(buf[:n]); e != nil {
				return total, nil, e
			}
			h.Write(buf[:n])
			total += int64(n)
		} else {
			empty++
			if empty >= 100 {
				return total, nil, io.ErrNoProgress
			}
		}
		if e = ctx.Err(); e != nil {
			return total, nil, e
		}
		if readErr != nil {
			if !errors.Is(readErr, io.EOF) {
				return total, nil, readErr
			}
			break
		}
	}
	if e = file.Sync(); e != nil {
		return total, nil, e
	}
	if e = file.Close(); e != nil {
		return total, nil, e
	}
	return total, h.Sum(nil), nil
}
func (f *Files) publish(token string) error {
	if !validFile(token + ".part") {
		return ErrInvalid
	}
	info, e := f.root.Lstat(token + ".part")
	if e != nil {
		return e
	}
	if !info.Mode().IsRegular() {
		return ErrInvalid
	}
	if _, e = f.root.Lstat(token + ".ready"); !errors.Is(e, os.ErrNotExist) {
		if e != nil {
			return e
		}
		return ErrConflict
	}
	if e = f.root.Rename(token+".part", token+".ready"); e != nil {
		return e
	}
	dir, e := f.root.Open(".")
	if e != nil {
		return e
	}
	defer dir.Close()
	return dir.Sync()
}
