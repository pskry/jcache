package main

import (
	"flag"
	"fmt"
	"github.com/baeda/jcache/internal/app/jcache"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
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
	runtime.GOMAXPROCS(runtime.NumCPU() * 2)

	evict := flag.Bool("e", false, "evict cache")
	flag.Parse()
	if *evict {
		os.RemoveAll(basePath)
		os.Exit(0)
	}

	err := runAsCompilerFacade(os.Args)
	if err != nil {
		panic(err)
	}
}

func runAsStandalone() {

}

func runAsCompilerFacade(args []string) error {
	jc, err := jcache.NewCache(
		basePath,
		jcache.Command,
		mkLogger(),
		args,
	)
	if err != nil {
		return err
	}

	info, err := jc.Execute()
	if err != nil {
		return err
	}

	fmt.Fprint(os.Stdout, info.Stdout)
	fmt.Fprint(os.Stderr, info.Stderr)
	os.Exit(info.Exit)

	return nil
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
