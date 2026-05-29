// Copyright 2026 The Lattice Authors, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package log

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"sync"
)

var (
	level       = &slog.LevelVar{}
	rootHandler slog.Handler
	once        sync.Once
)

func init() {
	level.Set(slog.LevelInfo)
}

// SetLevel updates the global log level by name ("debug", "info", "warn", "error").
func SetLevel(logLevel string) {
	level.Set(GetLogLevel(logLevel))
}

// Err returns a slog.Attr for an error, for use with structured logging.
func Err(err error) slog.Attr {
	return slog.Any("err", err)
}

// Logger wraps slog.Logger with a convenience Error method that accepts an
// error as the second positional argument (matching existing call sites).
type Logger struct {
	*slog.Logger
}

// AutoErrHandler rewrites bare error values that arrive with an empty or
// "!BADKEY" key to use the canonical "err" key instead.
type AutoErrHandler struct {
	slog.Handler
}

func (h *AutoErrHandler) Handle(ctx context.Context, r slog.Record) error {
	newR := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	r.Attrs(func(a slog.Attr) bool {
		if err, ok := a.Value.Any().(error); ok && (a.Key == "!BADKEY" || a.Key == "") {
			newR.AddAttrs(slog.String("err", err.Error()))
		} else {
			newR.AddAttrs(a)
		}
		return true
	})
	return h.Handler.Handle(ctx, newR)
}

func getHandler() slog.Handler {
	once.Do(func() {
		var inner slog.Handler
		if os.Getenv("LOG_FORMAT") == "json" {
			inner = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
				AddSource: true,
				Level:     level,
			})
		} else {
			inner = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
				AddSource: true,
				Level:     level,
			})
		}
		rootHandler = &AutoErrHandler{Handler: inner}
	})
	return rootHandler
}

func (l *Logger) Error(msg string, err error, args ...any) {
	l.Logger.Error(msg, append([]any{"err", err}, args...)...)
}

// GetLogger returns a Logger tagged with the given module name.
func GetLogger(module string) *Logger {
	logger := slog.New(getHandler()).With("mod", module)
	return &Logger{logger}
}

// GetLogLevel converts a level name to slog.Level.
func GetLogLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "error":
		return slog.LevelError
	case "warn", "warning":
		return slog.LevelWarn
	default:
		return slog.LevelInfo
	}
}
