//go:build go1.21

package actionslog

import (
	"context"
	"io"
	"log/slog"
	"math"
	"os"
)

// HumanHandlerOptions are options for NewHumanHandler.
type HumanHandlerOptions struct {
	// Output is the writer to write to. Defaults to os.Stderr.
	Output io.Writer

	// Level is the minimum level to log. Defaults to slog.LevelInfo.
	Level slog.Level

	// AddSource causes the handler to compute the source code position
	// of the log statement and add a "source" attribute to the output.
	AddSource bool

	// IncludeTime controls whether a record's "time" will be logged.
	IncludeTime bool

	// IncludeLevel controls whether a record's "level" will be logged.
	IncludeLevel bool
}

// HumanHandler is a slog.Handler that writes human-readable log entries.
// Because it is for human consumption, changes to the format are not
// considered breaking changes. Entries may be multi-line.
// The current format looks like this:
//
//	<message> key=value group.key="value with spaces and\nline breaks" group.key2=value2
//
// No escaping is done on the message. Attributes are handled by slog.TextHandler.
type HumanHandler struct {
	w        io.Writer
	handler  slog.Handler
	hasAttrs bool
}

func NewHumanHandler(opts *HumanHandlerOptions) *HumanHandler {
	if opts == nil {
		opts = &HumanHandlerOptions{}
	}
	output := opts.Output
	if output == nil {
		output = os.Stderr
	}
	level := opts.Level
	if level == 0 {
		level = slog.LevelInfo
	}
	replace := func(groups []string, attr slog.Attr) slog.Attr {
		if len(groups) > 0 {
			return attr
		}
		switch attr.Key {
		case slog.MessageKey:
			return slog.Attr{}
		case slog.TimeKey:
			if !opts.IncludeTime {
				return slog.Attr{}
			}
		case slog.LevelKey:
			if !opts.IncludeLevel {
				return slog.Attr{}
			}
		}
		return attr
	}
	return &HumanHandler{
		w: output,
		handler: slog.NewTextHandler(output, &slog.HandlerOptions{
			Level:       level,
			AddSource:   opts.AddSource,
			ReplaceAttr: replace,
		}),
	}
}

func (h *HumanHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

func (h *HumanHandler) Handle(ctx context.Context, record slog.Record) error {
	_, err := io.WriteString(h.w, record.Message)
	if err != nil {
		return err
	}
	if !h.hasAttrs && record.NumAttrs() == 0 {
		return nil
	}
	_, err = io.WriteString(h.w, " ")
	if err != nil {
		return err
	}
	return h.handler.Handle(ctx, record)
}

func (h *HumanHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &HumanHandler{
		w:        h.w,
		hasAttrs: h.hasAttrs || len(attrs) > 0,
		handler:  h.handler.WithAttrs(attrs),
	}
}

func (h *HumanHandler) WithGroup(name string) slog.Handler {
	return &HumanHandler{
		w:        h.w,
		hasAttrs: h.hasAttrs,
		handler:  h.handler.WithGroup(name),
	}
}

func DefaultHandler(w io.Writer) slog.Handler {
	return NewHumanHandler(&HumanHandlerOptions{
		Output: w,
		Level:  slog.Level(math.MinInt),
	})
}
