package actionslog

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"runtime"
	"strconv"
	"sync"
)

type Wrapper struct {
	handler    slog.Handler
	writer     io.Writer
	linkSource bool
	mux        sync.Mutex
	buf        *[]byte
	parent     *Wrapper
}

func NewWrapper(w io.Writer, handler func(io.Writer) slog.Handler) *Wrapper {
	return &Wrapper{
		handler: handler(&escapeWriter{w: w}),
		writer:  w,
	}
}

func (w *Wrapper) Handle(ctx context.Context, record slog.Record) error {
	line := "::error"
	switch {
	case record.Level < slog.LevelInfo:
		line = "::debug"
	case record.Level < slog.LevelWarn:
		line = "::notice"
	case record.Level < slog.LevelError:
		line = "::warning"
	}
	if w.linkSource && record.PC != 0 {
		frames := runtime.CallersFrames([]uintptr{record.PC})
		frame, _ := frames.Next()
		line += " file=" + frame.File + ",line=" + strconv.Itoa(frame.Line)
	}
	line += "::"

	root := w
	for root.parent != nil {
		root = root.parent
	}
	root.mux.Lock()
	defer root.mux.Unlock()
	if root.buf == nil {
		b := make([]byte, 0, 1024)
		root.buf = &b
	}
	*root.buf = (*root.buf)[:0]
	*root.buf = append(*root.buf, line...)
	err := w.handler.Handle(ctx, record)
	if err != nil {
		return err
	}
	*root.buf = append(*root.buf, '\n')
	_, err = w.writer.Write(*root.buf)
	return err
}

func (w *Wrapper) Enabled(ctx context.Context, level slog.Level) bool {
	return w.handler.Enabled(ctx, level)
}

func (w *Wrapper) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &Wrapper{
		handler:    w.handler.WithAttrs(attrs),
		writer:     w.writer,
		linkSource: w.linkSource,
		parent:     w,
	}
}

func (w *Wrapper) WithGroup(name string) slog.Handler {
	return &Wrapper{
		handler:    w.handler.WithGroup(name),
		writer:     w.writer,
		linkSource: w.linkSource,
		parent:     w,
	}
}

type escapeWriter struct {
	w    io.Writer
	pool sync.Pool
}

func (e *escapeWriter) Write(p []byte) (int, error) {
	n := 0
	buf, ok := e.pool.Get().(*[]byte)
	if !ok {
		b := make([]byte, 0, len(p))
		buf = &b
	}
	defer e.pool.Put(buf)
	*buf = (*buf)[:0]
	for len(p) > 0 {
		i := bytes.IndexAny(p, "\n\r%")
		if i < 0 {
			*buf = append(*buf, p...)
			break
		}
		*buf = append(*buf, p[:i]...)
		p = p[i:]
		n += i
		switch p[0] {
		case '\n':
			*buf = append(*buf, "%0A"...)
		case '\r':
			*buf = append(*buf, "%0D"...)
		case '%':
			*buf = append(*buf, "%25"...)
		}
		p = p[1:]
		n++
	}
	return n, nil
}