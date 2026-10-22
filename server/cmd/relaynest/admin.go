package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"golang.org/x/term"
	"relaynest/server/internal/auth"
	"relaynest/server/internal/storage"
)

const adminUsage = "用法: relaynest admin init --db PATH --username NAME | relaynest admin reset-password --db PATH；密码从终端或 stdin 单行读取"

func runAdmin(ctx context.Context, args []string, input io.Reader, output io.Writer) error {
	if len(args) == 0 || (args[0] != "init" && args[0] != "reset-password") {
		return errors.New(adminUsage)
	}
	flags := flag.NewFlagSet("admin", flag.ContinueOnError)
	// flag 默认错误包含未知参数；禁止将潜在密码参数原样写出。
	flags.SetOutput(io.Discard)
	path := flags.String("db", "", "数据库文件路径")
	var username string
	if args[0] == "init" {
		flags.StringVar(&username, "username", "", "管理员用户名")
	}
	if err := flags.Parse(args[1:]); err != nil || flags.NArg() != 0 || *path == "" || (args[0] == "init" && username == "") {
		return errors.New(adminUsage)
	}
	password, err := readPassword(ctx, input, output)
	if err != nil {
		return errors.New("admin: password input failed")
	}
	defer clear(password)
	if ctx.Err() != nil {
		return errors.New("admin: password input failed")
	}
	if len(password) == 0 {
		return errors.New("admin: password required")
	}
	store, err := storage.Open(ctx, storage.Config{Path: *path})
	if err != nil {
		return errors.New("admin: database open failed")
	}
	var operationErr error
	if args[0] == "init" {
		operationErr = auth.InitAdministrator(ctx, store, username, password)
	} else {
		operationErr = auth.ResetPassword(ctx, store, password)
	}
	closeErr := store.Close()
	if operationErr != nil {
		return operationErr
	}
	if closeErr != nil {
		return errors.New("admin: database close failed")
	}
	_, err = fmt.Fprintln(output, "管理员操作已完成")
	return err
}

func readPassword(ctx context.Context, input io.Reader, output io.Writer) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if file, ok := input.(*os.File); ok && term.IsTerminal(int(file.Fd())) {
		if _, err := fmt.Fprint(output, "密码: "); err != nil {
			return nil, err
		}
		password, err := readTerminalPassword(ctx, file)
		fmt.Fprintln(output)
		return password, err
	}
	return readPasswordLine(input)
}

func readPasswordLine(input io.Reader) ([]byte, error) {
	// stdin 只移除一个 LF/CRLF 行结束符；空格、大小写和其他字符保持原样。
	password, err := bufio.NewReader(input).ReadBytes('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		clear(password)
		return nil, err
	}
	if bytes.HasSuffix(password, []byte{'\n'}) {
		password = bytes.TrimSuffix(password, []byte{'\n'})
		password = bytes.TrimSuffix(password, []byte{'\r'})
	}
	return password, nil
}
