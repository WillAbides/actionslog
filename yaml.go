//go:build go1.21

package actionslog

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"gopkg.in/yaml.v3"
)

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
		return m.val.Time().String(), nil
	case slog.KindGroup:
		return attrsMarshaler{attrs: m.val.Group()}, nil
	case slog.KindAny:
		return anyMarshaler{any: m.val.Any()}, nil
	default:
		panic(fmt.Sprintf("bad kind: %s", m.val.Kind()))
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
	node := yaml.Node{
		Kind: yaml.MappingNode,
	}
	for _, attr := range attrs {
		keyNode := yaml.Node{
			Kind:  yaml.ScalarNode,
			Value: attr.Key,
		}
		valNode := &yaml.Node{}
		err := valNode.Encode(valMarshaler{attr.Value})
		if err != nil {
			return nil, err
		}
		node.Content = append(node.Content, &keyNode, valNode)
	}
	return &node, nil
}

type anyMarshaler struct {
	any any
}

func (m anyMarshaler) MarshalYAML() (any, error) {
	a := m.any
	_, ok := a.(yaml.Marshaler)
	if ok {
		return a, nil
	}
	_, ok = a.(json.Marshaler)
	if ok {
		jb, err := json.Marshal(a)
		if err != nil {
			return nil, err
		}
		var val any
		err = json.Unmarshal(jb, &val)
		if err != nil {
			return nil, err
		}
		return val, nil
	}
	_, ok = a.(error)
	if ok {
		return a.(error).Error(), nil
	}
	_, ok = a.(fmt.Stringer)
	if ok {
		return a.(fmt.Stringer).String(), nil
	}
	// try encoding it as yaml before falling back to json
	node := &yaml.Node{}
	err := node.Encode(a)
	if err == nil {
		return node, nil
	}
	jb, err := json.Marshal(a)
	if err != nil {
		return nil, err
	}
	var val any
	err = json.Unmarshal(jb, &val)
	if err != nil {
		return nil, err
	}
	return val, nil
}
