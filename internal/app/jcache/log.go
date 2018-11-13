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
)

type (
	Logger interface {
		Info(format string, args ...interface{})
		Debug(format string, args ...interface{})
		log(skip int, level, format string, args ...interface{})
	}
	logger struct {
		id  string
		out io.Writer
	}
	loggerChain struct {
		loggers []Logger
	}
)

func NewLoggerChain(loggers ...Logger) Logger {
	return &loggerChain{loggers}
}
func NewLogger(out io.Writer) Logger {
	l := logger{
		id:  uuid.New().String(),
		out: out,
	}
	return &l
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

	return NewLogger(out), nil
}
func (l *logger) Info(format string, args ...interface{}) {
	l.log(5, "INFO", format, args...)
}
func (l *logger) Debug(format string, args ...interface{}) {
	l.log(5, "DEBUG", format, args...)
}
func (l *logger) log(skip int, level, format string, args ...interface{}) {
	if l.out == nil {
		return
	}

	//now := time.Now().UTC()
	for i, arg := range args {
		if err, ok := arg.(error); ok {
			args[i] = errors.WithStack(err)
		}
	}

	l.write(skip, level, fmt.Sprintf(format, args...))
}
func (l *logger) write(skip int, level, msg string) {
	f := callerFrame(skip)
	if strings.HasSuffix(msg, "\n") {
		msg = msg[:len(msg)-1]
	}
	gopath := filepath.Join(os.Getenv("GOPATH"), "src")
	file, _ := filepath.Rel(gopath, f.File)
	traced := fmt.Sprintf("[%5s] %s:%d: %s\n", level, file, f.Line, msg)
	io.WriteString(l.out, traced)
}
func callerFrame(skip int) runtime.Frame {
	pc := make([]uintptr, 15)
	n := runtime.Callers(skip, pc)
	frames := runtime.CallersFrames(pc[:n])
	frame, _ := frames.Next()
	return frame
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
