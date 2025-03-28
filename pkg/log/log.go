// copy from wireguard-go, but refactor some

package log

import (
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
	//return log.New(os.Stdout, prefix+": "+logger.moduleName, log.Ldate|log.Ltime|log.Lshortfile).Printf
	return log.New(os.Stdout, logger.moduleName+" "+prefix+": ", log.Ldate|log.Ltime|log.Lshortfile).Printf
}

// NewLogger constructs a Logger that writes to stdout.
// It logs at the specified log level and above.
// It decorates log lines with the log level, date, time, and prepend.
func NewLogger(level int, prepend string) *Logger {
	logger := &Logger{prepend, DiscardLogf, DiscardLogf, DiscardLogf, DiscardLogf}

	if level >= LogLevelVerbose {
		logger.Verbosef = logger.logf("DEBUG")
	}

	if level >= LogLevelInfo {
		logger.Infof = logger.logf("INFO")
	}

	if level >= LogLevelWarning {
		logger.Warningf = logger.logf("WARNING")
	}

	if level >= LogLevelError {
		logger.Errorf = logger.logf("ERROR")
	}
	return logger
}

func (logger *Logger) SetLogLevel(level string) *Logger {

	switch logLevel(level) {
	case LogLevelSilent:
		logger.Verbosef = DiscardLogf
		logger.Infof = DiscardLogf
		logger.Warningf = DiscardLogf
		logger.Errorf = DiscardLogf
	case LogLevelVerbose:
		logger.Verbosef = logger.logf("DEBUG")
		logger.Infof = DiscardLogf
		logger.Errorf = DiscardLogf
	case LogLevelInfo:
		logger.Infof = logger.logf("INFO")
		logger.Verbosef = logger.logf("DEBUG")
		logger.Warningf = DiscardLogf
		logger.Errorf = DiscardLogf
	case LogLevelWarning:
		logger.Infof = logger.logf("INFO")
		logger.Verbosef = logger.logf("DEBUG")
		logger.Warningf = logger.logf("WARNING")
		logger.Errorf = DiscardLogf
	case LogLevelError:
		logger.Infof = logger.logf("INFO")
		logger.Verbosef = logger.logf("DEBUG")
		logger.Warningf = logger.logf("WARNING")
		logger.Errorf = logger.logf("WARNING")
	default:
		//empty
	}

	return logger
}
