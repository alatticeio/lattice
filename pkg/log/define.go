package log

var Loglevel = logLevel("silent")

func SetLogLevel(level string) int {
	Loglevel = logLevel(level)
	return Loglevel
}
