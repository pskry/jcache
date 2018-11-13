package main

import (
	"fmt"
	"github.com/baeda/jcache/internal/app/jcache"
	"github.com/pkg/errors"
	"log"
	"os"
	"path/filepath"
	"runtime/pprof"
	"time"
)

const basePath = "/home/baeda/dev/go/src/github.com/baeda/jcache/.opt"

var enableLog = true

func main() {
	now := time.Now()
	cli()
	fmt.Fprintf(os.Stderr, "\n\nTIME: %v\n", time.Since(now))
}
func profile() {
	os.MkdirAll(basePath, os.ModePerm)
	file, _ := os.Create(filepath.Join(basePath, "cpu.prof"))
	pprof.StartCPUProfile(file)
	defer pprof.StopCPUProfile()

	enableLog = false
	x := 0
	for i := 0; i < 5000; i++ {
		stdout, stderr, exit, err := entry()
		failOnErr(err)
		x += len(stdout) + len(stderr) + exit
		i++
	}
	fmt.Println(x)
}
func cli() {
	stdout, stderr, exit, err := entry()
	failOnErr(err)

	fmt.Fprint(os.Stdout, stdout)
	fmt.Fprint(os.Stderr, stderr)
	os.Exit(exit)
}
func entry() (stdout, stderr string, exit int, err error) {
	jc, err := jcache.NewCache(
		basePath,
		jcache.Command,
		mkLogger(),
		os.Args...,
	)
	if err != nil {
		return "", "", 0, err
	}
	return jc.Execute()
}

func mkLogger() jcache.Logger {
	if enableLog {
		logger, err := jcache.NewFileLogger(filepath.Join(basePath, "log.txt"))
		if err != nil {
			// well... just log to stderr
			logger = jcache.NewLogger(os.Stderr)
		}
		return logger
	} else {
		return jcache.NewLogger(nil)
	}
}

func failOnErr(err error) {
	if err != nil {
		log.Fatalf("%+v", errors.WithStack(err))
	}
}
