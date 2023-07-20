//go:build go1.21

package human_test

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/willabides/actionslog/human"
)

func ExampleHandler() {
	logger := slog.New(&human.Handler{
		Output:      os.Stdout,
		ExcludeTime: true,
	})
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
	//   level: INFO
	//   func: Example
	//   object: world
	// This is a stern warning
	//   level: WARN
	//   func: Example
	// got an error
	//   level: ERROR
	//   func: Example
	//   err: omg
	// this is a
	// multiline
	// message
	//   level: INFO
	//   func: Example
	// multiline value
	//   level: INFO
	//   func: Example
	//   value: |-
	//     this is a
	//     multiline
	//     value
	//
}

func TestHandler(t *testing.T) {
	t.Run("concurrency", func(t *testing.T) {
		var buf bytes.Buffer
		logger := slog.New(&human.Handler{
			Output:      &buf,
			ExcludeTime: true,
		})
		count := 1000
		sub := logger.With(slog.String("sub", "sub"))
		var wg sync.WaitGroup
		for i := 0; i < count; i++ {
			wg.Add(1)
			go func(i int) {
				logger.Info("hello", slog.Int("i", i))
				sub.Info("hello", slog.Int("i", i))
				wg.Done()
			}(i)
		}
		wg.Wait()
		assert.Equal(t, count*2, strings.Count(buf.String(), "hello"))
	})

	t.Run("basic", func(t *testing.T) {
		var buf bytes.Buffer
		handler := &human.Handler{
			Output:      &buf,
			ExcludeTime: true,
		}
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
			slog.Duration("duration", 66*time.Second),
			slog.Uint64("uint", 1),
			slog.Bool("bool", false),
			slog.Float64("float", 1.5),
			slog.Time("time", time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
			slog.Any("error", fmt.Errorf("omg\nI think I broke a nail")),
		)
		want := `
hi
  level: INFO
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
            duration: 1m6s
            uint: 1
            bool: false
            float: 1.5
            time: "2021-01-01T00:00:00.000Z"
            error: |-
              omg
              I think I broke a nail
`

		want = strings.TrimSpace(want)
		got := strings.TrimSpace(buf.String())
		require.Equal(t, want, got)
	})
}
