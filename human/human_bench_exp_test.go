// Code generated by script/generate. DO NOT EDIT.

//go:build !go1.21

package human_test

import (
	"context"
	"errors"
	"golang.org/x/exp/slog"
	"io"
	"runtime"
	"testing"
	"time"

	"github.com/willabides/actionslog/human"
)

var (
	testMessage  = "Test logging, but use a somewhat realistic message length."
	testDuration = 23 * time.Second
	testString   = "7e3b3b2aaeff56a7108fe11e154200dd/7819479873059528190"
	testInt      = 32768
	testTime     = time.Now()
	testError    = errors.New("fail")
)

func BenchmarkHandler(b *testing.B) {
	var pcs [1]uintptr
	runtime.Callers(0, pcs[:])

	b.Run("simple with source", func(b *testing.B) {
		record := slog.NewRecord(time.Time{}, slog.LevelInfo, testMessage, pcs[0])
		runBenchmarks(b, true, record, nil)
	})

	b.Run("simple", func(b *testing.B) {
		record := slog.NewRecord(time.Time{}, slog.LevelInfo, testMessage, 0)
		runBenchmarks(b, false, record, nil)
	})

	attrs := []slog.Attr{
		slog.String("string", testString),
		slog.Group(
			"group",
			slog.String("string", testString),
			slog.Any("any string", testString),
		),
		slog.Int("int", testInt),
		slog.Duration("duration", testDuration),
		slog.Time("time", testTime),
		slog.Any("error", testError),
		slog.Bool("bool", true),
	}

	b.Run("attrs", func(b *testing.B) {
		record := slog.NewRecord(time.Now(), 0, testMessage, 0)
		record.AddAttrs(append([]slog.Attr{}, attrs...)...)
		runBenchmarks(b, false, record, nil)
	})

	b.Run("groups", func(b *testing.B) {
		record := slog.NewRecord(time.Now(), 0, testMessage, 0)
		record.AddAttrs(append([]slog.Attr{}, attrs...)...)
		runBenchmarks(b, false, record, func(handler slog.Handler) slog.Handler {
			return handler.WithGroup(
				"group1",
			).WithGroup(
				"group2",
			).WithAttrs(
				append([]slog.Attr{}, attrs...),
			)
		})
	})

	b.Run("map", func(b *testing.B) {
		record := slog.NewRecord(time.Now(), 0, testMessage, 0)
		record.AddAttrs(slog.Any("map", map[string]any{
			"string": testString,
			"int":    testInt,
			"slice":  []string{"foo", "bar", "baz"},
		}))
		runBenchmarks(b, false, record, nil)
	})
}

func runBenchmarks(b *testing.B, addSource bool, record slog.Record, prepHandler func(slog.Handler) slog.Handler) {
	b.Helper()
	ctx := context.Background()

	bench := func(name string, handler slog.Handler) {
		b.Helper()
		if prepHandler != nil {
			handler = prepHandler(handler)
		}
		var err error
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					err = handler.Handle(ctx, record)
					if err != nil {
						panic(err)
					}
				}
			})
		})
	}

	bench("slog.TextHandler", slog.NewTextHandler(
		io.Discard,
		&slog.HandlerOptions{
			AddSource: addSource,
		},
	))

	bench("slog.JSONHandler", slog.NewJSONHandler(
		io.Discard,
		&slog.HandlerOptions{
			AddSource: addSource,
		},
	))

	bench("human.Handler", &human.Handler{
		Output:    io.Discard,
		AddSource: addSource,
	})
}
