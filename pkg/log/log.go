// copy from wireguard-go, but refactor some

package log

import (
	"fmt"
	"log"
	"os"
	"strings"
)

// A Logger provides logging for a Device.
// The functions are Printf-style functions.
// They must be safe for concurrent use.
// They do not require a trailing newline in the format.
// If nil, that level of logging will be silent.
type Logger struct {
	moduleName string
	Verbosef   func(format string, args ...any)
	Infof      func(format string, args ...any)
	Warningf   func(format string, args ...any)
	Errorf     func(format string, args ...any)
}

// Log levels for use with NewLogger.
const (
	LogLevelSilent  = iota // No logging
	LogLevelVerbose        // Debug logging
	LogLevelInfo           // Info logging
	LogLevelWarning        // Warning logging
	LogLevelError          // Error logging
)

func logLevel(level string) int {
	level = strings.ToLower(level)
	switch level {
	case "error":
		return LogLevelError
	case "verbose":
		return LogLevelVerbose
	case "info":
		return LogLevelInfo
	case "warning":
		return LogLevelWarning
	default:
		return LogLevelSilent
	}
}

// DiscardLogf Function for use in Logger for discarding logged lines.
func DiscardLogf(format string, args ...any) {}

func (logger *Logger) logf(prefix string) func(string, ...any) {
	return log.New(os.Stdout, fmt.Sprintf("[%s] %s: ", logger.moduleName, prefix), log.Ldate|log.Ltime|log.Lshortfile).Printf
}

// NewLogger constructs a Logger that writes to stdout.
// It logs at the specified log level and above.
// It decorates log lines with the log level, date, time, and prepend.
func NewLogger(level int, prepend string) *Logger {
	logger := &Logger{prepend, DiscardLogf, DiscardLogf, DiscardLogf, DiscardLogf}
	logger.set(level)
	return logger
}

func (logger *Logger) SetLogLevel(level string) *Logger {
	levelInt := logLevel(level)
	logger.set(levelInt)
	return logger
}

func (logger *Logger) set(level int) *Logger {
	switch level {
	case LogLevelSilent:
		logger.Verbosef = DiscardLogf
		logger.Infof = DiscardLogf
		logger.Warningf = DiscardLogf
		logger.Errorf = DiscardLogf
	case LogLevelVerbose:
		logger.Verbosef = logger.logf("DEBUG")
		logger.Infof = logger.logf("INFO")
		logger.Warningf = logger.logf("WARNING")
		logger.Errorf = logger.logf("ERROR")
	case LogLevelInfo:
		logger.Verbosef = DiscardLogf
		logger.Infof = logger.logf("INFO")
		logger.Warningf = logger.logf("WARNING")
		logger.Errorf = logger.logf("ERROR")
	case LogLevelWarning:
		logger.Infof = DiscardLogf
		logger.Verbosef = DiscardLogf
		logger.Warningf = logger.logf("WARNING")
		logger.Errorf = logger.logf("ERROR")
	case LogLevelError:
		logger.Infof = DiscardLogf
		logger.Verbosef = DiscardLogf
		logger.Warningf = DiscardLogf
		logger.Errorf = logger.logf("ERROR")
	default:
		//empty
	}

	return logger
}
