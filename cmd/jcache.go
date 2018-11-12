package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/baeda/jcache/internal/app/jcache"
	"github.com/pkg/errors"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"time"
)

const basePath = "/home/baeda/dev/go/src/github.com/baeda/jcache/.opt"

var javacDur time.Duration

func main() {
	start := time.Now()
	jc, err := newCache(basePath, jcache.Command, os.Args...)
	failOnErr(err)
	stdout, stderr, exit := jc.mainExitCode()
	elapsed := time.Since(start)
	fmt.Fprintf(os.Stderr, "Note: jCache finished in %+v\n", elapsed)
	if javacDur == 0 {
		fmt.Fprintln(os.Stderr, "Note: javac was not invoked.")
	} else {
		fmt.Fprintf(os.Stderr, "Note: javac took %+v\n", javacDur)
	}
	fmt.Fprint(os.Stdout, stdout)
	fmt.Fprint(os.Stderr, stderr)
	os.Exit(exit)
}

type SourceInfo struct {
	Path    string
	ModTime time.Time
	Sha256  string
}

type CompilerInfo struct {
	Out  string
	Err  string
	Exit int
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

var l jcache.Logger

func init() {
	ll, err := jcache.NewLogger(filepath.Join(basePath, "newlog.txt"))
	failOnErr(err)
	l = ll
}

func (j *jCache) mainExitCode() (string, string, int) {
	start := time.Now()

	l.Info("%v", j.osArgs)
	defer func() {
		elapsed := time.Since(start)
		l.Info("jCache finished in %+v\n.\n.\n.", elapsed)
	}()

	if j.needCompilation() {
		l.Info("cache-miss")

		err := writeSourceInfo(j.args, j.sourceInfoPath)
		failOnErr(err)

		repackArgs := len(j.args.FlatArgs) > 0

		var stdout, stderr string
		var exit int

		javacStart := time.Now()
		if repackArgs {
			dOption := -1
			for i, arg := range j.args.FlatArgs {
				if arg == "-d" {
					dOption = i
					break
				}
			}

			if dOption < 0 {
				// we'll need to add our out-dir
				newArgs := make([]string, len(j.args.FlatArgs)+2)
				newArgs[0] = "-d"
				newArgs[1] = j.dstCachePath
				for i := 0; i < len(j.args.FlatArgs); i++ {
					newArgs[i+2] = j.args.FlatArgs[i]
				}
				j.args.FlatArgs = newArgs
			} else {
				j.args.FlatArgs[dOption+1] = j.dstCachePath
			}

			fArg, err := ioutil.TempFile("", "jcache_args")
			failOnErr(err)
			for _, arg := range j.args.FlatArgs {
				fArg.WriteString("\"" + arg + "\"\n")
			}
			fArg.Close()
			defer os.RemoveAll(fArg.Name())
			stdout, stderr, exit, err = j.compileFunc(j.args.CompilerPath, "@"+fArg.Name())
		} else {
			stdout, stderr, exit, err = j.compileFunc(j.args.CompilerPath, j.args.OriginalArgs...)
		}
		failOnErr(err)
		javacDur = time.Since(javacStart)
		l.Info("javac execution-time: %v", javacDur)

		ci := CompilerInfo{
			Out:  stdout,
			Err:  stderr,
			Exit: exit,
		}

		err = writeCompilerInfo(ci, j.compilerInfoPath)
		failOnErr(err)
	} else {
		l.Info("cache-hit")
	}

	// here we'll just copy
	nFiles, nBytes, err := j.copyCachedFiles()
	failOnErr(err)

	l.Info("served %d bytes compiled from %d source files", nBytes, nFiles)

	// replay compiler stdout, stderr and exit
	ci, err := readCompilerInfo(j.compilerInfoPath)
	failOnErr(err)

	return ci.Out, ci.Err, ci.Exit
}

func (j *jCache) copyCachedFiles() (nFiles int, nBytes int64, err error) {
	err = filepath.Walk(j.dstCachePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			// continue walking.
			return nil
		}

		if !info.Mode().IsRegular() {
			return fmt.Errorf("file not regular: %s", path)
		}

		rel, err := filepath.Rel(j.dstCachePath, path)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(j.args.DstDir, rel)

		err = os.MkdirAll(filepath.Dir(dstPath), os.ModePerm)
		if err != nil {
			return err
		}

		source, err := os.Open(path)
		if err != nil {
			return err
		}
		defer source.Close()

		destination, err := os.Create(dstPath)
		if err != nil {
			return err
		}
		defer destination.Close()

		w, err := io.Copy(destination, source)
		if err != nil {
			return err
		}

		if w != info.Size() {
			return fmt.Errorf("partial copy (%d/%d bytes): %s", w, info.Size(), path)
		}

		nBytes += w
		nFiles++

		return nil
	})

	return
}

func (j *jCache) needCompilation() bool {
	if _, err := os.Stat(j.cachePath); os.IsNotExist(err) {
		l.Info("cache-path does not exist")
		return true
	}

	if _, err := os.Stat(j.sourceInfoPath); os.IsNotExist(err) {
		l.Info("source-info-path does not exist")
		return true
	}

	// see if any modified.....
	infoSlice, err := readSourceInfo(j.sourceInfoPath)
	if err != nil {
		s := fmt.Sprintf("Failed to read source-info.json. Recompiling. %+v", err)
		fmt.Fprint(os.Stderr, s)
		l.Info(s)
		return true
	}

	for _, info := range infoSlice {
		stat, err := os.Stat(info.Path)
		if err != nil {
			s := fmt.Sprintf("Failed to stat %s. Recompiling. %+v", info.Path, err)
			fmt.Fprint(os.Stderr, s)
			l.Info(s)
			return true
		}

		if !stat.ModTime().UTC().Equal(info.ModTime.UTC()) {
			s := fmt.Sprintf("Note: %s has been changed.\n"+
				"      modified: %v\n"+
				"      cached:   %v",
				info.Path, stat.ModTime().UTC(), info.ModTime.UTC())
			fmt.Fprint(os.Stderr, s)
			l.Info(s)

			fileDigest, err := shaFile(info.Path)
			if err != nil {
				s := fmt.Sprintf("Failed to sha256-digest %s. Recompiling. %+v", info.Path, err)
				fmt.Fprint(os.Stderr, s)
				l.Info(s)
				return true
			}

			if fileDigest == info.Sha256 {
				l.Info("Found identical digest. NOT recompiling.")
				return false
			}

			return true
		}
	}

	return false
}

func readSourceInfo(sourceInfoPath string) ([]SourceInfo, error) {
	var infoSlice []SourceInfo

	data, err := ioutil.ReadFile(sourceInfoPath)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(data, &infoSlice)
	if err != nil {
		return nil, err
	}

	return infoSlice, nil
}

func writeSourceInfo(args jcache.ParsedArgs, sourceInfoPath string) error {
	var infoSlice []SourceInfo

	for _, src := range args.Sources {
		stat, err := os.Stat(src)
		if err != nil {
			return err
		}

		fileDigest, err := shaFile(src)
		if err != nil {
			return err
		}

		info := SourceInfo{src, stat.ModTime().UTC(), fileDigest}
		infoSlice = append(infoSlice, info)
	}
	data, err := json.Marshal(infoSlice)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(sourceInfoPath, data, 0644)
	if err != nil {
		return err
	}

	return nil
}

func shaFile(name string) (string, error) {
	bytes, err := ioutil.ReadFile(name)
	if err != nil {
		return "", err
	}

	sum256 := sha256.Sum256(bytes)
	return hex.EncodeToString(sum256[:]), nil
}

func readCompilerInfo(compilerInfoPath string) (ci CompilerInfo, err error) {
	data, err := ioutil.ReadFile(compilerInfoPath)
	if err != nil {
		return
	}
	err = json.Unmarshal(data, &ci)
	if err != nil {
		return
	}

	return
}

func writeCompilerInfo(ci CompilerInfo, compilerInfoPath string) error {
	data, err := json.Marshal(ci)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(compilerInfoPath, data, 0644)
	if err != nil {
		return err
	}

	return nil
}

func failOnErr(err error) {
	if err != nil {
		log.Fatalf("%+v", errors.WithStack(err))
	}
}
