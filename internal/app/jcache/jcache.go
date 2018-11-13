package jcache

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type (
	jCache struct {
		basePath            string
		compileFunc         CompileFunc
		osArgs              []string
		args                ParsedArgs
		cachePath           string
		sourceInfoPath      string
		destinationInfoPath string
		dstHashes           map[string]string
		compilerInfoPath    string
		classesCachePath    string
		includeCachePath    string
		log                 Logger
	}
	CompileFunc func(string, ...string) (string, string, int, error)
)

func NewCache(basePath string, compileFunc CompileFunc, logger Logger, osArgs ...string) (*jCache, error) {
	args, err := ParseArgs(osArgs)
	if err != nil {
		return nil, err
	}

	cachePath := filepath.Join(basePath, args.UUID)
	classesCachePath := filepath.Join(cachePath, "classes")
	includeCachePath := filepath.Join(cachePath, "include")

	err = os.MkdirAll(classesCachePath, os.ModePerm)
	if err != nil {
		return nil, err
	}
	err = os.MkdirAll(includeCachePath, os.ModePerm)
	if err != nil {
		return nil, err
	}

	if args.DstDir != "" {
		err = os.MkdirAll(args.DstDir, os.ModePerm)
		if err != nil {
			return nil, err
		}
	}
	if args.IncDir != "" {
		err = os.MkdirAll(args.IncDir, os.ModePerm)
		if err != nil {
			return nil, err
		}
	}
	if args.GenDir != "" {
		err = os.MkdirAll(args.GenDir, os.ModePerm)
		if err != nil {
			return nil, err
		}
	}

	jc := &jCache{
		basePath:            basePath,
		compileFunc:         compileFunc,
		osArgs:              osArgs,
		args:                args,
		cachePath:           cachePath,
		sourceInfoPath:      filepath.Join(cachePath, "source-info.json"),
		destinationInfoPath: filepath.Join(cachePath, "destination-info.json"),
		compilerInfoPath:    filepath.Join(cachePath, "compiler-info.json"),
		classesCachePath:    classesCachePath,
		includeCachePath:    includeCachePath,
		log:                 logger,
	}

	return jc, nil
}
func (j *jCache) optionIndexOf(option string) int {
	for i, arg := range j.args.FlatArgs {
		if arg == option {
			return i
		}
	}

	return -1
}
func optionIndexOf(args []string, option string) int {
	for i, arg := range args {
		if arg == option {
			return i
		}
	}

	return -1
}
func (j *jCache) repackArgs() []string {
	if len(j.args.FlatArgs) == 0 {
		return j.args.OriginalArgs
	}

	repacked := j.args.FlatArgs
	repacked = redirectArgOption(repacked, "-d", j.classesCachePath, true)
	// adding -h changes the behaviour of javac (v1.8+). We don't want that
	repacked = redirectArgOption(repacked, "-h", j.includeCachePath, false)
	return repacked
}
func redirectArgOption(argsIn []string, option, value string, addIfNotExists bool) []string {
	// re-pack compiler arguments
	optIdx := optionIndexOf(argsIn, option)
	if optIdx >= 0 {
		args := make([]string, len(argsIn))
		copy(args, argsIn)

		args[optIdx+1] = value
		return args
	}
	if !addIfNotExists {
		return argsIn
	}

	// we'll need to add our classes-out-dir
	args := make([]string, len(argsIn)+2)
	args[0] = option
	args[1] = value
	for i := 0; i < len(argsIn); i++ {
		args[i+2] = argsIn[i]
	}
	return args
}
func (j *jCache) Execute() (string, string, int, error) {
	stdout, stderr, exit, err := j.execute()
	if err == nil {
		return stdout, stderr, exit, nil
	}

	// clear my cache and re-compile
	if e := os.RemoveAll(j.cachePath); e != nil {
		return "", "", 0, err // ret original error
	}
	// TODO baeda - we probably want to run JUST javac. nothing altered, to ensure that we actually didn't mess anything up for people.
	return j.execute()
}
func (j *jCache) execute() (string, string, int, error) {
	start := time.Now()

	j.log.Info("%v", j.osArgs)
	defer func() {
		elapsed := time.Since(start)
		j.log.Info("jCache finished in %+v\n.\n.\n.", elapsed)
	}()

	if j.needCompilation() {
		j.log.Info("cache-miss")

		err := MarshalFileInfoSlice(j.args.Sources, j.sourceInfoPath)
		if err != nil {
			return "", "", 0, err
		}

		repackedArgs := j.repackArgs()
		filename, err := writeArgsToTmpFile(repackedArgs)
		if err != nil {
			return "", "", 0, err
		}

		j.log.Info("%s %s\n", j.args.CompilerPath, "@"+filename)
		javacStart := time.Now()
		stdout, stderr, exit, err := j.compileFunc(j.args.CompilerPath, "@"+filename)
		if err != nil {
			return "", "", 0, err
		}

		j.log.Info("javac execution-time: %v", time.Since(javacStart))

		var destinations []string
		err = filepath.Walk(j.classesCachePath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if info.IsDir() {
				return nil
			}

			destinations = append(destinations, path)
			return nil
		})
		if err != nil {
			return "", "", 0, err
		}
		err = MarshalFileInfoSlice(destinations, j.destinationInfoPath)
		if err != nil {
			return "", "", 0, err
		}

		ci := CompilerInfo{
			Out:  stdout,
			Err:  stderr,
			Exit: exit,
		}

		err = MarshalCompilerInfo(ci, j.compilerInfoPath)
		if err != nil {
			return "", "", 0, err
		}
	} else {
		j.log.Info("cache-hit")
	}

	// here we'll just copy
	nFiles, nBytes, err := j.copyCachedFiles()
	if err != nil {
		return "", "", 0, err
	}

	j.log.Info("served %d bytes compiled from %d source files", nBytes, nFiles)

	// replay compiler stdout, stderr and exit
	ci, err := UnmarshalCompilerInfo(j.compilerInfoPath)
	if err != nil {
		return "", "", 0, err
	}

	return ci.Out, ci.Err, ci.Exit, nil
}
func (j *jCache) copyCachedFiles() (nFiles int, nBytes int64, err error) {
	const N = 2

	wg := sync.WaitGroup{}
	wg.Add(N)
	f := make([]int, N)
	b := make([]int64, N)
	e := make([]error, N)

	go func() {
		f[N-2], b[N-2], e[N-2] = copyCachedFiles(j.classesCachePath, j.args.DstDir)
		wg.Done()
	}()
	go func() {
		f[N-1], b[N-1], e[N-1] = copyCachedFiles(j.includeCachePath, j.args.IncDir)
		wg.Done()
	}()

	wg.Wait()

	for _, err = range e {
		if err != nil {
			return
		}
	}

	for _, n := range f {
		nFiles += n
	}
	for _, n := range b {
		nBytes += n
	}

	return
}
func copyCachedFiles(basePath, dstPath string) (nFiles int, nBytes int64, err error) {
	err = filepath.Walk(
		basePath,
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
			fqcp, err := filepath.Rel(basePath, src)
			if err != nil {
				return err
			}

			dst := filepath.Join(dstPath, fqcp)
			if err != nil {
				return err
			}

			err = os.MkdirAll(filepath.Dir(dst), os.ModePerm)
			if err != nil {
				return err
			}

			w, err := CopyFile(src, dst)
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
func (j *jCache) lazyLoadDstHashes() error {
	if j.dstHashes != nil {
		return nil
	}

	infoSlice, err := UnmarshalFileInfoSlice(j.destinationInfoPath)
	if err != nil {
		return err
	}

	m := make(map[string]string)
	for _, info := range infoSlice {
		m[info.Path] = info.Sha256
	}
	j.dstHashes = m

	return nil
}
func (j *jCache) needCompilation() bool {
	if DoesNotExist(j.cachePath) {
		j.log.Info("cache-path does not exist")
		return true
	}

	if DoesNotExist(j.sourceInfoPath) {
		j.log.Info("source-info-path does not exist")
		return true
	}

	if DoesNotExist(j.compilerInfoPath) {
		j.log.Info("source-info-path does not exist")
		return true
	}

	// see if any modified.....
	infoSlice, err := UnmarshalFileInfoSlice(j.sourceInfoPath)
	if err != nil {
		j.log.Info("Failed to read source-info.json. Recompiling. %+v", err)
		return true
	}

	for _, info := range infoSlice {
		stat, err := os.Stat(info.Path)
		if err != nil {
			j.log.Info("Failed to stat %s. Recompiling. %+v", info.Path, err)
			return true
		}

		tStat := stat.ModTime().UTC()
		tInfo := info.ModTime.UTC()
		if !tStat.Equal(tInfo) {
			j.log.Info("%s has been changed. modified: %v - cached: %v",
				info.Path, tStat, tInfo)

			hash, err := Sha256File(info.Path)
			if err != nil {
				j.log.Info("Failed to sha256 sum %s. %+v", info.Path, err)
				return true
			}

			if hash == info.Sha256 {
				j.log.Info("Found identical digest. NOT recompiling.")
				return false
			}

			return true
		}
	}

	return false
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
