package main

import (
	"os"
	"testing"

	"golang.org/x/exp/slog"
)

// global test setups
func TestMain(m *testing.M) {
	opts := slog.HandlerOptions{
		Level: slog.LevelError,
	}
	textHandler := slog.NewTextHandler(os.Stdout, &opts)
	slog.SetDefault(slog.New(textHandler))
	code := m.Run()
	os.Exit(code)
}

// TODO: add integration test
