//go:build go1.21

package human

import (
	"context"
	"io"
	"log/slog"
	"math"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

// Options are options for New.
type Options struct {
	// Output is the writer to write to. Defaults to os.Stderr.
	Output io.Writer

	// Level is the minimum level to log. Defaults to slog.LevelInfo.
	Level slog.Leveler

	// ExcludeTime, if true, will exclude the time from the output.
	ExcludeTime bool

	// ExcludeLevel, if true, will exclude the level from the output.
	ExcludeLevel bool

	// AddSource, if true, will add the source file and line number to the output.
	AddSource bool
}

// Handler is a slog.Handler that writes human-readable log entries.
// Because it is for human consumption, changes to the format are not
// considered breaking changes. Entries may be multi-line.
// The current format looks like this:
//
//		<message>
//	   <attributes>
//
// No escaping is done on the message. Attributes are in YAML format with the top level
// indented to make it visually distinct from the message.
type Handler struct {
	opts   Options
	depth  int
	yaml   []byte
	groups []string
}

func New(opts *Options) *Handler {
	if opts == nil {
		opts = &Options{}
	}
	return &Handler{
		opts:   *opts,
		groups: make([]string, 0, 10),
	}
}

// WithOutput returns a new slog.Handler with the given output writer and permissive level.
// This is primarily for use with actionslog.Wrapper.
func WithOutput(w io.Writer) slog.Handler {
	return New(
		&Options{
			Output: w,
			Level:  slog.Level(math.MinInt),
		},
	)
}

func (h *Handler) Enabled(_ context.Context, level slog.Level) bool {
	if h.opts.Level != nil {
		return level >= h.opts.Level.Level()
	}
	return level >= slog.LevelInfo
}

func (h *Handler) Handle(_ context.Context, record slog.Record) error {
	output := h.opts.Output
	if output == nil {
		output = os.Stderr
	}
	frontAttrs := borrowAttrs()
	if !h.opts.ExcludeTime && !record.Time.IsZero() {
		*frontAttrs = append(*frontAttrs, slog.Time("time", record.Time))
	}
	if !h.opts.ExcludeLevel {
		*frontAttrs = append(*frontAttrs, slog.String("level", record.Level.String()))
	}
	if h.opts.AddSource && record.PC != 0 {
		frames := runtime.CallersFrames([]uintptr{record.PC})
		frame, _ := frames.Next()
		*frontAttrs = append(*frontAttrs, slog.String(
			"source",
			frame.File+":"+strconv.Itoa(frame.Line),
		))
	}
	recAttrs := borrowAttrs()
	record.Attrs(func(attr slog.Attr) bool {
		*recAttrs = append(*recAttrs, attr)
		return true
	})
	*recAttrs = resolveAttrs(*recAttrs)
	line := borrowBytes()
	*line = append(*line, record.Message...)
	*line = append(*line, '\n')
	if len(*frontAttrs) > 0 {
		*line = appendYaml(*line, h.depth, h.groups, *frontAttrs)
	}
	if len(h.yaml) > 0 {
		*line = append(*line, h.yaml...)
	}
	if len(*recAttrs) > 0 {
		*line = appendYaml(*line, h.depth, h.groups, *recAttrs)
	}
	_, err := output.Write(*line)
	returnBytes(line)
	returnAttrs(recAttrs)
	returnAttrs(frontAttrs)
	return err
}

func appendYaml(
	b []byte,
	depth int,
	groups []string,
	attrs []slog.Attr,
) []byte {
	attrs = resolveAttrs(attrs)
	if len(attrs) == 0 {
		return b
	}
	indents := 1 + depth - len(groups)
	prefix := strings.Repeat("  ", indents)
	for i := len(groups) - 1; i >= 0; i-- {
		attrs = []slog.Attr{
			{
				Key:   groups[i],
				Value: slog.GroupValue(attrs...),
			},
		}
	}
	buf := borrowBytes()
	defer returnBytes(buf)
	for _, attr := range attrs {
		*buf = appendAttrYaml(*buf, attr)
	}
	iaBuf := borrowBytes()
	defer returnBytes(iaBuf)
	ia := indentAppender{
		prefix: prefix,
		buf:    iaBuf,
	}
	//nolint:errcheck // impossible error
	_, _ = ia.Write(*buf)
	return append(b, *ia.buf...)
}

func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	attrs = resolveAttrs(attrs)
	if len(attrs) == 0 {
		return h
	}
	return &Handler{
		opts:  h.opts,
		depth: h.depth,
		yaml:  appendYaml(h.yaml, h.depth, h.groups, attrs),
	}
}

func (h *Handler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	return &Handler{
		opts:   h.opts,
		depth:  h.depth + 1,
		yaml:   h.yaml,
		groups: append(h.groups, name),
	}
}

// indentAppender is like indentWriter, but it appends to a byte slice.
type indentAppender struct {
	buf    *[]byte
	prefix string
}

func (x *indentAppender) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}
	needsPrefix := len(*x.buf) == 0 || (*x.buf)[len(*x.buf)-1] == '\n'
	for _, b := range p {
		if needsPrefix {
			*x.buf = append(*x.buf, x.prefix...)
			needsPrefix = false
		}
		if b == '\n' {
			needsPrefix = true
		}
		*x.buf = append(*x.buf, b)
		n++
	}
	return n, nil
}

func resolveAttrs(attrs []slog.Attr) []slog.Attr {
	resolved := borrowAttrs()
	for _, attr := range attrs {
		kind := attr.Value.Kind()
		if kind == slog.KindLogValuer || kind == slog.KindAny {
			attr.Value = attr.Value.Resolve()
			kind = attr.Value.Kind()
		}
		// inline groups with empty keys
		if kind == slog.KindGroup && attr.Key == "" {
			*resolved = append(*resolved, resolveAttrs(attr.Value.Group())...)
		}
		// elide empty attrs
		if attr.Equal(slog.Attr{}) {
			continue
		}
		*resolved = append(*resolved, attr)
	}
	attrs = append(attrs[:0], *resolved...)
	returnAttrs(resolved)
	return attrs
}

var bytesPool sync.Pool

func borrowBytes() *[]byte {
	v := bytesPool.Get()
	if v == nil {
		b := make([]byte, 0, 1024)
		return &b
	}
	return v.(*[]byte)
}

func returnBytes(b *[]byte) {
	*b = (*b)[:0]
	bytesPool.Put(b)
}

var attrsPool sync.Pool

func borrowAttrs() *[]slog.Attr {
	v := attrsPool.Get()
	if v == nil {
		attrs := make([]slog.Attr, 0, 16)
		return &attrs
	}
	return v.(*[]slog.Attr)
}

func returnAttrs(attrs *[]slog.Attr) {
	*attrs = (*attrs)[:0]
	attrsPool.Put(attrs)
}
