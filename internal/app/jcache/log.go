package jcache

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

type Logger interface {
	Info(format string, args ...interface{})
	Debug(format string, args ...interface{})
	log(skip int, level, format string, args ...interface{})
}

type logger struct {
	id  string
	out io.Writer
}

func NewLogger(out io.Writer) Logger {
	l := logger{
		id:  uuid.New().String(),
		out: out,
	}
	return &l
}
func (l *logger) Info(format string, args ...interface{}) {
	l.log(5, "INFO", format, args...)
}
func (l *logger) Debug(format string, args ...interface{}) {
	l.log(5, "DEBUG", format, args...)
}
func (l *logger) log(skip int, level, format string, args ...interface{}) {
	for i, arg := range args {
		if err, ok := arg.(error); ok {
			args[i] = errors.WithStack(err)
		}
	}

	l.write(skip, level, fmt.Sprintf(format, args...))
}
func (l *logger) write(skip int, level, msg string) {
	f := getCallerFrame(skip)
	if strings.HasSuffix(msg, "\n") {
		msg = msg[:len(msg)-1]
	}
	goPath := filepath.Join(os.Getenv("GOPATH"), "src")
	file, _ := filepath.Rel(goPath, f.File)
	traced := fmt.Sprintf("[%5s] %s:%d: %s\n", level, file, f.Line, msg)
	io.WriteString(l.out, traced)
}
func getCallerFrame(skip int) runtime.Frame {
	pc := make([]uintptr, 15)
	n := runtime.Callers(skip, pc)
	frames := runtime.CallersFrames(pc[:n])
	frame, _ := frames.Next()
	return frame
}

type syncedWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (s *syncedWriter) Write(p []byte) (n int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.w.Write(p)
}

func NewFileLogger(logFile string) (Logger, error) {
	dir := filepath.Dir(logFile)
	err := os.MkdirAll(dir, os.ModePerm)
	if err != nil {
		return nil, err
	}

	out, err := os.OpenFile(logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}

	return NewLogger(&syncedWriter{sync.Mutex{}, out}), nil
}

type loggerChain struct {
	loggers []Logger
}

func NewLoggerChain(loggers ...Logger) Logger {
	return &loggerChain{loggers}
}
func (l *loggerChain) Info(format string, args ...interface{}) {
	l.log(5, "INFO", format, args...)
}
func (l *loggerChain) Debug(format string, args ...interface{}) {
	l.log(5, "DEBUG", format, args...)
}
func (l *loggerChain) log(skip int, level, format string, args ...interface{}) {
	for _, logger := range l.loggers {
		logger.log(skip+1, level, format, args...)
	}
}
