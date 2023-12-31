// Code generated by script/generate. DO NOT EDIT.

//go:build !go1.21

package human

import (
	"bytes"
	"encoding/json"
	"fmt"
	"golang.org/x/exp/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/goccy/go-yaml"
)

var (
	indentPrefixes     [255]string
	indentPrefixesOnce sync.Once
)

func getIndentPrefix(indents int) string {
	indentPrefixesOnce.Do(func() {
		indentPrefixes[0] = ""
		for i := 1; i < len(indentPrefixes); i++ {
			indentPrefixes[i] = indentPrefixes[i-1] + "  "
		}
	})
	if indents < len(indentPrefixes) {
		return indentPrefixes[indents]
	}
	return strings.Repeat("  ", indents)
}

func appendYamlAttr(resources *resourcePool, dst []byte, attr slog.Attr) []byte {
	kind := attr.Value.Kind()
	if kind == slog.KindAny || kind == slog.KindLogValuer {
		attr.Value = attr.Value.Resolve()
		kind = attr.Value.Kind()
	}
	if kind == slog.KindAny {
		return appendYamlAnyAttr(resources, dst, attr)
	}
	dst = appendYamlKey(dst, attr.Key)
	return appendYamlValue(resources, dst, attr.Value)
}

func appendYamlKey(dst []byte, key string) []byte {
	key = strings.TrimSpace(key)
	if key == "" {
		return append(dst, `"": `...)
	}
	if strings.ContainsAny(key, ":\t\r\n") {
		dst = strconv.AppendQuote(dst, key)
	} else {
		dst = append(dst, key...)
	}
	return append(dst, ": "...)
}

func appendYamlValString(resources *resourcePool, dst []byte, s string) []byte {
	s = strings.TrimSpace(s)
	if strings.ContainsAny(s, "\n\r\t:") || s == "" {
		bufBytes := resources.borrowBytes()
		buf := bytes.NewBuffer(*bufBytes)
		err := yaml.NewEncoder(buf, yaml.Indent(2)).Encode(s)
		if err != nil {
			buf.Reset()
			buf.WriteString(strconv.Quote("!BAD STRING " + s))
		}
		dst = append(dst, buf.Bytes()...)
		*bufBytes = buf.Bytes()
	} else {
		dst = append(dst, s...)
	}
	if len(dst) != 0 || dst[len(dst)-1] != '\n' {
		dst = append(dst, '\n')
	}
	return dst
}

func appendYamlValue(resources *resourcePool, dst []byte, val slog.Value) []byte {
	switch val.Kind() {
	case slog.KindInt64:
		dst = strconv.AppendInt(dst, val.Int64(), 10)
	case slog.KindUint64:
		dst = strconv.AppendUint(dst, val.Uint64(), 10)
	case slog.KindBool:
		dst = strconv.AppendBool(dst, val.Bool())
	case slog.KindDuration:
		dst = appendDuration(dst, val.Duration())
	case slog.KindFloat64:
		dst = strconv.AppendFloat(dst, val.Float64(), 'f', -1, 64)
	case slog.KindTime:
		dst = appendYAMLTime(dst, val.Time())
	case slog.KindString:
		dst = appendYamlValString(resources, dst, val.String())
	case slog.KindGroup:
		// remove trailing space after ":" if any
		if len(dst) > 1 && dst[len(dst)-1] == ' ' && dst[len(dst)-2] == ':' {
			dst = dst[:len(dst)-1]
		}
		dst = append(dst, "\n  "...)
		b := resources.borrowBytes()
		for _, a := range val.Group() {
			*b = appendYamlAttr(
				resources,
				(*b)[:0],
				a,
			)
			for i := range *b {
				dst = append(dst, (*b)[i])
				if (*b)[i] == '\n' {
					dst = append(dst, ' ', ' ')
				}
			}
		}
		resources.returnBytes(b)
	default:
		dst = appendYamlValString(resources, dst, "!ERROR unknown kind: "+val.String())
	}
	dst = bytes.TrimRight(dst, " \t\r\n")
	if len(dst) != 0 || dst[len(dst)-1] != '\n' {
		dst = append(dst, '\n')
	}
	return dst
}

// appendYamlAnyAttr appends both key and value. We need to do it this way because we don't know
// what the yaml formatting will be ahead of time.
func appendYamlAnyAttr(resources *resourcePool, dst []byte, attr slog.Attr) []byte {
	val := attr.Value.Any()
	// use errors' error message only if it doesn't implement one of these marshalers
	switch v := val.(type) {
	case yaml.BytesMarshaler,
		yaml.InterfaceMarshaler,
		yaml.BytesMarshalerContext,
		yaml.InterfaceMarshalerContext,
		json.Marshaler:
	case error:
		dst = appendYamlKey(dst, attr.Key)
		dst = appendYamlValString(resources, dst, v.Error())
		dst = bytes.TrimRight(dst, " \t\r\n")
		if len(dst) != 0 || dst[len(dst)-1] != '\n' {
			dst = append(dst, '\n')
		}
		return dst
	}

	bufBytes := resources.borrowBytes()
	buf := bytes.NewBuffer(*bufBytes)
	mp := resources.borrowMap()
	defer resources.returnMap(mp)
	mp[attr.Key] = val
	err := yaml.NewEncoder(buf, yaml.UseJSONMarshaler(), yaml.Indent(2)).Encode(mp)
	if err != nil {
		dst = appendYamlKey(dst, attr.Key)
		return appendYamlValString(resources, dst, fmt.Sprintf("!ERROR encoding: %s", err.Error()))
	}
	dst = append(dst, buf.Bytes()...)
	dst = bytes.TrimRight(dst, " \t\r\n")
	if len(dst) != 0 || dst[len(dst)-1] != '\n' {
		dst = append(dst, '\n')
	}
	return dst
}

// Adapted from log/slog.appendJSONTime in go stdlib.
func appendYAMLTime(buf []byte, t time.Time) []byte {
	const rfc3339Millis = "2006-01-02T15:04:05.000Z07:00"
	if y := t.Year(); y < 0 || y >= 10000 {
		return append(buf, "!BAD TIME tim.Time year outside of range [0,9999]"...)
	}
	buf = append(buf, '"')
	buf = t.AppendFormat(buf, rfc3339Millis)
	buf = append(buf, '"')
	return buf
}
