//go:build go1.21

package main

import (
	"fmt"
	"io"
	"log/slog"

	"github.com/willabides/actionslog"
	"github.com/willabides/actionslog/human"
)

func main() {
	logger := slog.New(&actionslog.Wrapper{
		AddSource: true,
		Handler: func(w io.Writer) slog.Handler {
			return human.New(&human.Options{
				Output: w,
				Level:  slog.LevelDebug,
			})
		},
	}).With(slog.String("foo", "bar"))

	logger.Info("hello", slog.String("object", "world"))

	logger.Warn(`This is a stern warning.
Please stop doing whatever you're doing`, slog.Any("activities", []string{
		"doing bad stuff",
		"getting caught",
	}))

	logger.Error("got an error", slog.Any("err", fmt.Errorf("omg")))

	logger.Debug("this is a debug message")

	noSourceLogger := slog.New(&actionslog.Wrapper{})
	noSourceLogger.Info("this log line has no source")
}
