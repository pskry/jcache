package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/baeda/jcache/internal/app/jcache"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const basePath = "/home/baeda/dev/go/src/github.com/baeda/jcache/.opt"

var javacDur time.Duration

var jCacheUUID = uuid.New().String()

func main() {
	start := time.Now()
	stdout, stderr, exit := mainExitCode(basePath, jcache.Command, os.Args...)
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

type CompileFunc func(string, ...string) (string, string, int, error)

func appendLog(format string, args ...interface{}) {
	now := time.Now().UTC()

	os.MkdirAll(basePath, os.ModePerm)
	logf, err := os.OpenFile(filepath.Join(basePath, "log.txt"), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	failOnErr(err)
	defer logf.Close()

	s := fmt.Sprintf(format, args...)
	lines := strings.FieldsFunc(s, func(r rune) bool {
		return r == '\n' || r == '\r'
	})

	if len(lines) == 1 {
		logf.WriteString(fmt.Sprintf("[%s][%s] - %s\n", now.Format(time.RFC3339), jCacheUUID, strings.TrimSpace(s)))
	} else {
		logf.WriteString(fmt.Sprintf("[%s][%s] - %s\n", now.Format(time.RFC3339), jCacheUUID, strings.TrimSpace(lines[0])))
		for i := 1; i < len(lines); i++ {
			logf.WriteString(fmt.Sprintf("                                                             - %s\n", strings.TrimSpace(lines[i])))
		}
	}
}

func mainExitCode(basePath string, compileFunc CompileFunc, osArgs ...string) (string, string, int) {
	start := time.Now()
	appendLog("%v", osArgs)
	defer func() {
		elapsed := time.Since(start)
		appendLog("jCache finished in %+v\n.\n.\n.", elapsed)
	}()

	args, err := jcache.ParseArgs(osArgs)
	failOnErr(err)

	cachePath := filepath.Join(basePath, args.UUID)
	sourceInfoPath := filepath.Join(cachePath, "source-info.json")
	compilerInfoPath := filepath.Join(cachePath, "compiler-info.json")
	dstCachePath := filepath.Join(cachePath, "classes")

	err = os.MkdirAll(dstCachePath, os.ModePerm)
	failOnErr(err)

	if needCompilation(cachePath, sourceInfoPath) {
		appendLog("cache-miss")

		err = os.MkdirAll(cachePath, os.ModePerm)
		failOnErr(err)

		err = writeSourceInfo(args, sourceInfoPath)
		failOnErr(err)

		repackArgs := len(args.FlatArgs) > 0

		var stdout, stderr string
		var exit int

		javacStart := time.Now()
		if repackArgs {
			dOption := -1
			for i, arg := range args.FlatArgs {
				if arg == "-d" {
					dOption = i
					break
				}
			}

			if dOption < 0 {
				// we'll need to add our out-dir
				newArgs := make([]string, len(args.FlatArgs)+2)
				newArgs[0] = "-d"
				newArgs[1] = dstCachePath
				for i := 0; i < len(args.FlatArgs); i++ {
					newArgs[i+2] = args.FlatArgs[i]
				}
				args.FlatArgs = newArgs
			} else {
				args.FlatArgs[dOption+1] = dstCachePath
			}

			fArg, err := ioutil.TempFile("", "jcache_args")
			failOnErr(err)
			for _, arg := range args.FlatArgs {
				fArg.WriteString("\"" + arg + "\"\n")
			}
			fArg.Close()
			defer os.RemoveAll(fArg.Name())
			stdout, stderr, exit, err = compileFunc(args.CompilerPath, "@"+fArg.Name())
		} else {
			stdout, stderr, exit, err = compileFunc(args.CompilerPath, args.OriginalArgs...)
		}
		failOnErr(err)
		javacDur = time.Since(javacStart)
		appendLog("javac execution-time: %v", javacDur)

		ci := CompilerInfo{
			Out:  stdout,
			Err:  stderr,
			Exit: exit,
		}

		err = writeCompilerInfo(ci, compilerInfoPath)
		failOnErr(err)
	} else {
		appendLog("cache-hit")
	}

	copiedFiles := 0
	copiedBytes := int64(0)
	// here we'll just copy
	err = filepath.Walk(dstCachePath, func(path string, info os.FileInfo, err error) error {
		failOnErr(err)
		if info.IsDir() {
			return nil
		}

		if !info.Mode().IsRegular() {
			return fmt.Errorf("not a regular file: %s", path)
		}

		rel, err := filepath.Rel(dstCachePath, path)
		failOnErr(err)
		dstPath := filepath.Join(args.DstDir, rel)

		err = os.MkdirAll(filepath.Dir(dstPath), os.ModePerm)
		failOnErr(err)

		source, err := os.Open(path)
		failOnErr(err)
		defer source.Close()

		destination, err := os.Create(dstPath)
		failOnErr(err)
		defer destination.Close()

		written, err := io.Copy(destination, source)
		failOnErr(err)

		if written != info.Size() {
			return fmt.Errorf("partial copy (%d/%d bytes): %s", written, info.Size(), path)
		}

		copiedBytes += written
		copiedFiles++

		return nil
	})
	failOnErr(err)

	appendLog("served %d bytes compiled from %d source files", copiedBytes, copiedFiles)

	ci, err := readCompilerInfo(compilerInfoPath)
	failOnErr(err)

	return ci.Out, ci.Err, ci.Exit
}

func needCompilation(cachePath, sourceInfoPath string) bool {
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		appendLog("cache-path does not exist")
		return true
	}

	if _, err := os.Stat(sourceInfoPath); os.IsNotExist(err) {
		appendLog("source-info-path does not exist")
		return true
	}

	// see if any modified.....
	infoSlice, err := readSourceInfo(sourceInfoPath)
	if err != nil {
		s := fmt.Sprintf("Failed to read source-info.json. Recompiling. %+v", err)
		fmt.Fprint(os.Stderr, s)
		appendLog(s)
		return true
	}

	for _, info := range infoSlice {
		stat, err := os.Stat(info.Path)
		if err != nil {
			s := fmt.Sprintf("Failed to stat %s. Recompiling. %+v", info.Path, err)
			fmt.Fprint(os.Stderr, s)
			appendLog(s)
			return true
		}

		if !stat.ModTime().UTC().Equal(info.ModTime.UTC()) {
			s := fmt.Sprintf("Note: %s has been changed.\n"+
				"      modified: %v\n"+
				"      cached:   %v",
				info.Path, stat.ModTime().UTC(), info.ModTime.UTC())
			fmt.Fprint(os.Stderr, s)
			appendLog(s)

			fileDigest, err := shaFile(info.Path)
			if err != nil {
				s := fmt.Sprintf("Failed to sha256-digest %s. Recompiling. %+v", info.Path, err)
				fmt.Fprint(os.Stderr, s)
				appendLog(s)
				return true
			}

			if fileDigest == info.Sha256 {
				appendLog("Found identical digest. NOT recompiling.")
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
