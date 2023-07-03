//go:build go1.21

package human

import (
	"bytes"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml"
)

const rfc3339Millis = "2006-01-02T15:04:05.000Z07:00"

func appendAttrYaml(dst []byte, attr slog.Attr) []byte {
	kind := attr.Value.Kind()
	if kind == slog.KindAny || kind == slog.KindLogValuer {
		attr.Value = attr.Value.Resolve()
		kind = attr.Value.Kind()
	}
	if kind == slog.KindAny {
		b, err := yaml.MarshalWithOptions(
			attrsMarshaler{attrs: []slog.Attr{attr}},
			yaml.UseJSONMarshaler(),
			yaml.Indent(2),
		)
		if err != nil {
			b = []byte(fmt.Sprintf("!ERROR encoding: %q", err.Error()))
		}
		dst = append(dst, b...)
		if len(dst) == 0 || dst[len(dst)-1] != '\n' {
			dst = append(dst, '\n')
		}
		return dst
	}
	key := strings.TrimSpace(attr.Key)
	if strings.ContainsAny(key, ":\t\r\n") || key == "" {
		b, err := yaml.Marshal(key)
		if err != nil {
			b = []byte(strconv.Quote("!BAD KEY " + key))
		}
		key = strings.TrimSpace(string(b))
	}
	val := attr.Value
	dst = append(dst, key...)
	dst = append(dst, ':')
	switch kind {
	case slog.KindGroup:
		dst = append(dst, "\n  "...)
		b := borrowBytes()
		defer returnBytes(b)
		for _, a := range val.Group() {
			*b = appendAttrYaml((*b)[:0], a)
			*b = bytes.ReplaceAll(*b, []byte("\n"), []byte("\n  "))
			dst = append(dst, *b...)
		}
	case slog.KindBool,
		slog.KindFloat64,
		slog.KindInt64,
		slog.KindUint64,
		slog.KindDuration:
		dst = append(dst, ' ')
		dst = append(dst, val.String()...)
	case slog.KindTime:
		dst = append(dst, ' ')
		dst = append(dst, val.Time().Format(rfc3339Millis)...)
	case slog.KindString:
		dst = append(dst, ' ')
		s := strings.TrimSpace(val.String())
		if strings.ContainsAny(s, "\n\r\t:") || s == "" {
			b, err := yaml.Marshal(s)
			if err != nil {
				b = []byte(strconv.Quote("!BAD STRING " + s))
			}
			b = bytes.TrimSpace(b)
			s = string(b)
		}
		dst = append(dst, s...)
	}
	dst = bytes.TrimRight(dst, " \t\r\n")
	if len(dst) == 0 || dst[len(dst)-1] != '\n' {
		dst = append(dst, '\n')
	}
	return dst
}

type valMarshaler struct {
	val slog.Value
}

func (m valMarshaler) MarshalYAML() (any, error) {
	switch m.val.Kind() {
	case slog.KindString:
		return m.val.String(), nil
	case slog.KindInt64:
		return m.val.Int64(), nil
	case slog.KindUint64:
		return m.val.Uint64(), nil
	case slog.KindFloat64:
		return m.val.Float64(), nil
	case slog.KindBool:
		return m.val.Bool(), nil
	case slog.KindDuration:
		return m.val.Duration().String(), nil
	case slog.KindTime:
		return m.val.Time().Format(rfc3339Millis), nil
	case slog.KindGroup:
		return attrsMarshaler{attrs: m.val.Group()}, nil
	case slog.KindAny:
		return anyMarshaler{any: m.val.Any()}, nil
	default:
		panic("bad kind: " + m.val.Kind().String())
	}
}

type attrsMarshaler struct {
	attrs []slog.Attr
}

func (m attrsMarshaler) MarshalYAML() (any, error) {
	attrs := m.attrs
	if len(attrs) == 0 {
		return nil, nil
	}
	mp := make(yaml.MapSlice, 0, len(attrs))
	for _, attr := range attrs {
		mp = append(mp, yaml.MapItem{
			Key:   attr.Key,
			Value: valMarshaler{attr.Value},
		})
	}
	return mp, nil
}

type anyMarshaler struct {
	any any
}

func (m anyMarshaler) MarshalYAML() (any, error) {
	switch v := m.any.(type) {
	case fmt.Stringer:
		return v.String(), nil
	case error:
		return v.Error(), nil
	}
	return m.any, nil
}
