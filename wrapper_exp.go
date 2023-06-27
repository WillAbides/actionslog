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

// Handler is a slog.Handler that wraps another slog.Handler and formats
// its output for GitHub Actions.
type Handler struct {
	opts    Options
	output  io.Writer
	parent  *Handler
	handler slog.Handler

	// These fields start with _ to indicate they should only be accessed on the root.
	_mux sync.Mutex
	_buf *bytes.Buffer
}

func (w *Handler) clone() *Handler {
	return &Handler{
		opts:   w.opts,
		parent: w,
	}
}

func (w *Handler) root() *Handler {
	if w.parent == nil {
		return w
	}
	return w.parent.root()
}

func (w *Handler) withLock(fn func(w io.Writer, handler slog.Handler)) {
	root := w.root()
	root._mux.Lock()
	if root._buf == nil {
		root._buf = &bytes.Buffer{}
	}
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
	root._mux.Unlock()
}

func (w *Handler) Enabled(ctx context.Context, level slog.Level) bool {
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

func (w *Handler) Handle(ctx context.Context, record slog.Record) error {
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

func (w *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	var handler slog.Handler
	w.withLock(func(_ io.Writer, h slog.Handler) {
		handler = h
	})
	h2 := w.clone()
	h2.handler = handler.WithAttrs(attrs)
	return h2
}

func (w *Handler) WithGroup(name string) slog.Handler {
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
