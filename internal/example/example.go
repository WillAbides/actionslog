//go:build go1.21

package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/willabides/actionslog"
)

func main() {
	logger := slog.New(actionslog.New(
		os.Stdout,
		&actionslog.Options{
			AddSource: true,
			Level:     slog.LevelDebug,
		},
	)).With(slog.String("foo", "bar"))

	logger.Info("hello", slog.String("object", "world"))

	logger.Warn(`This is a stern warning
Pleas stop doing whatever you're doing'`, slog.Any("activities", []string{
		"doing bad stuff",
		"getting caught",
	}))

	logger.Error("got an error", slog.Any("err", fmt.Errorf("omg")))

	logger.Debug("this is a debug message")

	noSourceLogger := slog.New(actionslog.New(os.Stdout, nil))
	noSourceLogger.Info("this log line has no source")
}
