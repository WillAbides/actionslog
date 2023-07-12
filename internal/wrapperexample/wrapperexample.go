//go:build go1.21

package main

import (
	"io"
	"log/slog"

	"github.com/willabides/actionslog"
)

func main() {
	logger := slog.New(&actionslog.Wrapper{
		AddSource: true,
		Handler: func(w io.Writer) slog.Handler {
			return slog.NewTextHandler(w, nil)
		},
	})

	logger = logger.With(slog.String("foo", "bar"))
	logger.Info("hello", "object", "world")
}
