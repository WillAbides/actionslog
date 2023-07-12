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
			return slog.NewTextHandler(w, &slog.HandlerOptions{
				ReplaceAttr: func(groups []string, attr slog.Attr) slog.Attr {
					switch attr.Key {
					case "time", "level":
						return slog.Attr{}
					}
					return attr
				},
			})
		},
	})

	logger = logger.With(slog.String("foo", "bar"))
	logger.Info("hello", "object", "world")
}
