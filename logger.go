package coverage

import (
	"log"
	"os"
)

// logger defines an interface for logging messages.
// It allows using any custom logger that implements Infof and Errorf methods.
type logger interface {
	Infof(format string, args ...any)
	Errorf(format string, args ...any)
}

// defaultLogger is a basic implementation of the logger interface.
// It writes info messages to stdout and error messages to stderr.
type defaultLogger struct {
	info  *log.Logger
	error *log.Logger
}

// newDefaultLogger creates and returns a new defaultLogger instance.
// The logger prefixes messages with severity level (INFO/ERROR),
// and includes date, time, and short file name in each log entry.
func newDefaultLogger() *defaultLogger {
	return &defaultLogger{
		info:  log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile),
		error: log.New(os.Stderr, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile),
	}
}

// Infof logs informational messages using the standard "info" logger.
// Example usage:
//
//	l.Infof("Starting coverage server on port %d", 8081)
func (l *defaultLogger) Infof(format string, args ...any) {
	l.info.Printf(format, args...)
}

// Errorf logs error messages using the standard "error" logger.
// Example usage:
//
//	l.Errorf("Failed to read coverage profile: %v", err)
func (l *defaultLogger) Errorf(format string, args ...any) {
	l.error.Printf(format, args...)
}
