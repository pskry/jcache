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
	"time"
)

type (
	Logger interface {
		Info(format string, args ...interface{})
	}
	logger struct {
		id  string
		out io.Writer
	}
)

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
	if l.out == nil {
		return
	}

	now := time.Now().UTC()
	for i, arg := range args {
		if err, ok := arg.(error); ok {
			args[i] = errors.WithStack(err)
		}
	}

	lines := splitLines(format, args...)

	for i, line := range lines {
		if i == 0 {
			l.write(fmtLine(l.id, now.Format(time.RFC3339), line))
		} else {
			l.write(fmtLineContd(line))
		}
	}
}

func (l *logger) write(msg string) {
	f := callerFrame(4)
	if strings.HasSuffix(msg, "\n") {
		msg = msg[:len(msg)-1]
	}
	var traced string
	if strings.HasPrefix(msg, "                                                             -") {
		traced = msg + "\n"
	} else {
		traced = fmt.Sprintf("%s :: %s:%d (%s)\n", msg, f.File, f.Line, f.Function)
	}
	io.WriteString(l.out, traced)
}
func fmtLine(id, timeFmt, msg string) string {
	return fmt.Sprintf("[%s][%s] - %s\n", timeFmt, id, strings.TrimSpace(msg))
}
func fmtLineContd(msg string) string {
	return fmt.Sprintf("                                                             - %s\n", strings.TrimSpace(msg))
}
func splitLines(format string, args ...interface{}) []string {
	formatted := fmt.Sprintf(format, args...)
	return strings.FieldsFunc(formatted, splitByNewline)
}
func splitByNewline(r rune) bool {
	return r == '\n' || r == '\r'
}

func callerFrame(skip int) runtime.Frame {
	pc := make([]uintptr, 15)
	n := runtime.Callers(skip, pc)
	frames := runtime.CallersFrames(pc[:n])
	frame, _ := frames.Next()
	return frame
}
