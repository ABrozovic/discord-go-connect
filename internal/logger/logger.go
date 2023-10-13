package logger

import (
	"fmt"
	"io"
	"log"
	"runtime/debug"
)

// LogLevel represents the severity level of log messages.
type LogLevel int

const (
	Info LogLevel = iota
	Error
	Debug
)

// Handler represents the error handling and logging interface.
type Handler interface {
	Info(format string, args ...interface{})
	Error(format string, args ...interface{})
	Debug(format string, args ...interface{})
}

// StandardLoggerHandler is a standard implementation of the LoggerHandler interface.
type StandardLoggerHandler struct {
	infoLogger  *log.Logger
	errorLogger *log.Logger
	debugLogger *log.Logger
}

// NewLogger creates a new instance of StandardErrorHandler.
func NewLogger(out io.Writer) *StandardLoggerHandler {
	return &StandardLoggerHandler{
		infoLogger:  log.New(out, "[INFO] ", log.Ldate|log.Ltime),
		errorLogger: log.New(out, "[ERROR] ", log.Ldate|log.Ltime|log.Llongfile),
		debugLogger: log.New(out, "[DEBUG] ", log.Ldate|log.Ltime|log.Llongfile),
	}
}

// Info logs an informational message.
func (e *StandardLoggerHandler) Info(format string, args ...interface{}) {
	e.logMessage(Info, format, args...)
}

// Error logs an error message.
func (e *StandardLoggerHandler) Error(format string, args ...interface{}) {
	e.logMessage(Error, format, args...)
}

// Debug logs a debug message.
func (e *StandardLoggerHandler) Debug(format string, args ...interface{}) {
	e.logMessage(Debug, format, args...)
}

// logMessage logs a message at the specified log level.
func (e *StandardLoggerHandler) logMessage(level LogLevel, format string, args ...interface{}) {
	var logger *log.Logger

	var stackTrace string

	switch level {
	case Info:
		logger = e.infoLogger
	case Error:
		logger = e.errorLogger
		stackTrace = string(debug.Stack())
	case Debug:
		logger = e.debugLogger
	default:
		logger = e.infoLogger
	}

	message := fmt.Sprintf(format, args...)
	logMessage := fmt.Sprintf("%s\n%s", message, stackTrace)

	if err := logger.Output(3, logMessage); err != nil {
		log.Fatal(err)
	}
}
