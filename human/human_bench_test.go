//go:build go1.21

package human_test

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/willabides/actionslog/human"
)

var globalBuf bytes.Buffer

func BenchmarkHandler(b *testing.B) {
	ctx := context.Background()
	globalBuf = *bytes.NewBuffer(make([]byte, 0, 1024))

	b.Run("human", func(b *testing.B) {
		handler := human.New(&human.Options{
			Output: &globalBuf,
		})
		benchmarkHandler(ctx, b, handler)
	})
}

func benchmarkHandler(ctx context.Context, b *testing.B, handler slog.Handler) {
	b.Helper()
	record := slog.NewRecord(time.Time{}, 0, "text", 0)
	record.AddAttrs(slog.String("foo", "bar"))
	var err error
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		globalBuf.Reset()
		err = handler.Handle(ctx, record)
		if err != nil {
			break
		}
	}
	require.NoError(b, err)
}
