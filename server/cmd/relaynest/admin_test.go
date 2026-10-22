package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"relaynest/server/internal/auth"
	"relaynest/server/internal/storage"
)

func TestAdminCLI(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "admin.sqlite")
	var output bytes.Buffer
	invoke := func(args []string, password string) error {
		t.Helper()
		output.Reset()
		err := runAdmin(ctx, args, strings.NewReader(password), &output)
		if strings.Contains(output.String(), "SYNTHETIC_SECRET") || (err != nil && strings.Contains(err.Error(), "SYNTHETIC_SECRET")) {
			t.Fatal("password leaked")
		}
		return err
	}
	for _, args := range [][]string{nil, {"init"}, {"init", "--db", path}, {"init", "--db", path, "--username", "a", "--password=SYNTHETIC_SECRET"}, {"init", "--db", path, "--username", "a", "SYNTHETIC_SECRET"}, {"reset-password", "--db", path, "--SYNTHETIC_SECRET"}} {
		if err := invoke(args, "SYNTHETIC_SECRET"); err == nil {
			t.Fatal("invalid arguments accepted")
		}
	}
	args := []string{"init", "--db", path, "--username", "Admin"}
	if err := invoke(args, "\n"); err == nil {
		t.Fatal("empty password accepted")
	}
	if err := invoke(args, " SYNTHETIC_SECRET \r\n"); err != nil {
		t.Fatal(err)
	}
	if err := invoke(args, "another"); !errors.Is(err, auth.ErrAlreadyInitialized) {
		t.Fatal(err)
	}
	if err := invoke([]string{"reset-password", "--db", path}, "NEW_SYNTHETIC_SECRET\n"); err != nil {
		t.Fatal(err)
	}
	s, err := storage.Open(ctx, storage.Config{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a, err := auth.ReadAdministrator(ctx, s)
	if err != nil || a.Username != "Admin" || a.AuthGeneration != 2 || !auth.VerifyPassword([]byte("NEW_SYNTHETIC_SECRET"), a.PasswordHash) {
		t.Fatal("CLI did not persist reset", err)
	}
}

type failedInput struct{}

func (failedInput) Read([]byte) (int, error) { return 0, errors.New("SYNTHETIC_SECRET") }

func TestAdminInputFailureIsSafe(t *testing.T) {
	var out bytes.Buffer
	err := runAdmin(context.Background(), []string{"init", "--db", filepath.Join(t.TempDir(), "x"), "--username", "a"}, failedInput{}, &out)
	if err == nil || strings.Contains(err.Error()+out.String(), "SYNTHETIC_SECRET") {
		t.Fatal("input error leaked")
	}
}

func TestPasswordLineEndings(t *testing.T) {
	for _, value := range []struct{ input, want string }{{" space \n", " space "}, {" a\r\n", " a"}, {"a\r", "a\r"}, {"a", "a"}, {"\n", ""}, {"a\nb\n", "a"}} {
		got, err := readPassword(context.Background(), strings.NewReader(value.input), io.Discard)
		if err != nil || string(got) != value.want {
			t.Fatal("line ending handling changed password", err)
		}
	}
}

func TestAdminCancelledBeforePasswordDoesNotOpenDatabase(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	path := filepath.Join(t.TempDir(), "cancelled.sqlite")
	err := runAdmin(ctx, []string{"init", "--db", path, "--username", "Admin"}, strings.NewReader("SYNTHETIC_SECRET\n"), io.Discard)
	if err == nil || strings.Contains(err.Error(), "SYNTHETIC_SECRET") {
		t.Fatalf("unsafe cancellation error: %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("cancelled init touched database: %v", err)
	}
}
