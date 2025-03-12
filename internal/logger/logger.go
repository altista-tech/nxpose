package logger

import (
	"os"

	"github.com/sirupsen/logrus"
)

// Logger is a wrapper around logrus.Logger
type Logger struct {
	*logrus.Logger
}

// New creates a new logger instance
func New(verbose bool) *Logger {
	log := logrus.New()
	log.SetOutput(os.Stdout)

	// Set log level based on verbose flag
	if verbose {
		log.SetLevel(logrus.DebugLevel)
	} else {
		log.SetLevel(logrus.InfoLevel)
	}

	// Set formatter
	log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	return &Logger{log}
}

// SetVerbose sets the logger to verbose mode
func (l *Logger) SetVerbose(verbose bool) {
	if verbose {
		l.SetLevel(logrus.DebugLevel)
	} else {
		l.SetLevel(logrus.InfoLevel)
	}
}

// WithField adds a field to the log entry
func (l *Logger) WithField(key string, value interface{}) *logrus.Entry {
	return l.Logger.WithField(key, value)
}

// WithFields adds multiple fields to the log entry
func (l *Logger) WithFields(fields map[string]interface{}) *logrus.Entry {
	return l.Logger.WithFields(logrus.Fields(fields))
}

// WithError adds an error to the log entry
func (l *Logger) WithError(err error) *logrus.Entry {
	return l.Logger.WithError(err)
}

// Debug logs a debug message
func (l *Logger) Debug(msg string) {
	l.Logger.Debug(msg)
}

// Info logs an info message
func (l *Logger) Info(msg string) {
	l.Logger.Info(msg)
}

// Warn logs a warning message
func (l *Logger) Warn(msg string) {
	l.Logger.Warn(msg)
}

// Error logs an error message
func (l *Logger) Error(msg string) {
	l.Logger.Error(msg)
}

// Fatal logs a fatal message and exits with status code 1
func (l *Logger) Fatal(msg string) {
	l.Logger.Fatal(msg)
}
