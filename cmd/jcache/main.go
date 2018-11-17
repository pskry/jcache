package main

import (
	"flag"
	"fmt"
	"github.com/baeda/jcache/internal/app/jcache"
	"os"
	"path/filepath"
	"runtime/pprof"
	"strconv"
	"time"
)

var basePath string
var verbose bool

func init() {
	basePath = os.Getenv("JCACHE_PATH")
	v, err := strconv.ParseBool(os.Getenv("JCACHE_VERBOSE"))
	if err != nil {
		verbose = false
	}
	verbose = v
}

func main() {
	evict := flag.Bool("e", false, "evict cache")
	flag.Parse()
	if *evict {
		os.RemoveAll(basePath)
		os.Exit(0)
	}

	now := time.Now()
	err := cli()
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

	verbose = false
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
	if verbose {
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
