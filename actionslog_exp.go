// Code generated by script/generate. DO NOT EDIT.

//go:build !go1.21

package actionslog

import (
	"bytes"
	"context"
	"golang.org/x/exp/slog"
	"io"
	"os"
	"runtime"
	"strconv"
	"sync"
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

// Options is configuration for a Wrapper.
type Options struct {
	// LevelLog maps a slog.Level to a Log. Defaults to DefaultLevelLog. To write debug to
	// LogNotice instead of LogDebug, use something like:
	//    func(level slog.Level) Log {
	//      l := actionslog.DefaultLevelLog.Log(level)
	//      if l == actionslog.LogDebug {
	//        return actionslog.LogNotice
	//      }
	//      return l
	//    }
	LevelLog func(slog.Level) Log

	// AddSource causes the handler to compute the source code position
	// of the log statement so that it can be linked from the GitHub Actions UI.
	AddSource bool

	// Level sets the level for the Wrapper itself. If it is set, the Wrapper will only pass through logs that
	// are at or above this level. Handler may have its own level set as well. It is probably advisable
	// to either set it on the Wrapper or the Handler but not both.
	Level slog.Leveler

	// Handler is a function that returns the inner slog.Handler that will be used to format the parts
	// of the log that aren't specific to GitHub Actions. Default is DefaultHandler.
	Handler func(w io.Writer) slog.Handler
}

// Wrapper is a slog.Handler that wraps another slog.Handler and formats its output for GitHub Actions.
type Wrapper struct {
	opts   Options
	output io.Writer
	parent *Wrapper

	// handler should only be accessed by withLock().
	handler slog.Handler

	// mux should only be accessed on the root Wrapper.
	mux sync.Mutex
}

// New returns a new Wrapper.
func New(w io.Writer, opts *Options) slog.Handler {
	if opts == nil {
		opts = &Options{}
	}

	return &Wrapper{
		opts:   *opts,
		output: w,
	}
}

func (w *Wrapper) clone() *Wrapper {
	return &Wrapper{
		opts:   w.opts,
		parent: w,
	}
}

func (w *Wrapper) root() *Wrapper {
	if w.parent == nil {
		return w
	}
	return w.parent.root()
}

func (w *Wrapper) withLock(fn func(w io.Writer, handler slog.Handler)) {
	root := w.root()
	root.mux.Lock()
	out := root.output
	if out == nil {
		out = os.Stdout
	}
	if w.handler == nil {
		if w.opts.Handler != nil {
			w.handler = w.opts.Handler(&escapeWriter{w: out})
		} else {
			w.handler = DefaultHandler(&escapeWriter{w: out})
		}
	}
	fn(out, w.handler)
	root.mux.Unlock()
}

func (w *Wrapper) Enabled(ctx context.Context, level slog.Level) bool {
	if w.opts.Level != nil {
		if level < w.opts.Level.Level() {
			return false
		}
	}
	var handler slog.Handler
	w.withLock(func(_ io.Writer, h slog.Handler) {
		handler = h
	})
	return handler.Enabled(ctx, level)
}

func (w *Wrapper) Handle(ctx context.Context, record slog.Record) error {
	levelLog := w.opts.LevelLog
	if levelLog == nil {
		levelLog = DefaultLevelLog
	}
	actionsLog := levelLog(record.Level)

	line := "::" + string(actionsLog) + " "
	if w.opts.AddSource {
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

	var err error
	w.withLock(func(writer io.Writer, handler slog.Handler) {
		_, err = writer.Write([]byte(line))
		if err != nil {
			return
		}
		err = handler.Handle(ctx, record)
		if err != nil {
			return
		}
		_, err = writer.Write([]byte{'\n'})
	})
	return err
}

func (w *Wrapper) WithAttrs(attrs []slog.Attr) slog.Handler {
	var handler slog.Handler
	w.withLock(func(_ io.Writer, h slog.Handler) {
		handler = h
	})
	h2 := w.clone()
	h2.handler = handler.WithAttrs(attrs)
	return h2
}

func (w *Wrapper) WithGroup(name string) slog.Handler {
	var handler slog.Handler
	w.withLock(func(_ io.Writer, h slog.Handler) {
		handler = h
	})
	h2 := w.clone()
	h2.handler = handler.WithGroup(name)
	return h2
}

var (
	escapedNL      = []byte("%0A")
	escapedCR      = []byte("%0D")
	escapedPercent = []byte("%25")
)

type escapeWriter struct {
	w io.Writer
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
			// skip trailing newline
			_, err = e.w.Write(escapedNL)
		case '\r':
			_, err = e.w.Write(escapedCR)
		case '%':
			_, err = e.w.Write(escapedPercent)
		}
		if err != nil {
			return n, err
		}
		p = p[1:]
		n++
	}
	return n, nil
}
