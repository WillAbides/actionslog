//go:build go1.21

package main

import (
	"io"
	"log/slog"
	"os"

	"github.com/willabides/actionslog"
	"github.com/willabides/actionslog/human"
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
	}).With(slog.String("foo", "bar"))

	logger = slog.New(human.New(&human.Options{
		Output: os.Stdout,
	})).With(slog.String("foo", "bar"))

	logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
	})).With(slog.String("foo", "bar"))

	logger.Info("greetings from your human-readable slog handler", slog.Any("config", map[string]any{
		"baz": "qux",

	}))
}
