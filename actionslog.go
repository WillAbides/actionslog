//go:build go1.21

package actionslog

import (
	"io"
	"log/slog"
)

// Log is a log level in GitHub Actions.
type Log string

const (
	LogDebug  Log = "debug"
	LogNotice Log = "notice"
	LogWarn   Log = "warning"
	LogError  Log = "error"
)

// DefaultLevelLog is the default mapping from slog.Level to Log.
func DefaultLevelLog(level slog.Level) Log {
	switch {
	case level < slog.LevelInfo:
		return LogDebug
	case level < slog.LevelWarn:
		return LogNotice
	case level < slog.LevelError:
		return LogWarn
	default:
		return LogError
	}
}

type Options struct {
	// LevelLog maps a slog.Level to a Log. Defaults to DefaultLevelLog. To write debug to
	// LogNotice instead of LogDebug, use something like:
	//    func(level slog.Level) Log {
	//      l := actionslog.DefaultLevelLog.Log(level)
	//      if l == actionslog.LogDebug {
	//        return actionslog.LogNotice
	//      }
	//      return l
	//    }
	LevelLog func(slog.Level) Log

	// AddSource causes the handler to compute the source code position
	// of the log statement so that it can be linked from the GitHub Actions UI.
	AddSource bool

	// Level reports the minimum record level that will be logged.
	// The handler discards records with lower levels.
	// This does not affect the log level of the inner slog.Handler. For best results, either
	// set the level of the inner slog.Handler to slog.LevelDebug and use this Level to adjust
	// the minimum level or leave this nil and set the level of the inner slog.Handler. Do not
	// set both or you may confuse yourself.
	Level slog.Leveler

	// Handler is a function that returns the inner slog.Handler that will be used to format the parts
	// of the log that aren't specific to GitHub Actions. Default is DefaultHandler.
	Handler func(w io.Writer) slog.Handler
}

func New(w io.Writer, opts *Options) slog.Handler {
	if opts == nil {
		opts = &Options{}
	}

	return &Handler{
		opts:   *opts,
		output: w,
	}
}
