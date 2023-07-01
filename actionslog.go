//go:build go1.21

package actionslog

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"runtime"
	"strconv"
	"sync"
)

// ActionsLog is a log level in GitHub Actions.
type ActionsLog int

func (a ActionsLog) String() string {
	switch a {
	case LogDebug:
		return "debug"
	case LogNotice:
		return "notice"
	case LogWarn:
		return "warning"
	case LogError:
		return "error"
	default:
		panic("invalid ActionsLog")
	}
}

const (
	LogDebug ActionsLog = iota
	LogNotice
	LogWarn
	LogError
)

// DefaultActionsLog is the default mapping from slog.Level to ActionsLog.
func DefaultActionsLog(level slog.Level) ActionsLog {
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

// Wrapper is a slog.Handler that wraps another slog.Handler and formats its output for GitHub Actions.
type Wrapper struct {
	// Handler is a function that returns the handler the Wrapper will wrap. Handler is only called once, so changes
	// after the Wrapper is created will not be reflected. Defaults to DefaultHandler.
	Handler func(w io.Writer) slog.Handler

	// Output is the io.Writer that the Wrapper will write to. Defaults to os.Stdout because that is what GitHub
	// Actions expects. Output should not be changed after the Wrapper is created.
	Output io.Writer

	// AddSource causes the Wrapper to compute the source code position
	// of the log statement so that it can be linked from the GitHub Actions UI.
	AddSource bool

	// Level sets the level for the Wrapper itself. If it is set, the Wrapper will only pass through logs that
	// are at or above this level. Handler may have its own level set as well. It is probably advisable
	// to either set it on the Wrapper or the Handler but not both.
	Level slog.Leveler

	// ActionsLogger maps a slog.Level to an ActionsLog. Defaults to DefaultActionsLog. See ExampleWrapper_writeDebugToNotice for
	// an example of a custom ActionsLogger.
	ActionsLogger func(level slog.Level) ActionsLog

	parent *Wrapper

	// only the root's buf is used.
	buf *[]byte

	// handler should only be accessed by withLock().
	handler slog.Handler

	// mux should only be accessed on the root Wrapper.
	mux sync.Mutex

	initOnce sync.Once
}

func (w *Wrapper) init() {
	w.initOnce.Do(func() {
		if w.parent != nil {
			w.parent.init()
			return
		}
		buf := make([]byte, 0, 1024)
		w.buf = &buf
		handler := w.Handler
		if handler == nil {
			handler = DefaultHandler
		}
		w.handler = handler(&escapeWriter{buf: w.buf})
	})
}

func (w *Wrapper) Enabled(ctx context.Context, level slog.Level) bool {
	w.init()
	if w.Level != nil {
		if level < w.Level.Level() {
			return false
		}
	}
	return w.handler.Enabled(ctx, level)
}

func (w *Wrapper) Handle(ctx context.Context, record slog.Record) error {
	w.init()
	levelLog := w.ActionsLogger
	if levelLog == nil {
		levelLog = DefaultActionsLog
	}
	actionsLog := levelLog(record.Level)
	root := w
	for root.parent != nil {
		root = root.parent
	}
	root.mux.Lock()
	defer root.mux.Unlock()
	output := root.Output
	if output == nil {
		output = os.Stdout
	}

	*root.buf = (*root.buf)[:0]
	*root.buf = append(*root.buf, "::"+actionsLog.String()+" "...)
	if w.AddSource {
		frames := runtime.CallersFrames([]uintptr{record.PC})
		frame, _ := frames.Next()
		if frame.File != "" {
			*root.buf = append(*root.buf, "file="...)
			*root.buf = append(*root.buf, frame.File...)
			if frame.Line > 0 {
				*root.buf = append(*root.buf, ',')
			}
		}
		if frame.Line > 0 {
			*root.buf = append(*root.buf, "line="...)
			*root.buf = strconv.AppendInt(*root.buf, int64(frame.Line), 10)
		}
	}
	*root.buf = append(*root.buf, "::"...)
	err := w.handler.Handle(ctx, record)
	if err != nil {
		return err
	}
	// remove trailing "%0A" and "%0D" from the buffer
	for {
		lb := len(*root.buf)
		if lb < 3 {
			break
		}
		if (*root.buf)[lb-3] != '%' || (*root.buf)[lb-2] != '0' {
			break
		}
		if (*root.buf)[lb-1] != 'A' && (*root.buf)[lb-1] != 'D' {
			break
		}
		*root.buf = (*root.buf)[:lb-3]
	}
	*root.buf = append(*root.buf, '\n')
	_, err = io.WriteString(output, string(*root.buf))
	return err
}

func (w *Wrapper) child(fn func(slog.Handler) slog.Handler) *Wrapper {
	return &Wrapper{
		parent:        w,
		AddSource:     w.AddSource,
		Level:         w.Level,
		ActionsLogger: w.ActionsLogger,
		handler:       fn(w.handler),
	}
}

func (w *Wrapper) WithAttrs(attrs []slog.Attr) slog.Handler {
	w.init()
	return w.child(func(h slog.Handler) slog.Handler {
		return h.WithAttrs(attrs)
	})
}

func (w *Wrapper) WithGroup(name string) slog.Handler {
	w.init()
	return w.child(func(h slog.Handler) slog.Handler {
		return h.WithGroup(name)
	})
}

type escapeWriter struct {
	buf *[]byte
}

func (e *escapeWriter) Write(p []byte) (int, error) {
	n := 0
	for len(p) > 0 {
		i := bytes.IndexAny(p, "\n\r%")
		if i < 0 {
			*e.buf = append(*e.buf, p...)
			break
		}
		*e.buf = append(*e.buf, p[:i]...)
		p = p[i:]
		n += i
		switch p[0] {
		case '\n':
			*e.buf = append(*e.buf, "%0A"...)
		case '\r':
			*e.buf = append(*e.buf, "%0D"...)
		case '%':
			*e.buf = append(*e.buf, "%25"...)
		}
		p = p[1:]
		n++
	}
	return n, nil
}
