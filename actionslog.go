package actionslog

import (
	"bytes"
	"context"
	"io"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/exp/slog"
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

	// ReplaceAttr is called to rewrite each non-group attribute before it is logged.
	// The attribute's value has been resolved (see [Value.Resolve]).
	// If ReplaceAttr returns an Attr with Key == "", the attribute is discarded.
	//
	// The built-in attributes with keys "time", "level", "source", and "msg"
	// are passed to this function, except that time is omitted
	// if zero, and source is omitted if AddSource is false.
	//
	// The first argument is a list of currently open groups that contain the
	// Attr. It must not be retained or modified. ReplaceAttr is never called
	// for Group attributes, only their contents. For example, the attribute
	// list
	//
	//     Int("a", 1), Group("g", Int("b", 2)), Int("c", 3)
	//
	// results in consecutive calls to ReplaceAttr with the following arguments:
	//
	//     nil, Int("a", 1)
	//     []string{"g"}, Int("b", 2)
	//     nil, Int("c", 3)
	//
	// ReplaceAttr can be used to change the default keys of the built-in
	// attributes, convert types (for example, to replace a `time.Time` with the
	// integer seconds since the Unix epoch), sanitize personal information, or
	// remove attributes from the output.
	ReplaceAttr func(groups []string, a slog.Attr) slog.Attr
}

type Handler struct {
	opts       Options
	mux        *sync.Mutex
	w          io.Writer
	handler    slog.Handler
	handlerBuf *bytes.Buffer
}

var _ slog.Handler = &Handler{}

func New(w io.Writer, opts *Options) *Handler {
	if opts == nil {
		opts = new(Options)
	}
	replace := func(groups []string, attr slog.Attr) slog.Attr {
		if opts.ReplaceAttr != nil {
			attr = opts.ReplaceAttr(groups, attr)
		}
		if attr.Key == "time" || attr.Key == "level" || attr.Key == "msg" {
			return slog.Attr{}
		}
		return attr
	}
	var handlerBuf bytes.Buffer
	handler := slog.NewTextHandler(&handlerBuf, &slog.HandlerOptions{
		Level:       opts.Level,
		ReplaceAttr: replace,
	})
	var mux sync.Mutex
	return &Handler{
		opts:       *opts,
		mux:        &mux,
		w:          w,
		handler:    handler,
		handlerBuf: &handlerBuf,
	}
}

func (h *Handler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

func (h *Handler) Handle(ctx context.Context, record slog.Record) error {
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
	h.mux.Lock()
	defer h.mux.Unlock()
	h.handlerBuf.Reset()
	err := h.handler.Handle(ctx, record)
	if err != nil {
		return err
	}
	handlerOut := escapeString(strings.TrimSpace(h.handlerBuf.String()))
	if line[len(line)-1] != ' ' && record.Message != "" && handlerOut != "" {
		line += " "
	}
	line += handlerOut
	line += "\n"
	_, err = io.WriteString(h.w, line)
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
	return &Handler{
		opts:       h.opts,
		w:          h.w,
		mux:        h.mux,
		handler:    h.handler.WithAttrs(attrs),
		handlerBuf: h.handlerBuf,
	}
}

func (h *Handler) WithGroup(name string) slog.Handler {
	return &Handler{
		opts:       h.opts,
		w:          h.w,
		mux:        h.mux,
		handler:    h.handler.WithGroup(name),
		handlerBuf: h.handlerBuf,
	}
}
