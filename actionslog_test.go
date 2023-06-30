//go:build go1.21

package actionslog_test

import (
	"bytes"
	"fmt"
	"log/slog"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/willabides/actionslog"
)

func ExampleWrapper() {
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
	// ::notice ::hello func=Example object=world%0A
	// ::warning ::This is a stern warning func=Example%0A
	// ::error ::got an error func=Example err=omg%0A
	// ::debug ::this is a debug message func=Example%0A
	// ::notice ::this is a %0A multiline %0A message func=Example%0A
	// ::notice ::multiline value func=Example value="this is a\nmultiline\nvalue"%0A
}

func ExampleWrapper_writeDebugToNotice() {
	// This gets around GitHub Actions' behavior of hiding debug messages unless you specify
	// "enable debug logging" by outputting debug messages as notice messages.

	logger := slog.New(&actionslog.Wrapper{
		ActionsLogger: func(level slog.Level) actionslog.ActionsLog {
			defaultLog := actionslog.DefaultActionsLog(level)
			if defaultLog == actionslog.LogDebug {
				return actionslog.LogNotice
			}
			return defaultLog
		},
	})
	logger.Debug("this is a debug message")
	logger.Info("this is an info message")

	// Output:
	//
	// ::notice ::this is a debug message
	// ::notice ::this is an info message
}

func TestWrapper(t *testing.T) {
	t.Run("concurrency", func(t *testing.T) {
		var buf bytes.Buffer
		logger := slog.New(&actionslog.Wrapper{Output: &buf})
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
			requireStringContains(t, "::notice ::hello i="+strconv.Itoa(i)+"%0A\n", buf.String())
			requireStringContains(t, "::notice ::hello sub=sub i="+strconv.Itoa(i)+"%0A\n", buf.String())
		}
	})

	t.Run("AddSource", func(t *testing.T) {
		var buf bytes.Buffer
		logger := slog.New(&actionslog.Wrapper{
			Output:    &buf,
			AddSource: true,
		})
		_, wantFile, wantLine, _ := runtime.Caller(0)
		logger.Info("hello")
		wantLine++
		want := "::notice file=" + wantFile + ",line=" + strconv.Itoa(wantLine) + "::hello\n"
		requireEqualString(t, want, buf.String())
	})

	t.Run("WithGroup", func(t *testing.T) {
		var buf bytes.Buffer
		logger := slog.New(&actionslog.Wrapper{Output: &buf})
		logger = logger.WithGroup("group1")
		logger = logger.With(slog.String("a", "b"))
		logger.Info("hello")
		requireEqualString(t, "::notice ::hello group1.a=b%0A\n", buf.String())
	})

	t.Run("WithGroup and attrs", func(t *testing.T) {
		var buf bytes.Buffer
		logger := slog.New(&actionslog.Wrapper{Output: &buf})
		logger = logger.WithGroup("group1")
		logger = logger.With(slog.String("a", "b"))
		logger = logger.With(slog.String("c", "d"))
		logger.Info("hello", slog.String("e", "f"))
		requireEqualString(t, "::notice ::hello group1.a=b group1.c=d group1.e=f%0A\n", buf.String())
	})

	t.Run("Debug to notice", func(t *testing.T) {
		var buf bytes.Buffer
		logger := slog.New(&actionslog.Wrapper{
			Output: &buf,
			Level:  slog.LevelDebug,
			ActionsLogger: func(level slog.Level) actionslog.ActionsLog {
				l := actionslog.DefaultActionsLog(level)
				if l == actionslog.LogDebug {
					return actionslog.LogNotice
				}
				return l
			},
		})
		logger.Debug("hello")
		requireEqualString(t, "::notice ::hello\n", buf.String())
	})

	t.Run("escapes message", func(t *testing.T) {
		var buf bytes.Buffer
		logger := slog.New(&actionslog.Wrapper{Output: &buf})
		logger.Info("percentages\r\n50% 75% 100%")
		requireEqualString(t, "::notice ::percentages%0D%0A50%25 75%25 100%25\n", buf.String())
	})

	t.Run("debug", func(t *testing.T) {
		var buf bytes.Buffer
		logger := slog.New(&actionslog.Wrapper{
			Output: &buf,
			Level:  slog.LevelDebug,
		})
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
		logger := slog.New(&actionslog.Wrapper{
			Output: &buf,
			Level:  slog.LevelInfo,
		})
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
		logger := slog.New(&actionslog.Wrapper{
			Output: &buf,
			Level:  slog.LevelWarn,
		})
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
		logger := slog.New(&actionslog.Wrapper{
			Output: &buf,
			Level:  slog.LevelError,
		})
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
