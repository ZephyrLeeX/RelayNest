package main

import (
	"context"
	"io"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/sys/unix"
)

func readTerminalPassword(parent context.Context, file *os.File) (password []byte, err error) {
	ctx, stop := signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
	defer stop()
	fd := int(file.Fd())
	original, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		return nil, err
	}
	// 与 x/term 的 Linux 密码模式一致；状态只在当前作用域修改，
	// 避免阻塞读取协程在取消清理后才关闭回显的竞态。
	hidden := *original
	hidden.Lflag &^= unix.ECHO
	hidden.Lflag |= unix.ICANON | unix.ISIG
	hidden.Iflag |= unix.ICRNL
	if err = unix.IoctlSetTermios(fd, unix.TCSETS, &hidden); err != nil {
		return nil, err
	}
	defer func() {
		if restoreErr := unix.IoctlSetTermios(fd, unix.TCSETS, original); restoreErr != nil {
			clear(password)
			password, err = nil, restoreErr
		}
	}()
	flags, err := unix.FcntlInt(file.Fd(), unix.F_GETFL, 0)
	if err != nil {
		return nil, err
	}
	if err = unix.SetNonblock(fd, true); err != nil {
		return nil, err
	}
	defer func() {
		if _, flagErr := unix.FcntlInt(file.Fd(), unix.F_SETFL, flags); flagErr != nil {
			clear(password)
			password, err = nil, flagErr
		}
	}()
	// 非阻塞模式防止 poll 就绪后信号清空输入，使随后的 Read 永久阻塞。
	// canonical 模式下仅在整行或 EOF 就绪时读取；短轮询使信号取消
	// 有界返回，同时无需遗留一个持有密码的阻塞读取协程。
	for {
		if err = ctx.Err(); err != nil {
			clear(password)
			return nil, err
		}
		poll := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
		_, err = unix.Poll(poll, 50)
		if err == unix.EINTR {
			continue
		}
		if err != nil {
			clear(password)
			return nil, err
		}
		if poll[0].Revents == 0 {
			continue
		}
		var b [1]byte
		n, readErr := unix.Read(fd, b[:])
		if readErr == unix.EINTR || readErr == unix.EAGAIN {
			continue
		}
		if readErr != nil {
			clear(password)
			return nil, readErr
		}
		if n == 0 || b[0] == '\n' {
			if err = ctx.Err(); err != nil {
				clear(password)
				return nil, err
			}
			if n == 0 && len(password) == 0 {
				return nil, io.EOF
			}
			return password, nil
		}
		switch b[0] {
		case '\b':
			if len(password) > 0 {
				password = password[:len(password)-1]
			}
		case '\r':
		default:
			password = append(password, b[0])
		}
	}
}
