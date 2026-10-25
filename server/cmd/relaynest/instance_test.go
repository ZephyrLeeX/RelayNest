package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"relaynest/server/internal/auth"
	"relaynest/server/internal/storage"
)

func TestInstanceRevokeAtomic(t *testing.T) {
	for _, mode := range []string{"success", "rollback", "exhausted", "delete-failure"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			path := filepath.Join(t.TempDir(), "db")
			store, err := storage.Open(ctx, storage.Config{Path: path})
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			generation := int64(7)
			if mode == "exhausted" {
				generation = math.MaxInt64
			}
			var oldID string
			err = store.Write(ctx, func(tx *sql.Tx) error {
				var err error
				oldID, err = storage.ReadInstanceIDTx(ctx, tx)
				if err != nil {
					return err
				}
				if _, err = tx.ExecContext(ctx, "INSERT INTO administrator(id,username,password_hash,auth_generation) VALUES(1,'admin','unchanged',?)", generation); err != nil {
					return err
				}
				for i, client := range []string{"web", "android"} {
					_, err = tx.ExecContext(ctx, "INSERT INTO sessions(id,token_hash,auth_generation,client_type,device_id,device_name,idle_expires_at,absolute_expires_at,revoked) VALUES(?,randomblob(32),?,?,?,'temporary',1,2,?)", client, generation, client, client, i)
					if err != nil {
						return err
					}
				}
				if mode == "delete-failure" {
					_, err = tx.ExecContext(ctx, "CREATE TRIGGER deny_delete BEFORE DELETE ON sessions BEGIN SELECT RAISE(ABORT,'test failure'); END")
				}
				return err
			})
			if err != nil {
				t.Fatal(err)
			}
			err = store.Write(ctx, func(tx *sql.Tx) error {
				if _, err := storage.RotateInstanceIDTx(ctx, tx, oldID); err != nil {
					return err
				}
				if err := auth.RevokeInstanceSessionsTx(ctx, tx); err != nil {
					return err
				}
				if mode == "rollback" {
					return errors.New("transaction failure")
				}
				return nil
			})
			if mode == "success" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil {
				t.Fatal("failure committed")
			}
			// 成功换 ID 后旧 Store 拒绝读写，重开后核对已提交状态。
			store.Close()
			reopened, err := storage.Open(ctx, storage.Config{Path: path})
			if err != nil {
				t.Fatal(err)
			}
			defer reopened.Close()
			err = reopened.Read(ctx, func(tx *sql.Tx) error {
				id, err := storage.ReadInstanceIDTx(ctx, tx)
				if err != nil {
					return err
				}
				var count int
				var current int64
				var hash string
				if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM sessions").Scan(&count); err != nil {
					return err
				}
				if err = tx.QueryRowContext(ctx, "SELECT auth_generation,password_hash FROM administrator WHERE id=1").Scan(&current, &hash); err != nil {
					return err
				}
				if hash != "unchanged" {
					t.Fatal("password changed")
				}
				if mode == "success" {
					if id == oldID || count != 0 || current != generation+1 {
						t.Fatal("incomplete rotation")
					}
				} else if id != oldID || count != 2 || current != generation {
					t.Fatal("partial rollback")
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestInstanceArguments(t *testing.T) {
	valid := []string{"rotate-id", "--db", "private/path", "--expected-instance-id", "65aebbc4-f899-4277-b580-bf7a52a1209c", "--confirm-offline"}
	got, err := parseInstanceArguments(valid)
	if err != nil || got.path != "private/path" {
		t.Fatal("valid arguments rejected")
	}
	cases := [][]string{
		nil, {"rotate-id"}, valid[:len(valid)-1],
		append(append([]string{}, valid...), "--db", "another"),
		append(append([]string{}, valid...), "--confirm-offline"),
		append(append([]string{}, valid...), "--new-instance-id=secret"),
		append(append([]string{}, valid...), "positional"),
		{"rotate-id", "--db=x", "--expected-instance-id=invalid", "--confirm-offline"},
		{"rotate-id", "--db=x", "--expected-instance-id=65aebbc4-f899-4277-b580-bf7a52a1209c", "--confirm-offline=false"},
	}
	for i, args := range cases {
		if _, err := parseInstanceArguments(args); err != errInstanceUsage {
			t.Fatalf("case %d accepted", i)
		}
	}
}

func TestInstanceDatabaseLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "database")
	if _, err := lockInstanceDatabase(path, true); !os.IsNotExist(err) {
		t.Fatal("missing database must not be created")
	}
	if err := os.WriteFile(path, []byte("temporary"), 0600); err != nil {
		t.Fatal(err)
	}
	shared, err := lockInstanceDatabase(path, false)
	if err != nil {
		t.Fatal(err)
	}
	if lock, err := lockInstanceDatabase(path, true); err == nil {
		lock.Close()
		t.Fatal("running process lock accepted maintenance")
	}
	shared.Close()
	exclusive, err := lockInstanceDatabase(path, true)
	if err != nil {
		t.Fatal(err)
	}
	defer exclusive.Close()
	if lock, err := lockInstanceDatabase(path, true); err == nil {
		lock.Close()
		t.Fatal("concurrent maintenance accepted")
	}
}

// 临时库使用真实会话生成/认证接口，不用静态 token 假装登录成功。
func maintenanceFixture(t *testing.T) (string, string, []auth.SessionSecrets) {
	t.Helper()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "private-db")
	store, err := storage.Open(ctx, storage.Config{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	var id string
	err = store.Write(ctx, func(tx *sql.Tx) error {
		var err error
		id, err = storage.ReadInstanceIDTx(ctx, tx)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO administrator(id,username,password_hash,auth_generation) VALUES(1,'admin','unchanged',1)"); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO bots(id,name,enabled,can_send,can_receive,version) VALUES('bot','bot',1,1,1,1)"); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, "INSERT INTO bot_credentials(bot_id,kind,credential_id,generation,digest) VALUES('bot','api_token','preserve',1,randomblob(32))")
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	sessions := auth.NewSessionStore(store)
	var secrets []auth.SessionSecrets
	for _, client := range []string{"web", "android"} {
		input := auth.SessionInput{ClientType: client, DeviceName: "temporary"}
		if client == "android" {
			input.DeviceID = "65aebbc4-f899-4277-b580-bf7a52a1209c"
		}
		_, secret, err := sessions.Create(ctx, 1, input)
		if err != nil {
			t.Fatal(err)
		}
		secrets = append(secrets, secret)
	}
	// 集成真实 0006 后自动填入真实待办；基础版无此表，不伪造迁移。
	err = store.Write(ctx, func(tx *sql.Tx) error {
		var exists bool
		if err := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM sqlite_schema WHERE type='table' AND name='push_jobs')").Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return nil
		}
		_, err := tx.ExecContext(ctx, "INSERT INTO push_jobs(id,session_id,device_id,target_change_seq,next_attempt_at) SELECT 'maintenance-test',id,device_id,1,'temporary' FROM sessions WHERE client_type='android'")
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	return path, id, secrets
}

func invokeMaintenance(path, id string) (int, string) {
	var output strings.Builder
	code := runInstance(context.Background(), []string{"rotate-id", "--db", path, "--expected-instance-id", id, "--confirm-offline"}, &output)
	return code, output.String()
}

func databaseState(t *testing.T, path string) string {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var id, epoch, hash, secret string
	var generation, count int64
	if err = db.QueryRow("SELECT instance_id FROM instance_identity").Scan(&id); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow("SELECT sync_epoch FROM system_state").Scan(&epoch); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow("SELECT password_hash,auth_generation FROM administrator").Scan(&hash, &generation); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow("SELECT hex(digest) FROM bot_credentials").Scan(&secret); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow("SELECT count(*) FROM sessions").Scan(&count); err != nil {
		t.Fatal(err)
	}
	var exists bool
	if err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM sqlite_schema WHERE type='table' AND name='push_jobs')").Scan(&exists); err != nil {
		t.Fatal(err)
	}
	pending := -1
	if exists {
		if err = db.QueryRow("SELECT count(*) FROM push_jobs").Scan(&pending); err != nil {
			t.Fatal(err)
		}
	}
	return fmt.Sprintf("%s|%s|%s|%s|%d|%d|%d", id, epoch, hash, secret, generation, count, pending)
}

func TestInstanceCLIAtomicFailures(t *testing.T) {
	for _, mode := range []string{"expected", "random", "write", "revoke", "commit", "busy", "exhausted"} {
		t.Run(mode, func(t *testing.T) {
			path, id, _ := maintenanceFixture(t)
			before := databaseState(t, path)
			db, err := sql.Open("sqlite", path)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			expectedCode := 1
			switch mode {
			case "exhausted":
				_, err = db.Exec("UPDATE administrator SET auth_generation=?", int64(math.MaxInt64))
				before = databaseState(t, path)
			case "expected":
				id = "65aebbc4-f899-4277-b580-bf7a52a1209c"
				expectedCode = 4
			case "random":
				old := rand.Reader
				rand.Reader = failedRandom{}
				defer func() { rand.Reader = old }()
			case "write":
				_, err = db.Exec("CREATE TRIGGER deny_update BEFORE UPDATE ON instance_identity BEGIN SELECT RAISE(ABORT,'test failure'); END")
			case "revoke":
				_, err = db.Exec("CREATE TRIGGER deny_delete BEFORE DELETE ON sessions BEGIN SELECT RAISE(ABORT,'test failure'); END")
			case "commit":
				_, err = db.Exec("CREATE TABLE commit_parent(id INTEGER PRIMARY KEY); CREATE TABLE commit_child(id INTEGER REFERENCES commit_parent(id) DEFERRABLE INITIALLY DEFERRED); CREATE TRIGGER fail_commit AFTER DELETE ON sessions BEGIN INSERT INTO commit_child(id) VALUES(99); END")
			case "busy":
				_, err = db.Exec("BEGIN IMMEDIATE")
				defer db.Exec("ROLLBACK")
				expectedCode = 3
			}
			if err != nil {
				t.Fatal(err)
			}
			code, output := invokeMaintenance(path, id)
			if code != expectedCode {
				t.Fatalf("unexpected exit %d, expected %d", code, expectedCode)
			}
			if strings.Contains(output, path) || strings.Contains(output, id) || strings.Contains(output, "test failure") {
				t.Fatal("sensitive output")
			}
			if after := databaseState(t, path); before != after {
				t.Fatal("failed operation changed database")
			}
		})
	}
}

type failedRandom struct{}

func (failedRandom) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func TestInstanceRestoreCloneAndMove(t *testing.T) {
	for _, mode := range []string{"backup-restore", "clone", "normal-move"} {
		t.Run(mode, func(t *testing.T) {
			original, id, secrets := maintenanceFixture(t)
			before := strings.Split(databaseState(t, original), "|")
			target := filepath.Join(t.TempDir(), "restored-db")
			if mode == "normal-move" {
				if err := os.Rename(original, target); err != nil {
					t.Fatal(err)
				}
			} else {
				data, err := os.ReadFile(original)
				if err != nil {
					t.Fatal(err)
				}
				if err = os.WriteFile(target, data, 0600); err != nil {
					t.Fatal(err)
				}
				// 备份之后源库撤销会话；旧备份会重新带回仍有效的会话，维护须消除此风险。
				source, err := storage.Open(context.Background(), storage.Config{Path: original})
				if err != nil {
					t.Fatal(err)
				}
				if err = auth.NewSessionStore(source).Revoke(context.Background(), secrets[0].Token, "web"); err != nil {
					t.Fatal(err)
				}
				source.Close()
				if code, _ := invokeMaintenance(target, id); code != 0 {
					t.Fatalf("rotation exit %d", code)
				}
			}
			reopened, err := storage.Open(context.Background(), storage.Config{Path: target})
			if err != nil {
				t.Fatal(err)
			}
			defer reopened.Close()
			sessions := auth.NewSessionStore(reopened)
			for i, client := range []string{"web", "android"} {
				_, _, err := sessions.Inspect(context.Background(), secrets[i].Token, client)
				if mode == "normal-move" {
					if err != nil {
						t.Fatal("normal move revoked valid session")
					}
				} else if !errors.Is(err, auth.ErrInvalidSession) {
					t.Fatal("old session survived restoration")
				}
			}
			after := strings.Split(databaseState(t, target), "|")
			if before[6] != "-1" {
				expected := "0"
				if mode == "normal-move" {
					expected = "1"
				}
				if before[6] != "1" || after[6] != expected {
					t.Fatal("push jobs were not preserved or revoked with sessions")
				}
			}
			if before[1] != after[1] || before[2] != after[2] || before[3] != after[3] {
				t.Fatal("epoch/password/bot secret changed")
			}
			if mode == "normal-move" {
				if before[0] != after[0] || after[4] != "1" || after[5] != "2" {
					t.Fatal("move changed identity")
				}
			} else if before[0] == after[0] || after[4] != "2" || after[5] != "0" {
				t.Fatal("incomplete restore rotation")
			}
		})
	}
}

// 拒绝路径逐字节核对已关闭连接的数据库，确保没有迁移、补台账或换身份。
func TestInstanceRejectIncompleteDatabase(t *testing.T) {
	cases := map[string]string{
		"missing-file": "", "empty": "", "missing-ledger": "DROP TABLE schema_migrations",
		"partial":    "DELETE FROM schema_migrations WHERE version=(SELECT max(version) FROM schema_migrations)",
		"unknown":    "INSERT INTO schema_migrations(version,name,checksum,applied_at) VALUES(999,'unknown','unknown',0)",
		"name":       "UPDATE schema_migrations SET name='unknown' WHERE version=1",
		"checksum":   "UPDATE schema_migrations SET checksum='unknown' WHERE version=1",
		"missing-id": "DELETE FROM instance_identity",
		"invalid-id": "PRAGMA ignore_check_constraints=ON; UPDATE instance_identity SET instance_id='corrupt'",
	}
	for mode, statement := range cases {
		t.Run(mode, func(t *testing.T) {
			path, id, _ := maintenanceFixture(t)
			db, err := sql.Open("sqlite", path)
			if err != nil {
				t.Fatal(err)
			}
			if statement != "" {
				if _, err = db.Exec(statement); err != nil {
					t.Fatal(err)
				}
			}
			if err = db.Close(); err != nil {
				t.Fatal(err)
			}
			if mode == "missing-file" {
				if err = os.Remove(path); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "empty" {
				if err = os.WriteFile(path, nil, 0600); err != nil {
					t.Fatal(err)
				}
			}
			before, beforeErr := os.ReadFile(path)
			code, output := invokeMaintenance(path, id)
			if code != 4 {
				t.Fatalf("exit %d", code)
			}
			if strings.Contains(output, path) || strings.Contains(output, id) {
				t.Fatal("sensitive output")
			}
			after, afterErr := os.ReadFile(path)
			if mode == "missing-file" {
				if !os.IsNotExist(beforeErr) || !os.IsNotExist(afterErr) {
					t.Fatal("missing database created")
				}
			} else if beforeErr != nil || afterErr != nil || !bytes.Equal(before, after) {
				t.Fatal("rejected database changed")
			}
		})
	}
}

// 子进程在安全随机读取处暂停，观察真实维护命令持有的排他锁。
func TestInstanceMaintenanceProcessHelper(t *testing.T) {
	if os.Getenv("RELAYNEST_MAINTENANCE_HELPER") != "1" {
		return
	}
	rand.Reader = waitingRandom{rand.Reader}
	os.Exit(runInstance(context.Background(), []string{"rotate-id", "--db", os.Getenv("RELAYNEST_MAINTENANCE_DB"), "--expected-instance-id", os.Getenv("RELAYNEST_MAINTENANCE_ID"), "--confirm-offline"}, os.Stderr))
}

type waitingRandom struct{ source io.Reader }

func (r waitingRandom) Read(b []byte) (int, error) {
	if _, err := fmt.Fprintln(os.Stdout, "locked"); err != nil {
		return 0, err
	}
	var signal [1]byte
	if _, err := io.ReadFull(os.Stdin, signal[:]); err != nil {
		return 0, err
	}
	return r.source.Read(b)
}
func maintenanceRejected(t *testing.T, path, id string) {
	t.Helper()
	raw, err := mainProcess("instance", "rotate-id", "--db", path, "--expected-instance-id", id, "--confirm-offline").CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 3 || strings.Contains(string(raw), id) || strings.Contains(string(raw), path) {
		t.Fatal("incorrect concurrent rejection")
	}
}
func TestInstanceMaintenanceProcessExclusion(t *testing.T) {
	path, id, _ := maintenanceFixture(t)
	before := databaseState(t, path)
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Link(path, alias); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestInstanceMaintenanceProcessHelper$")
	cmd.Env = append(os.Environ(), "RELAYNEST_MAINTENANCE_HELPER=1", "RELAYNEST_MAINTENANCE_DB="+path, "RELAYNEST_MAINTENANCE_ID="+id)
	input, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	stopped := false
	defer func() {
		if !stopped {
			cmd.Process.Kill()
			<-done
		}
	}()
	ready := make(chan string, 1)
	go func() { line, _ := bufio.NewReader(output).ReadString('\n'); ready <- line }()
	select {
	case line := <-ready:
		if line != "locked\n" {
			t.Fatal("maintenance did not reach random source")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("maintenance timeout")
	}
	maintenanceRejected(t, alias, id)
	if err := mainProcess("-db", alias, "-external-origin", "https://relay.example", "-listen", "127.0.0.1:0").Run(); err == nil {
		t.Fatal("server started during maintenance")
	}
	if databaseState(t, path) != before {
		t.Fatal("rejected process changed database")
	}
	if _, err = input.Write([]byte{1}); err != nil {
		t.Fatal(err)
	}
	input.Close()
	select {
	case err := <-done:
		stopped = true
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("maintenance commit timeout")
	}
	if databaseState(t, path) == before {
		t.Fatal("maintenance did not commit")
	}
	assertNoDatabaseFDs(t, filepath.Dir(path))
}
func TestInstanceRunningServerRejectsMaintenance(t *testing.T) {
	path, id, _ := maintenanceFixture(t)
	before := databaseState(t, path)
	reservation, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := reservation.Addr().String()
	reservation.Close()
	cmd := mainProcess("-db", path, "-external-origin", "https://relay.example", "-listen", addr)
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	stopped := false
	defer func() {
		if !stopped {
			cmd.Process.Kill()
			<-done
		}
	}()
	client := &http.Client{Timeout: time.Second}
	deadline := time.Now().Add(10 * time.Second)
	for {
		res, err := client.Get("http://" + addr + "/health/live")
		if err == nil {
			res.Body.Close()
			if res.StatusCode != 200 {
				t.Fatal(res.StatusCode)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("server startup timeout")
		}
		time.Sleep(20 * time.Millisecond)
	}
	maintenanceRejected(t, path, id)
	if databaseState(t, path) != before {
		t.Fatal("rejection changed database")
	}
	if err = cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	select {
	case err = <-done:
		stopped = true
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("server shutdown timeout")
	}
	if code, _ := invokeMaintenance(path, id); code != 0 {
		t.Fatalf("shutdown did not release lock: %d", code)
	}
}

func TestInstanceMainProcess(t *testing.T) {
	path, id, _ := maintenanceFixture(t)
	before := databaseState(t, path)
	args := []string{"instance", "rotate-id", "--db", path, "--expected-instance-id", id, "--confirm-offline"}
	for _, expected := range []int{0, 4, 2} {
		current := args
		if expected == 2 {
			current = args[:len(args)-1]
		}
		raw, err := mainProcess(current...).CombinedOutput()
		code := 0
		if err != nil {
			var exit *exec.ExitError
			if !errors.As(err, &exit) {
				t.Fatal(err)
			}
			code = exit.ExitCode()
		}
		if code != expected {
			t.Fatalf("exit %d, expected %d", code, expected)
		}
		after := databaseState(t, path)
		if expected == 0 && after == before {
			t.Fatal("successful command did not commit")
		}
		if expected != 0 && after != before {
			t.Fatal("rejected command changed database")
		}
		newID := strings.Split(after, "|")[0]
		if strings.Contains(string(raw), path) || strings.Contains(string(raw), id) || strings.Contains(string(raw), newID) {
			t.Fatal("sensitive command output")
		}
		before = after
	}
}
