//go:build go1.21

package human

import (
	"context"
	"io"
	"log/slog"
	"os"
	"runtime"
	"strconv"
	"sync"
)

// Handler is a slog.Handler that writes human-readable log entries.
// Because it is for human consumption, changes to the format are not
// considered breaking changes. Entries may be multi-line.
// The current format looks like this:
//
//	<message>
//	  <attributes as yaml>
//
// No escaping is done on the message. Attributes are in YAML format with the top level
// indented to make it visually distinct from the message.
type Handler struct {
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

	depth         int
	pendingGroups []string // groups that have been added but not yet written
	yaml          []byte
	rootHandler   *Handler

	// Below here is only accessed on rootHandler
	resources resourcePool
	mu        sync.Mutex
}

func (h *Handler) root() *Handler {
	if h.rootHandler == nil {
		return h
	}
	return h.rootHandler
}

func (h *Handler) Enabled(_ context.Context, level slog.Level) bool {
	if h.Level != nil {
		return level >= h.Level.Level()
	}
	return level >= slog.LevelInfo
}

func (h *Handler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	return &Handler{
		Output:        h.Output,
		Level:         h.Level,
		ExcludeTime:   h.ExcludeTime,
		ExcludeLevel:  h.ExcludeLevel,
		AddSource:     h.AddSource,
		rootHandler:   h.root(),
		depth:         h.depth + 1,
		pendingGroups: append(h.pendingGroups, name),
		yaml:          h.yaml,
	}
}

func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	root := h.root()
	resources := &root.resources
	attrs = resolveAttrs(resources, attrs)
	if len(attrs) == 0 {
		return h
	}
	return &Handler{
		Output:       h.Output,
		Level:        h.Level,
		ExcludeTime:  h.ExcludeTime,
		ExcludeLevel: h.ExcludeLevel,
		AddSource:    h.AddSource,
		rootHandler:  root,
		depth:        h.depth,
		yaml:         h.appendYaml(h.yaml, attrs),
	}
}

// WithOutput returns a new Handler that writes to output.
// This is primarily meant for use with [github.com/willabides/actionslog.Wrapper]
func (h *Handler) WithOutput(output io.Writer) slog.Handler {
	return &Handler{
		Output:        output,
		Level:         h.Level,
		ExcludeTime:   h.ExcludeTime,
		ExcludeLevel:  h.ExcludeLevel,
		AddSource:     h.AddSource,
		depth:         h.depth,
		pendingGroups: append([]string{}, h.pendingGroups...),
		yaml:          append([]byte{}, h.yaml...),
		rootHandler:   nil, // new Output means this is the root handler now
	}
}

func (h *Handler) Handle(_ context.Context, record slog.Record) error {
	root := h.root()
	pool := &root.resources
	entry := pool.borrowBytes()
	*entry = append(*entry, record.Message...)
	*entry = append(*entry, '\n')
	if !h.ExcludeTime && !record.Time.IsZero() {
		*entry = append(*entry, "  "+slog.TimeKey+": "...)
		*entry = appendYAMLTime(*entry, record.Time)
		*entry = append(*entry, '\n')
	}
	if !h.ExcludeLevel {
		*entry = append(*entry, "  "+slog.LevelKey+": "+record.Level.String()+"\n"...)
	}
	if h.AddSource && record.PC != 0 {
		*entry = h.appendSource(record, *entry)
	}
	*entry = append(*entry, h.yaml...)
	attrs := pool.borrowAttrs()
	record.Attrs(func(attr slog.Attr) bool {
		*attrs = append(*attrs, attr)
		return true
	})
	*attrs = resolveAttrs(pool, *attrs)

	if len(*attrs) > 0 {
		*entry = h.appendYaml(*entry, *attrs)
	}
	root.mu.Lock()
	output := h.Output
	if output == nil {
		output = os.Stderr
	}
	_, err := output.Write(*entry)
	root.mu.Unlock()
	pool.returnBytes(entry)
	pool.returnAttrs(attrs)
	return err
}

func (h *Handler) appendSource(record slog.Record, dst []byte) []byte {
	ptrs := h.resources.borrowPtrs()
	*ptrs = append(*ptrs, record.PC)
	frame, _ := runtime.CallersFrames(*ptrs).Next()
	if frame.Function == "" && frame.File == "" {
		return dst
	}
	dst = append(dst, "  "+slog.SourceKey+":\n"...)
	if frame.Function != "" {
		dst = append(dst, "    function: "...)
		dst = append(dst, frame.Function...)
		dst = append(dst, '\n')
	}
	if frame.File != "" {
		dst = append(dst, "    file: "...)
		dst = append(dst, frame.File...)
		dst = append(dst, '\n')
	}
	if frame.Line != 0 {
		dst = append(dst, "    line: "...)
		dst = strconv.AppendInt(dst, int64(frame.Line), 10)
		dst = append(dst, '\n')
	}
	h.resources.returnPtrs(ptrs)
	return dst
}

func (h *Handler) appendYaml(dst []byte, attrs []slog.Attr) []byte {
	resources := &h.root().resources
	attrs = resolveAttrs(resources, attrs)
	if len(attrs) == 0 {
		return dst
	}
	indents := 1 + h.depth - len(h.pendingGroups)
	for i := 0; i < len(h.pendingGroups); i++ {
		prefix := getIndentPrefix(indents)
		dst = append(dst, prefix+h.pendingGroups[i]+":\n"...)
		indents++
	}
	prefix := getIndentPrefix(indents)
	buf := resources.borrowBytes()
	for _, attr := range attrs {
		*buf = appendYamlAttr(resources, *buf, attr)
		for _, b := range *buf {
			if len(dst) == 0 || dst[len(dst)-1] == '\n' {
				dst = append(dst, prefix...)
			}
			dst = append(dst, b)
		}
		*buf = (*buf)[:0]
	}
	resources.returnBytes(buf)
	return dst
}

// resolveAttrs resolves members of attrs.
// Resolving entails:
//   - Calling Resolve() on any LogValuer or Any values
//   - Inlining groups with empty keys
//   - Omitting zero-value Attrs
func resolveAttrs(resources *resourcePool, attrs []slog.Attr) []slog.Attr {
	resolved := resources.borrowAttrs()
	for _, attr := range attrs {
		kind := attr.Value.Kind()
		if kind == slog.KindLogValuer || kind == slog.KindAny {
			attr.Value = attr.Value.Resolve()
			kind = attr.Value.Kind()
		}
		// inline groups with empty keys
		if kind == slog.KindGroup && attr.Key == "" {
			*resolved = append(*resolved, resolveAttrs(resources, attr.Value.Group())...)
		}
		// elide empty attrs
		if attr.Equal(slog.Attr{}) {
			continue
		}
		*resolved = append(*resolved, attr)
	}
	attrs = append(attrs[:0], *resolved...)
	resources.returnAttrs(resolved)
	return attrs
}

type resourcePool struct {
	mapPool   sync.Pool
	bytesPool sync.Pool
	attrsPool sync.Pool
	ptrPool   sync.Pool
}

func (p *resourcePool) borrowMap() map[string]any {
	v := p.mapPool.Get()
	if v == nil {
		return map[string]any{}
	}
	return v.(map[string]any)
}

func (p *resourcePool) returnMap(v map[string]any) {
	for k := range v {
		delete(v, k)
	}
	p.mapPool.Put(v)
}

func (p *resourcePool) borrowBytes() *[]byte {
	v := p.bytesPool.Get()
	if v == nil {
		b := make([]byte, 0, 1024)
		return &b
	}
	return v.(*[]byte)
}

func (p *resourcePool) returnBytes(v *[]byte) {
	*v = (*v)[:0]
	p.bytesPool.Put(v)
}

func (p *resourcePool) borrowAttrs() *[]slog.Attr {
	v := p.attrsPool.Get()
	if v == nil {
		attrs := make([]slog.Attr, 0, 16)
		return &attrs
	}
	return v.(*[]slog.Attr)
}

func (p *resourcePool) returnAttrs(v *[]slog.Attr) {
	*v = (*v)[:0]
	p.attrsPool.Put(v)
}

func (p *resourcePool) borrowPtrs() *[]uintptr {
	v := p.ptrPool.Get()
	if v == nil {
		ptrs := make([]uintptr, 0, 16)
		return &ptrs
	}
	return v.(*[]uintptr)
}

func (p *resourcePool) returnPtrs(v *[]uintptr) {
	*v = (*v)[:0]
	p.ptrPool.Put(v)
}
