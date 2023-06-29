//go:build go1.21

package actionslog_test

import (
	"bytes"
	"fmt"
	"log/slog"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/willabides/actionslog"
)

func Example() {
	logger := slog.New(&actionslog.Wrapper{})
	logger = logger.With(slog.String("func", "Example"))

	logger.Info("hello", slog.String("object", "world"))
	logger.Warn("This is a stern warning")
	logger.Error("got an error", slog.Any("err", fmt.Errorf("omg")))
	logger.Debug("this is a debug message")
	logger.Info("this is a \n multiline \n message")
	logger.Info("multiline value", slog.String("value", "this is a\nmultiline\nvalue"))
	// Output:
	//
	// ::notice ::hello%0Afunc: Example%0Aobject: world%0A
	// ::warning ::This is a stern warning%0Afunc: Example%0A
	// ::error ::got an error%0Afunc: Example%0Aerr: omg%0A
	// ::debug ::this is a debug message%0Afunc: Example%0A
	// ::notice ::this is a %0A multiline %0A message%0Afunc: Example%0A
	// ::notice ::multiline value%0Afunc: Example%0Avalue: |-%0A  this is a%0A  multiline%0A  value%0A
}

func TestHandler(t *testing.T) {
	t.Run("concurrency", func(t *testing.T) {
		var buf bytes.Buffer
		logger := slog.New(actionslog.New(&buf, nil))
		sub := logger.With(slog.String("sub", "sub"))
		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(i int) {
				logger.Info("hello", slog.Int("i", i))
				sub.Info("hello", slog.Int("i", i))
				wg.Done()
			}(i)
		}
		wg.Wait()
		for i := 0; i < 100; i++ {
			requireStringContains(t, fmt.Sprintf("::notice ::hello%si: %d%s\n", "%0A", i, "%0A"), buf.String())
			requireStringContains(t, fmt.Sprintf("::notice ::hello%ssub: sub%si: %d%s\n", "%0A", "%0A", i, "%0A"), buf.String())
		}
	})

	t.Run("AddSource", func(t *testing.T) {
		var buf bytes.Buffer
		logger := slog.New(actionslog.New(&buf, &actionslog.Options{
			AddSource: true,
		}))
		_, wantFile, wantLine, _ := runtime.Caller(0)
		logger.Info("hello")
		wantLine++
		want := fmt.Sprintf("::notice file=%s,line=%d::hello\n", wantFile, wantLine)
		requireEqualString(t, want, buf.String())
	})

	t.Run("WithGroup", func(t *testing.T) {
		var buf bytes.Buffer
		logger := slog.New(actionslog.New(&buf, nil))
		logger = logger.WithGroup("group1")
		logger = logger.With(slog.String("a", "b"))
		logger.Info("hello")
		requireEqualString(t, "::notice ::hello%0Agroup1:%0A  a: b%0A\n", buf.String())
	})

	t.Run("WithGroup and attrs", func(t *testing.T) {
		var buf bytes.Buffer
		logger := slog.New(actionslog.New(&buf, nil))
		logger = logger.WithGroup("group1")
		logger = logger.With(slog.String("a", "b"))
		logger = logger.With(slog.String("c", "d"))
		logger.Info("hello", slog.String("e", "f"))
		requireEqualString(t, "::notice ::hello%0Agroup1:%0A  a: b%0A  c: d%0A  e: f%0A\n", buf.String())
	})

	t.Run("Debug to notice", func(t *testing.T) {
		var buf bytes.Buffer
		logger := slog.New(actionslog.New(&buf, &actionslog.Options{
			LevelLog: func(level slog.Level) actionslog.Log {
				l := actionslog.DefaultLevelLog(level)
				if l == actionslog.LogDebug {
					return actionslog.LogNotice
				}
				return l
			},
			Level: slog.LevelDebug,
		}))
		logger.Debug("hello")
		requireEqualString(t, "::notice ::hello\n", buf.String())
	})

	t.Run("escapes message", func(t *testing.T) {
		var buf bytes.Buffer
		logger := slog.New(actionslog.New(&buf, nil))
		logger.Info("percentages\r\n50% 75% 100%")
		requireEqualString(t, "::notice ::percentages%0D%0A50%25 75%25 100%25\n", buf.String())
	})

	t.Run("debug", func(t *testing.T) {
		var buf bytes.Buffer
		logger := slog.New(actionslog.New(&buf, &actionslog.Options{
			Level: slog.LevelDebug,
		}))
		logger.Debug("debug")
		logger.Info("info")
		logger.Warn("warn")
		logger.Error("error")

		requireEqualString(t, `::debug ::debug
::notice ::info
::warning ::warn
::error ::error
`, buf.String())
	})

	t.Run("info", func(t *testing.T) {
		var buf bytes.Buffer
		logger := slog.New(actionslog.New(&buf, &actionslog.Options{
			Level: slog.LevelInfo,
		}))
		logger.Debug("debug")
		logger.Info("info")
		logger.Warn("warn")
		logger.Error("error")
		requireEqualString(t, `::notice ::info
::warning ::warn
::error ::error
`, buf.String())
	})

	t.Run("warn", func(t *testing.T) {
		var buf bytes.Buffer
		logger := slog.New(actionslog.New(&buf, &actionslog.Options{
			Level: slog.LevelWarn,
		}))
		logger.Debug("debug")
		logger.Info("info")
		logger.Warn("warn")
		logger.Error("error")
		requireEqualString(t, `::warning ::warn
::error ::error
`, buf.String())
	})

	t.Run("error", func(t *testing.T) {
		var buf bytes.Buffer
		logger := slog.New(actionslog.New(&buf, &actionslog.Options{
			Level: slog.LevelError,
		}))
		logger.Debug("debug")
		logger.Info("info")
		logger.Warn("warn")
		logger.Error("error")
		requireEqualString(t, `::error ::error
`, buf.String())
	})
}

func requireEqualString(t *testing.T, want, got string) {
	t.Helper()
	if want != got {
		t.Fatalf(`Strings not equal:
want: %s
got:  %s`, want, got)
	}
}

func requireStringContains(t *testing.T, want, got string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Fatalf(`String does not contain:
want: %s
got:  %s`, want, got)
	}
}
