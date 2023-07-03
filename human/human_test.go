//go:build go1.21

package human_test

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/require"
	"github.com/willabides/actionslog/human"
)

type mar struct {
	val string
}

func (m mar) MarshalYAML() ([]byte, error) {
	return strconv.AppendQuote(nil, m.val), nil
}

func TestMar(t *testing.T) {
	b, err := yaml.Marshal(mar{val: "foo\nbar"})
	require.NoError(t, err)
	fmt.Println(string(b))
	b, err = yaml.Marshal("hello\nworld")
	require.NoError(t, err)
	fmt.Println(string(b))
}

func ExampleHandler() {
	logger := slog.New(human.New(&human.Options{
		Output: os.Stdout,
	}))
	logger = logger.With(slog.String("func", "Example"))
	logger.Info("hello", slog.String("object", "world"))
	logger.Warn("This is a stern warning")
	logger.Error("got an error", slog.Any("err", fmt.Errorf("omg")))
	logger.Debug("this is a debug message")
	logger.Info("this is a\nmultiline\nmessage")
	logger.Info("multiline value", slog.String("value", "this is a\nmultiline\nvalue"))

	// Output:
	//
	// hello
	//   func: Example
	//   object: world
	// This is a stern warning
	//   func: Example
	// got an error
	//   func: Example
	//   err: omg
	// this is a
	// multiline
	// message
	//   func: Example
	// multiline value
	//   func: Example
	//   value: |-
	//     this is a
	//     multiline
	//     value
	//
}

func TestHuman_simple(t *testing.T) {
	var buf bytes.Buffer
	handler := human.New(&human.Options{
		Output: &buf,
	})
	logger := slog.New(handler)
	logger.Info("hello", slog.Any("object", map[string]string{
		"a": "a",
		"b": "",
	}))
	fmt.Println(buf.String())
}

func TestHuman(t *testing.T) {
	var buf bytes.Buffer
	handler := human.New(&human.Options{
		Output: &buf,
	})
	logger := slog.New(handler)
	logger = logger.With(slog.String("foo", "bar"))
	logger = logger.WithGroup("g1")
	logger = logger.With(slog.Any("thing", map[string]string{
		"a": "a",
		"b": "",
	}))
	logger = logger.WithGroup("g2")
	logger = logger.WithGroup("g3")
	logger = logger.With(slog.String("a", "b"), slog.Group("omg", "a", 1))
	logger = logger.WithGroup("g4")
	logger = logger.With(
		slog.String("", "empty key"),
		slog.String("", ""),
		slog.String("no value", ""),
	)
	logger = logger.WithGroup("g5")
	logger.Info("hi",
		slog.String("a", "b"),
		slog.String("", "empty key"),
		slog.Any("", nil),
	)
	want := `
hi
  foo: bar
  g1:
    thing:
      a: a
      b: ""
    g2:
      g3:
        a: b
        omg:
          a: 1
        g4:
          "": empty key
          "": ""
          no value: ""
          g5:
            a: b
            "": empty key
`

	want = strings.TrimSpace(want)
	got := strings.TrimSpace(buf.String())
	require.Equal(t, want, got)
}
