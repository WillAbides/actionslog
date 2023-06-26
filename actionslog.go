//go:build go1.21

package actionslog

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// Log is a log level in GitHub Actions.
type Log string

const (
	LogDebug  Log = "debug"
	LogNotice Log = "notice"
	LogWarn   Log = "warning"
	LogError  Log = "error"
)

// DefaultLevelLog is the default mapping from slog.Level to Log.
func DefaultLevelLog(level slog.Level) Log {
	switch {
	case level < slog.LevelInfo:
		return LogDebug
	case level < slog.LevelWarn:
		return LogNotice
	case level < slog.LevelError:
		return LogWarn
	default:
		return LogError
	}
}

type Options struct {
	// LevelLog maps a slog.Level to a Log. Defaults to DefaultLevelLog. To write debug to
	// LogNotice instead of LogDebug, use something like:
	//    func(level slog.Level) Log {
	//      l := DefaultLevelLog.Log(level)
	//      if l == LogDebug {
	//        return LogNotice
	//      }
	//      return l
	//    }
	LevelLog func(slog.Level) Log

	// AddSource causes the handler to compute the source code position
	// of the log statement and add a SourceKey attribute to the output.
	AddSource bool

	// Level reports the minimum record level that will be logged.
	// The handler discards records with lower levels.
	// If Level is nil, the handler assumes LevelInfo.
	// The handler calls Level.Level for each record processed;
	// to adjust the minimum level dynamically, use a LevelVar.
	Level slog.Leveler
}

type Handler struct {
	attrs        []slog.Attr
	groupIndices []int
	opts         Options
	mux          *sync.Mutex
	w            io.Writer
}

var _ slog.Handler = &Handler{}

func New(w io.Writer, opts *Options) *Handler {
	if opts == nil {
		opts = new(Options)
	}
	var mux sync.Mutex
	return &Handler{
		opts: *opts,
		mux:  &mux,
		w:    w,
	}
}

func (h *Handler) Enabled(_ context.Context, level slog.Level) bool {
	l := slog.LevelInfo
	if h.opts.Level != nil {
		l = h.opts.Level.Level()
	}
	return level >= l
}

func (h *Handler) Handle(ctx context.Context, record slog.Record) error {
	if !h.Enabled(ctx, record.Level) {
		return nil
	}
	levelLog := h.opts.LevelLog
	if levelLog == nil {
		levelLog = DefaultLevelLog
	}
	actionsLog := levelLog(record.Level)

	line := "::" + string(actionsLog) + " "
	if h.opts.AddSource {
		frames := runtime.CallersFrames([]uintptr{record.PC})
		frame, _ := frames.Next()
		if frame.File != "" {
			line += "file=" + frame.File
			if frame.Line > 0 {
				line += ","
			}
		}
		if frame.Line > 0 {
			line += "line=" + strconv.Itoa(frame.Line)
		}
	}
	line += "::"
	line += escapeString(record.Message)

	attrs := h.attrs
	record.Attrs(func(attr slog.Attr) bool {
		attrs = appendAttr(attrs, attr)
		return true
	})
	if len(attrs) > 0 {
		b, err := yaml.Marshal(attrsMarshaler{attrs: attrs})
		if err != nil {
			return err
		}
		b = bytes.TrimSpace(b)
		line += escapeString("\n" + string(b))
	}
	if !strings.HasSuffix(line, "\n") {
		line += "\n"
	}

	h.mux.Lock()
	defer h.mux.Unlock()
	_, err := io.WriteString(h.w, line)
	if err != nil {
		return err
	}

	return err
}

func escapeString(val string) string {
	var line string
	for _, r := range val {
		switch r {
		case '\n':
			line += "%0A"
		case '\r':
			line += "%0D"
		case '%':
			line += "%25"
		default:
			line += string(r)
		}
	}
	return line
}

func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	h2 := Handler{
		attrs:        h.attrs,
		groupIndices: h.groupIndices,
		opts:         h.opts,
		w:            h.w,
		mux:          h.mux,
	}
	for _, attr := range attrs {
		h2.attrs, _ = appendNestedAttr(h.groupIndices, h2.attrs, attr)
	}
	return &h2
}

func (h *Handler) WithGroup(name string) slog.Handler {
	h2 := Handler{
		attrs:        h.attrs,
		groupIndices: h.groupIndices,
		opts:         h.opts,
		w:            h.w,
		mux:          h.mux,
	}
	var idx int
	h2.attrs, idx = appendNestedAttr(h2.groupIndices, h2.attrs, slog.Group(name))
	h2.groupIndices = append(h2.groupIndices, idx)
	return &h2
}

func appendNestedAttr(groupIdx []int, attrs []slog.Attr, val slog.Attr) ([]slog.Attr, int) {
	if len(groupIdx) == 0 {
		attrs = appendAttr(attrs, val)
		return attrs, len(attrs) - 1
	}
	idx := groupIdx[0]
	groupAttrs := attrs[idx].Value.Group()
	groupAttrs, idx = appendNestedAttr(groupIdx[1:], groupAttrs, val)
	attrs[idx].Value = slog.GroupValue(groupAttrs...)
	return attrs, idx
}

func appendAttr(attrs []slog.Attr, a slog.Attr) []slog.Attr {
	a.Value = a.Value.Resolve()
	// inline groups with empty keys
	if a.Value.Kind() == slog.KindGroup && a.Key == "" {
		for _, attr := range a.Value.Group() {
			attrs = appendAttr(attrs, attr)
		}
		return attrs
	}
	return append(attrs, a)
}
