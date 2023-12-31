//go:build go1.21

package actionslog_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/willabides/actionslog"
)

func ExampleWrapper() {
	logger := slog.New(&actionslog.Wrapper{})
	logger = logger.With(slog.String("func", "Example"))

	logger.Info("hello", slog.String("object", "world"))
	logger.Warn("This is a stern warning")
	logger.Error("got an error", slog.Any("err", fmt.Errorf("omg")))
	logger.Debug("this is a debug message")
	logger.Info("this is a \n multiline \r\n message")
	logger.Info("multiline value", slog.String("value", "this is a\nmultiline\nvalue"))

	// Output:
	//
	// ::notice ::msg=hello func=Example object=world
	// ::warning ::msg="This is a stern warning" func=Example
	// ::error ::msg="got an error" func=Example err=omg
	// ::debug ::msg="this is a debug message" func=Example
	// ::notice ::msg="this is a \n multiline \r\n message" func=Example
	// ::notice ::msg="multiline value" func=Example value="this is a\nmultiline\nvalue"
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
	// ::notice ::msg="this is a debug message"
	// ::notice ::msg="this is an info message"
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
			requireStringContains(t, "::notice ::msg=hello i="+strconv.Itoa(i)+"\n", buf.String())
			requireStringContains(t, "::notice ::msg=hello sub=sub i="+strconv.Itoa(i)+"\n", buf.String())
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
		want := "::notice file=" + wantFile + ",line=" + strconv.Itoa(wantLine) + "::msg=hello\n"
		requireEqualString(t, want, buf.String())
	})

	t.Run("WithGroup", func(t *testing.T) {
		var buf bytes.Buffer
		logger := slog.New(&actionslog.Wrapper{Output: &buf})
		logger = logger.WithGroup("group1")
		logger = logger.With(slog.String("a", "b"))
		logger.Info("hello")
		requireEqualString(t, "::notice ::msg=hello group1.a=b\n", buf.String())
	})

	t.Run("WithGroup and attrs", func(t *testing.T) {
		var buf bytes.Buffer
		logger := slog.New(&actionslog.Wrapper{Output: &buf})
		logger = logger.WithGroup("group1")
		logger = logger.With(slog.String("a", "b"))
		logger = logger.With(slog.String("c", "d"))
		logger.Info("hello", slog.String("e", "f"))
		requireEqualString(t, "::notice ::msg=hello group1.a=b group1.c=d group1.e=f\n", buf.String())
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
		requireEqualString(t, "::notice ::msg=hello\n", buf.String())
	})

	t.Run("escapes message", func(t *testing.T) {
		t.Run("default handler", func(t *testing.T) {
			var buf bytes.Buffer
			logger := slog.New(&actionslog.Wrapper{Output: &buf})
			logger.Info("percentages\r\n50% 75% 100%")
			require.Equal(t, `::notice ::msg="percentages\r\n50%25 75%25 100%25"`+"\n", buf.String())
		})

		t.Run("rawMessageHandler", func(t *testing.T) {
			var buf bytes.Buffer
			logger := slog.New(&actionslog.Wrapper{
				Output: &buf,
				Handler: func(w io.Writer) slog.Handler {
					return &rawMsgHandler{w: w}
				},
			})
			logger.Info("percentages\r\n50% 75% 100%")
			require.Equal(t, "::notice ::percentages%0D%0A50%25 75%25 100%25\n", buf.String())
		})
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

		requireEqualString(t, `::debug ::msg=debug
::notice ::msg=info
::warning ::msg=warn
::error ::msg=error
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
		requireEqualString(t, `::notice ::msg=info
::warning ::msg=warn
::error ::msg=error
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
		requireEqualString(t, `::warning ::msg=warn
::error ::msg=error
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
		requireEqualString(t, `::error ::msg=error
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

type rawMsgHandler struct {
	w io.Writer
}

func (r *rawMsgHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (r *rawMsgHandler) Handle(_ context.Context, record slog.Record) error {
	_, err := r.w.Write([]byte(record.Message))
	return err
}

func (r *rawMsgHandler) WithAttrs([]slog.Attr) slog.Handler {
	return r
}

func (r *rawMsgHandler) WithGroup(string) slog.Handler {
	return r
}
