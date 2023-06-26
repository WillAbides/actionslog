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
			requireStringContains(t, fmt.Sprintf("::notice ::hello i=%d\n", i), buf.String())
			requireStringContains(t, fmt.Sprintf("::notice ::hello sub=sub i=%d\n", i), buf.String())
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
		logger = logger.With(slog.String("a", "b"))
		logger = logger.WithGroup("group1")
		logger.Info("hello")
		requireEqualString(t, "::notice ::hello a=b\n", buf.String())
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
