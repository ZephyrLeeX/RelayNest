//go:build !linux

package main

import (
	"context"
	"os"

	"golang.org/x/term"
)

func readTerminalPassword(ctx context.Context, file *os.File) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return term.ReadPassword(int(file.Fd()))
}
