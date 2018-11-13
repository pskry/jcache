package main

import (
	"fmt"
	"github.com/baeda/jcache/internal/app/jcache"
	"github.com/pkg/errors"
	"os"
	"path/filepath"
	"runtime/pprof"
	"time"
)

const basePath = "/home/baeda/dev/go/src/github.com/baeda/jcache/.opt"

var enableLog = true
var toRun = cli

func main() {
	now := time.Now()
	err := toRun()
	if err != nil {
		panic(err)
	}
	fmt.Fprintf(os.Stderr, "\n\nTIME: %v\n", time.Since(now))
}
func profile() error {
	os.MkdirAll(basePath, os.ModePerm)
	file, _ := os.Create(filepath.Join(basePath, "cpu.prof"))
	pprof.StartCPUProfile(file)
	defer pprof.StopCPUProfile()

	enableLog = false
	x := 0
	for i := 0; i < 5000; i++ {
		info, err := entry()
		if err != nil {
			return err
		}
		x += len(info.Stdout) + len(info.Stderr) + info.Exit
		i++
	}
	fmt.Println(x)

	return nil
}
func cli() error {
	info, err := entry()
	if err != nil {
		return err
	}

	fmt.Fprint(os.Stdout, info.Stdout)
	fmt.Fprint(os.Stderr, info.Stderr)
	os.Exit(info.Exit)

	return nil
}
func entry() (info *jcache.CompilerInfo, err error) {
	jc, err := jcache.NewCache(
		basePath,
		jcache.Command,
		mkLogger(),
		os.Args...,
	)
	if err != nil {
		return nil, err
	}
	return jc.Execute()
}

func mkLogger() jcache.Logger {
	if enableLog {
		stdout := jcache.NewLogger(os.Stdout)
		logger, err := jcache.NewFileLogger(filepath.Join(basePath, "log.txt"))
		if err != nil {
			// well... just log to stdout
			logger = stdout
		} else {
			logger = jcache.NewLoggerChain(logger, stdout)
		}
		return logger
	} else {
		return jcache.NewLogger(nil)
	}
}

func clearAndRetryOnError(err error) {
	if err == nil {
		return
	}

	fmt.Printf("%+v", errors.WithStack(err))
	fatalErr = err

	reRun := toRun
	toRun = fatal
	if err := os.RemoveAll(basePath); err != nil {
		fatal()
	} else {
		// rem went well.
		// retry
		reRun()
	}
}

var fatalErr error

func fatal() error {
	panic(fatalErr)
}
