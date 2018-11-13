package main

import (
	"fmt"
	"github.com/baeda/jcache/internal/app/jcache"
	"github.com/pkg/errors"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"runtime/pprof"
	"time"
)

const basePath = "/home/baeda/dev/go/src/github.com/baeda/jcache/.opt"

var javacDur time.Duration
var l jcache.Logger
var enableLog = true

func init() {
	if enableLog {
		ll, err := jcache.NewFileLogger(filepath.Join(basePath, "log.txt"))
		if err != nil {
			// well... just log to stderr
			ll = jcache.NewLogger(os.Stderr)
		}
		l = ll
	} else {
		l = jcache.NewLogger(nil)
	}
}

func main() {
	now := time.Now()
	main1()
	fmt.Fprintf(os.Stderr, "TIME: %v\n", time.Since(now))
}

func main1() {
	os.MkdirAll(basePath, os.ModePerm)
	file, _ := os.Create(filepath.Join(basePath, "cpu.prof"))
	pprof.StartCPUProfile(file)
	defer pprof.StopCPUProfile()

	x := 0
	for i := 0; i < 10000; i++ {
		x += main2(false)
	}

	fmt.Print(x)
}

func main22() {
	main2(true)
}

func main2(v bool) int {
	//os.MkdirAll(basePath, os.ModePerm)
	//file, err := os.Create(filepath.Join(basePath, "cpu.prof"))
	//pprof.StartCPUProfile(file)
	//failOnErr(err)
	//defer pprof.StopCPUProfile()

	start := time.Now()
	jc, err := newCache(basePath, jcache.Command, os.Args...)
	failOnErr(err)
	stdout, stderr, exit, err := jc.mainExitCode()
	failOnErr(err)

	elapsed := time.Since(start)
	if v {
		fmt.Fprintf(os.Stderr, "Note: jCache finished in %+v\n", elapsed)
		if javacDur == 0 {
			fmt.Fprintln(os.Stderr, "Note: javac was not invoked.")
		} else {
			fmt.Fprintf(os.Stderr, "Note: javac took %+v\n", javacDur)
		}
		fmt.Fprint(os.Stdout, stdout)
		fmt.Fprint(os.Stderr, stderr)
	}
	//os.Exit(exit)
	return len(stdout) + len(stderr) + exit + rand.Int()
}

type jCache struct {
	basePath         string
	compileFunc      CompileFunc
	osArgs           []string
	args             jcache.ParsedArgs
	cachePath        string
	sourceInfoPath   string
	compilerInfoPath string
	dstCachePath     string
}

func newCache(basePath string, compileFunc CompileFunc, osArgs ...string) (*jCache, error) {
	args, err := jcache.ParseArgs(osArgs)
	if err != nil {
		return nil, err
	}

	cachePath := filepath.Join(basePath, args.UUID)
	dstCachePath := filepath.Join(cachePath, "classes")

	err = os.MkdirAll(dstCachePath, os.ModePerm)
	if err != nil {
		return nil, err
	}

	jc := &jCache{
		basePath:         basePath,
		compileFunc:      compileFunc,
		osArgs:           osArgs,
		args:             args,
		cachePath:        cachePath,
		sourceInfoPath:   filepath.Join(cachePath, "source-info.json"),
		compilerInfoPath: filepath.Join(cachePath, "compiler-info.json"),
		dstCachePath:     dstCachePath,
	}

	return jc, nil
}

type CompileFunc func(string, ...string) (string, string, int, error)

func (j *jCache) dOptionIndex() int {
	for i, arg := range j.args.FlatArgs {
		if arg == "-d" {
			return i
		}
	}

	return -1
}

func (j *jCache) repackArgs() []string {
	if len(j.args.FlatArgs) == 0 {
		return j.args.OriginalArgs
	}

	// re-pack compiler arguments
	dOption := j.dOptionIndex()
	if dOption < 0 {
		// we'll need to add our out-dir
		args := make([]string, len(j.args.FlatArgs)+2)
		args[0] = "-d"
		args[1] = j.dstCachePath
		for i := 0; i < len(j.args.FlatArgs); i++ {
			args[i+2] = j.args.FlatArgs[i]
		}
		return args
	} else {
		args := make([]string, len(j.args.FlatArgs))
		copy(args, j.args.FlatArgs)

		args[dOption+1] = j.dstCachePath
		return args
	}
}

func writeArgsToTmpFile(args []string) (filename string, err error) {
	file, err := ioutil.TempFile("", "jcache_args")
	if err != nil {
		return
	}
	defer file.Close()

	fileName := file.Name()

	for _, arg := range args {
		file.WriteString("\"" + arg + "\"\n")
	}

	return fileName, nil
}

func (j *jCache) mainExitCode() (string, string, int, error) {
	start := time.Now()

	l.Info("%v", j.osArgs)
	defer func() {
		elapsed := time.Since(start)
		l.Info("jCache finished in %+v\n.\n.\n.", elapsed)
	}()

	if j.needCompilation() {
		l.Info("cache-miss")

		err := jcache.MarshalSourceInfoSlice(j.args.Sources, j.sourceInfoPath)
		if err != nil {
			return "", "", 0, err
		}

		var stdout, stderr string
		var exit int

		javacStart := time.Now()
		repackedArgs := j.repackArgs()
		if len(repackedArgs) < 8 { // TODO baeda - find reasonable threshold
			stdout, stderr, exit, err = j.compileFunc(j.args.CompilerPath, repackedArgs...)
		} else {
			filename, err := writeArgsToTmpFile(repackedArgs)
			if err != nil {
				return "", "", 0, err
			}

			stdout, stderr, exit, err = j.compileFunc(j.args.CompilerPath, "@"+filename)
		}
		if err != nil {
			return "", "", 0, err
		}

		javacDur = time.Since(javacStart)
		l.Info("javac execution-time: %v", javacDur)

		ci := jcache.CompilerInfo{
			Out:  stdout,
			Err:  stderr,
			Exit: exit,
		}

		err = jcache.MarshalCompilerInfo(ci, j.compilerInfoPath)
		if err != nil {
			return "", "", 0, err
		}
	} else {
		l.Info("cache-hit")
	}

	// here we'll just copy
	nFiles, nBytes, err := j.copyCachedFiles()
	if err != nil {
		return "", "", 0, err
	}

	l.Info("served %d bytes compiled from %d source files", nBytes, nFiles)

	// replay compiler stdout, stderr and exit
	ci, err := jcache.UnmarshalCompilerInfo(j.compilerInfoPath)
	if err != nil {
		return "", "", 0, err
	}

	return ci.Out, ci.Err, ci.Exit, nil
}

func (j *jCache) copyCachedFiles() (nFiles int, nBytes int64, err error) {
	err = filepath.Walk(
		j.dstCachePath,
		func(src string, srcInfo os.FileInfo, err error) error {
			// abort walking on first error encountered
			if err != nil {
				return err
			}

			if srcInfo.IsDir() {
				return nil
			}

			if !srcInfo.Mode().IsRegular() {
				return fmt.Errorf("file not regular: %s", src)
			}

			// construct fully qualified class name (JVM style)
			fqcp, err := filepath.Rel(j.dstCachePath, src)
			if err != nil {
				return err
			}

			dst := filepath.Join(j.args.DstDir, fqcp)

			if _, err := os.Stat(dst); err == nil {
				if srcHash, err := jcache.Sha256File(src); err == nil {
					if dstHash, err := jcache.Sha256File(dst); err == nil {
						if srcHash == dstHash {
							l.Info("skipping %s", src)
							return nil
						}
					}
				}
			}

			err = os.MkdirAll(filepath.Dir(dst), os.ModePerm)
			if err != nil {
				return err
			}

			w, err := jcache.CopyFile(src, dst)
			if err != nil {
				return err
			}

			if w != srcInfo.Size() {
				return fmt.Errorf(
					"partial copy (%d/%d bytes): %s",
					w, srcInfo.Size(), src)
			}

			nBytes += w
			nFiles++

			return nil
		})

	return
}

func (j *jCache) needCompilation() bool {
	if jcache.DoesNotExist(j.cachePath) {
		l.Info("cache-path does not exist")
		return true
	}

	if jcache.DoesNotExist(j.sourceInfoPath) {
		l.Info("source-info-path does not exist")
		return true
	}

	// see if any modified.....
	infoSlice, err := jcache.UnmarshalSourceInfoSlice(j.sourceInfoPath)
	if err != nil {
		l.Info("Failed to read source-info.json. Recompiling. %+v", err)
		return true
	}

	for _, info := range infoSlice {
		stat, err := os.Stat(info.Path)
		if err != nil {
			l.Info("Failed to stat %s. Recompiling. %+v", info.Path, err)
			return true
		}

		tStat := stat.ModTime().UTC()
		tInfo := info.ModTime.UTC()
		if !tStat.Equal(tInfo) {
			l.Info("%s has been changed. modified: %v - cached: %v",
				info.Path, tStat, tInfo)

			hash, err := jcache.Sha256File(info.Path)
			if err != nil {
				l.Info("Failed to sha256 sum %s. %+v", info.Path, err)
				return true
			}

			if hash == info.Sha256 {
				l.Info("Found identical digest. NOT recompiling.")
				return false
			}

			return true
		}
	}

	return false
}

func failOnErr(err error) {
	if err != nil {
		log.Fatalf("%+v", errors.WithStack(err))
	}
}
