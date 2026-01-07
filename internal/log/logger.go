package log

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"sync"

	"github.com/lmittmann/tint"
)

var (
	level       = &slog.LevelVar{}
	rootHandler slog.Handler
	once        sync.Once
)

func SetLevel(logLevel string) {
	level.Set(GetLogLevel(logLevel))
}

func init() {
	// 默认设置为 Info
	level.Set(slog.LevelInfo)
}

type Logger struct {
	*slog.Logger
}

// 重写 Error 方法
func (l *Logger) Error(msg string, err error, args ...any) {
	if err != nil {
		// 显式地组合成 key-value，既能触发我们的 AutoHandler，又能让 vet 闭嘴
		args = append([]any{"err", err}, args...)
	}
	l.Logger.Error(msg, args...)
}

// AutoErrHandler 包装了原有的 Handler
type AutoErrHandler struct {
	slog.Handler
}

func (h *AutoErrHandler) Handle(ctx context.Context, r slog.Record) error {
	// 检查参数中是否有直接传入的 error 类型
	r.Attrs(func(a slog.Attr) bool {
		// 如果发现某个 Value 是 error 类型，且它的 Key 为空（或者是我们约定的特殊占位符）
		// 或者直接检测 Value 类型并修正其 Key
		if err, ok := a.Value.Any().(error); ok && (a.Key == "!BADKEY" || a.Key == "") {
			a.Key = "err"
			a.Value = slog.StringValue(err.Error())
		}
		return true
	})

	return h.Handler.Handle(ctx, r)
}

func getHandler() slog.Handler {
	once.Do(func() {
		level.Set(slog.LevelInfo)
		// 所有的 Logger 共享这一个 Handler 实例
		rootHandler = tint.NewHandler(os.Stdout, &tint.Options{
			AddSource:  true,
			Level:      level,
			TimeFormat: "2006-01-02 15:04:05.000",
		})

		// 嵌套我们的自动错误处理器
	})
	handler := &AutoErrHandler{Handler: rootHandler}
	return handler
}

func GetLogger(module string) *Logger {
	// 每次调用只是基于同一个 Handler 包装一个新的“标签”
	loger := slog.New(getHandler()).With("mod", module)
	return &Logger{loger}
}

func GetLogLevel(level string) slog.Level {
	level = strings.ToLower(level)
	switch level {
	case "debug":
		return slog.LevelDebug
	case "error":
		return slog.LevelError
	case "info":
		return slog.LevelInfo
	case "warning":
		return slog.LevelWarn
	default:
		return slog.LevelInfo
	}
}
