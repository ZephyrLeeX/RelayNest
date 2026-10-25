package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"syscall"
	"time"

	"modernc.org/sqlite"
	"relaynest/server/internal/auth"
	"relaynest/server/internal/storage"
)

// 返回值只表达维护结果，底层 SQL/路径/候选身份不能进入命令输出。
func runInstance(ctx context.Context, args []string, output io.Writer) int {
	arguments, err := parseInstanceArguments(args)
	if err != nil {
		return instanceResult(output, 2)
	}
	lock, err := lockInstanceDatabase(arguments.path, true)
	if err != nil {
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return instanceResult(output, 3)
		}
		if os.IsNotExist(err) {
			return instanceResult(output, 4)
		}
		return instanceResult(output, 1)
	}
	defer lock.Close()
	store, err := storage.OpenOffline(ctx, storage.Config{Path: arguments.path, BusyTimeout: time.Millisecond})
	if err != nil {
		return instanceResult(output, instanceErrorCode(err, false))
	}
	operationStarted := false
	err = store.Write(ctx, func(tx *sql.Tx) error {
		current, err := storage.ReadInstanceIDTx(ctx, tx)
		if err != nil {
			return err
		}
		if current != arguments.expectedID {
			return storage.ErrInstanceMismatch
		}
		// 身份/expected 错误已在前面区分；后续随机、更新、撤销、提交错误为操作失败。
		operationStarted = true
		if _, err := storage.RotateInstanceIDTx(ctx, tx, arguments.expectedID); err != nil {
			return err
		}
		return auth.RevokeInstanceSessionsTx(ctx, tx)
	})
	closeErr := store.Close()
	if err != nil {
		return instanceResult(output, instanceErrorCode(err, operationStarted))
	}
	if closeErr != nil {
		// 已提交不能谎称回滚，也不能引导维护者重复执行原 expected ID。
		fmt.Fprintln(output, "实例身份已更新；数据库关闭检查失败，请保持停服并人工复核")
		return 0
	}
	return instanceResult(output, 0)
}

func instanceErrorCode(err error, operationStarted bool) int {
	var sqliteError *sqlite.Error
	if errors.As(err, &sqliteError) && (sqliteError.Code()&255 == 5 || sqliteError.Code()&255 == 6) {
		return 3
	}
	if !operationStarted && (errors.Is(err, storage.ErrInstanceUnavailable) || errors.Is(err, storage.ErrInstanceMismatch)) {
		return 4
	}
	return 1
}

func instanceResult(output io.Writer, code int) int {
	texts := map[int]string{0: "实例身份已更新，请重启服务并重新登录", 1: "实例维护失败，请保持停服并人工排查", 2: "实例维护参数错误", 3: "数据库忙或独占锁不可用", 4: "实例身份或数据库迁移状态不符合要求"}
	fmt.Fprintln(output, texts[code])
	return code
}

var maintenanceUUID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
var errInstanceUsage = errors.New("instance: invalid offline maintenance arguments")

type instanceArguments struct {
	path       string
	expectedID string
}

// 独立解析避免 flag 接受重复参数或在错误中回显敏感值。
func parseInstanceArguments(args []string) (instanceArguments, error) {
	var result instanceArguments
	if len(args) == 0 || args[0] != "rotate-id" {
		return result, errInstanceUsage
	}
	seen := make(map[string]bool)
	for i := 1; i < len(args); i++ {
		name, value, equal := strings.Cut(args[i], "=")
		if seen[name] {
			return instanceArguments{}, errInstanceUsage
		}
		seen[name] = true
		switch name {
		case "--confirm-offline":
			if equal {
				return instanceArguments{}, errInstanceUsage
			}
		case "--db", "--expected-instance-id":
			if !equal {
				i++
				if i >= len(args) {
					return instanceArguments{}, errInstanceUsage
				}
				value = args[i]
			}
			if value == "" || strings.HasPrefix(value, "--") {
				return instanceArguments{}, errInstanceUsage
			}
			if name == "--db" {
				result.path = value
			} else {
				result.expectedID = value
			}
		default:
			return instanceArguments{}, errInstanceUsage
		}
	}
	if !seen["--confirm-offline"] || result.path == "" || !maintenanceUUID.MatchString(result.expectedID) {
		return instanceArguments{}, errInstanceUsage
	}
	return result, nil
}

// 在现存数据库 inode 上加非阻塞进程锁；不创建库，不把锁当停服证明。
// 文件必须保持打开直到 Store 关闭，避免事务结束前释放维护互斥。
func lockInstanceDatabase(path string, exclusive bool) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	mode := syscall.LOCK_SH | syscall.LOCK_NB
	if exclusive {
		mode = syscall.LOCK_EX | syscall.LOCK_NB
	}
	if err := syscall.Flock(int(file.Fd()), mode); err != nil {
		file.Close()
		return nil, err
	}
	return file, nil
}
