package jcache

import (
	"fmt"
	"github.com/google/uuid"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Logger interface {
	Info(format string, args ...interface{})
}

func NewLogger(logFile string) (Logger, error) {
	dir := filepath.Dir(logFile)
	err := os.MkdirAll(dir, os.ModePerm)
	if err != nil {
		return nil, err
	}

	out, err := os.OpenFile(logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}

	l := logger{
		id:  uuid.New().String(),
		out: out,
	}
	return &l, nil
}

type logger struct {
	id  string
	out io.WriteCloser
}

func (l *logger) Info(format string, args ...interface{}) {
	now := time.Now().UTC()
	lines := splitLines(format, args...)

	for i, line := range lines {
		if i == 0 {
			io.WriteString(l.out, l.fmtLine(now.Format(time.RFC3339), line))
		} else {
			io.WriteString(l.out, l.fmtLineContd(line))
		}
	}
}

func (l *logger) Close() error {
	return l.out.Close()
}

func (l *logger) fmtLine(timeFmt string, msg string) string {
	return fmt.Sprintf("[%s][%s] - %s\n", timeFmt, l.id, strings.TrimSpace(msg))
}

func (l *logger) fmtLineContd(msg string) string {
	return fmt.Sprintf("                                                             - %s\n", strings.TrimSpace(msg))
}

func splitLines(format string, args ...interface{}) []string {
	formatted := fmt.Sprintf(format, args...)
	return strings.FieldsFunc(formatted, splitByNewline)
}

func splitByNewline(r rune) bool {
	return r == '\n' || r == '\r'
}
