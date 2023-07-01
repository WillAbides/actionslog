//go:build go1.21

package actionslog

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"math"
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

	buf bytes.Buffer

	// handler should only be accessed by withLock().
	handler slog.Handler

	// mux should only be accessed on the root Wrapper.
	mux sync.Mutex
}

func (w *Wrapper) root() *Wrapper {
	if w.parent == nil {
		return w
	}
	return w.parent.root()
}

func withLock[T any](w *Wrapper, fn func(*bytes.Buffer, slog.Handler) T) T {
	root := w.root()
	root.mux.Lock()
	defer root.mux.Unlock()
	root.buf.Reset()
	if w.handler == nil {
		handler := w.Handler
		if handler == nil {
			handler = DefaultHandler
		}
		w.handler = handler(&escapeWriter{w: &root.buf})
	}
	return fn(&root.buf, w.handler)
}

func (w *Wrapper) Enabled(ctx context.Context, level slog.Level) bool {
	if w.Level != nil {
		if level < w.Level.Level() {
			return false
		}
	}
	return withLock(w, func(_ *bytes.Buffer, h slog.Handler) bool {
		return h.Enabled(ctx, level)
	})
}

func (w *Wrapper) Handle(ctx context.Context, record slog.Record) error {
	levelLog := w.ActionsLogger
	if levelLog == nil {
		levelLog = DefaultActionsLog
	}
	actionsLog := levelLog(record.Level)
	root := w.root()
	root.mux.Lock()
	defer root.mux.Unlock()
	if w.handler == nil {
		handler := w.Handler
		if handler == nil {
			handler = DefaultHandler
		}
		w.handler = handler(&escapeWriter{w: &root.buf})
	}
	output := w.root().Output
	if output == nil {
		output = os.Stdout
	}
	buf := &root.buf
	buf.Reset()
	_, err := buf.WriteString("::" + actionsLog.String() + " ")
	if err != nil {
		return err
	}
	if w.AddSource {
		frames := runtime.CallersFrames([]uintptr{record.PC})
		frame, _ := frames.Next()
		if frame.File != "" {
			_, err = buf.WriteString("file=" + frame.File)
			if err != nil {
				return err
			}
			if frame.Line > 0 {
				err = buf.WriteByte(',')
				if err != nil {
					return err
				}
			}
		}
		if frame.Line > 0 {
			_, err = buf.WriteString("line=" + strconv.Itoa(frame.Line))
			if err != nil {
				return err
			}
		}
	}
	_, err = buf.WriteString("::")
	if err != nil {
		return err
	}
	err = w.handler.Handle(ctx, record)
	if err != nil {
		return err
	}
	// remove trailing "%0A" and "%0D" from the buffer
	for {
		b := buf.Bytes()
		if len(b) < 3 {
			break
		}
		if b[len(b)-3] != '%' {
			break
		}
		if b[len(b)-2] != '0' {
			break
		}
		if b[len(b)-1] != 'A' && b[len(b)-1] != 'D' {
			break
		}
		buf.Truncate(len(b) - 3)
	}
	err = buf.WriteByte('\n')
	if err != nil {
		return err
	}
	_, err = io.Copy(output, buf)
	if err != nil {
		return err
	}
	return err
}

func (w *Wrapper) child(fn func(slog.Handler) slog.Handler) *Wrapper {
	return withLock(w, func(_ *bytes.Buffer, h slog.Handler) *Wrapper {
		return &Wrapper{
			parent:        w,
			AddSource:     w.AddSource,
			Level:         w.Level,
			ActionsLogger: w.ActionsLogger,
			handler:       fn(w.handler),
		}
	})
}

func (w *Wrapper) WithAttrs(attrs []slog.Attr) slog.Handler {
	return w.child(func(h slog.Handler) slog.Handler {
		return h.WithAttrs(attrs)
	})
}

func (w *Wrapper) WithGroup(name string) slog.Handler {
	return w.child(func(h slog.Handler) slog.Handler {
		return h.WithGroup(name)
	})
}

type escapeWriter struct {
	w *bytes.Buffer
}

func (e *escapeWriter) Write(p []byte) (int, error) {
	n := 0
	for len(p) > 0 {
		i := bytes.IndexAny(p, "\n\r%")
		if i < 0 {
			nn, err := e.w.Write(p)
			n += nn
			if err != nil {
				return n, err
			}
			break
		}
		nn, err := e.w.Write(p[:i])
		n += nn
		if err != nil {
			return n, err
		}
		p = p[i:]
		switch p[0] {
		case '\n':
			_, err = e.w.WriteString("%0A")
		case '\r':
			_, err = e.w.WriteString("%0D")
		case '%':
			_, err = e.w.WriteString("%25")
		}
		if err != nil {
			return n, err
		}
		p = p[1:]
		n++
	}
	return n, nil
}

func DefaultHandler(w io.Writer) slog.Handler {
	return &prettyHandler{
		w: w,
		handler: slog.NewTextHandler(w, &slog.HandlerOptions{
			Level: slog.Level(math.MinInt),
			ReplaceAttr: func(groups []string, attr slog.Attr) slog.Attr {
				if len(groups) > 0 {
					return attr
				}
				switch attr.Key {
				case "time", "level", "msg":
					return slog.Attr{}
				}
				return attr
			},
		}),
	}
}

type prettyHandler struct {
	w        io.Writer
	handler  slog.Handler
	hasAttrs bool
}

func (p *prettyHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (p *prettyHandler) Handle(ctx context.Context, record slog.Record) error {
	_, err := io.WriteString(p.w, record.Message)
	if err != nil {
		return err
	}
	if !p.hasAttrs && record.NumAttrs() == 0 {
		return nil
	}
	_, err = io.WriteString(p.w, " ")
	if err != nil {
		return err
	}
	return p.handler.Handle(ctx, record)
}

func (p *prettyHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &prettyHandler{
		w:        p.w,
		handler:  p.handler.WithAttrs(attrs),
		hasAttrs: true,
	}
}

func (p *prettyHandler) WithGroup(name string) slog.Handler {
	return &prettyHandler{
		w:        p.w,
		handler:  p.handler.WithGroup(name),
		hasAttrs: p.hasAttrs,
	}
}
