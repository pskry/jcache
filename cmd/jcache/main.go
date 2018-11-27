package main

import (
	"flag"
	"fmt"
	"github.com/baeda/jcache/internal/app/jcache"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
)

const UsageText = `Usage: %s [options] COMPILER [compiler options]

Options:
    -c, --clear          clear the cache completely (except configuration)

    -h, --help           print this help text and exit
    -v, --version        print version and copyright information and exit

Full documentation at: <https://github.com/baeda/jcache>
`
const VersionText = `jcache v%s
Copyright (c) 2018 Peter Skrypalle
<my chosen license>
`

const ErrorText = `%[1]s: %[2]v
Try '%[1]s --help' for more information.
`

const (
	ExitSuccess = iota
	ExitErrFlag
	ExitErr
)

var basePath string
var verbose bool

type CLI struct {
	clear   bool
	version bool
}

func init() {
	basePath = os.Getenv("JCACHE_PATH")
	v, err := strconv.ParseBool(os.Getenv("JCACHE_VERBOSE"))
	if err != nil {
		verbose = false
	}
	verbose = v
}

func printUsage() {
	fmt.Fprintf(os.Stderr, UsageText, os.Args[0])
}

func printVersion() {
	fmt.Fprintf(os.Stderr, VersionText, "UNKNOWN")
}

func main() {
	os.Exit(mainExitCode())
}

func mainExitCode() int {
	// Since the flag package has no notion about a "sub-command",
	// we'll need to handle the flag-plumbing ourselves:
	// * Don't panic!
	// * Don't exit!
	// * Don't barf things into stderr unsolicited!
	fs := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	fs.Usage = func() {}
	fs.SetOutput(ioutil.Discard)

	// Setup our flags; excluding -h, --help which comes for free
	cli := CLI{}
	fs.BoolVar(&cli.clear, "c", false, "")
	fs.BoolVar(&cli.clear, "clear", false, "")
	fs.BoolVar(&cli.version, "v", false, "")
	fs.BoolVar(&cli.version, "version", false, "")

	err := fs.Parse(os.Args[1:])
	if err != nil {
		if err == flag.ErrHelp {
			// Printing program help is a terminal operation
			printUsage()
			return ExitSuccess
		}

		fmt.Fprintf(os.Stderr, ErrorText, os.Args[0], err)
		return ExitErrFlag
	}

	if cli.version {
		// Printing version information is a terminal operation
		printVersion()
		return ExitSuccess
	}

	if cli.clear {
		// Clear cache before running
		err = os.RemoveAll(basePath)
		if err != nil {
			fmt.Fprintf(os.Stderr,
				"Error clearing cache dir '%s': %+v\n", basePath, err)
			return ExitErr
		}
	}

	exit, err := jCache(fs.Args())
	if err != nil {
		fmt.Fprintf(os.Stderr, "%+v", err)
		return ExitErr
	}

	return exit
}

func jCache(args []string) (int, error) {
	jc, err := jcache.NewCache(
		basePath,
		jcache.Command,
		mkLogger(),
		args,
	)
	if err != nil {
		return ExitErr, err
	}

	info, err := jc.Execute()
	if err != nil {
		return ExitErr, err
	}

	// Replay javac output
	fmt.Fprint(os.Stdout, info.Stdout)
	fmt.Fprint(os.Stderr, info.Stderr)

	return info.Exit, nil
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
