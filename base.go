//go:build go1.21

package actionslog

import (
	"context"
	"log/slog"
)

type baseHandler struct {
	// Level reports the minimum record level that will be logged.
	// The handler discards records with lower levels.
	// If Level is nil, the handler assumes LevelInfo.
	// The handler calls Level.Level for each record processed;
	// to adjust the minimum level dynamically, use a LevelVar.
	Level slog.Leveler

	// HandleFunc is what is called on Handle.
	HandleFunc func(ctx context.Context, handlerAttrs []slog.Attr, record slog.Record) error

	attrs        []slog.Attr
	groupIndices []int
}

func (h *baseHandler) Enabled(_ context.Context, level slog.Level) bool {
	if h.Level == nil {
		return true
	}
	return level >= h.Level.Level()
}

func (h *baseHandler) Handle(ctx context.Context, record slog.Record) error {
	// noop if HandleFunc is nil
	if h.HandleFunc == nil {
		return nil
	}
	if record.NumAttrs() == 0 {
		return h.HandleFunc(ctx, h.attrs, record)
	}
	attrs := make([]slog.Attr, 0, record.NumAttrs()+len(h.attrs))
	attrs = append(attrs, h.attrs...)
	record.Attrs(func(attr slog.Attr) bool {
		attrs, _ = appendNestedAttr(h.groupIndices, attrs, attr)
		return true
	})
	r2 := slog.NewRecord(record.Time, record.Level, record.Message, record.PC)
	return h.HandleFunc(ctx, attrs, r2)
}

func (h *baseHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	h2 := baseHandler{
		Level:        h.Level,
		HandleFunc:   h.HandleFunc,
		attrs:        h.attrs,
		groupIndices: h.groupIndices,
	}
	for _, attr := range attrs {
		h2.attrs, _ = appendNestedAttr(h2.groupIndices, h2.attrs, attr)
	}
	return &h2
}

func (h *baseHandler) WithGroup(name string) slog.Handler {
	h2 := baseHandler{
		Level:        h.Level,
		HandleFunc:   h.HandleFunc,
		attrs:        h.attrs,
		groupIndices: h.groupIndices,
	}
	var idx int
	h2.attrs, idx = appendNestedAttr(h2.groupIndices, h2.attrs, slog.Group(name))
	h2.groupIndices = append(h2.groupIndices, idx)
	return &h2
}

func appendNestedAttr(groupIdx []int, attrs []slog.Attr, val slog.Attr) (_ []slog.Attr, newIdx int) {
	if len(groupIdx) == 0 {
		attrs = appendAttr(attrs, val)
		return attrs, len(attrs) - 1
	}
	groupAttrs, idx := appendNestedAttr(groupIdx[1:], attrs[groupIdx[0]].Value.Group(), val)
	attrs[groupIdx[0]].Value = slog.GroupValue(groupAttrs...)
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
