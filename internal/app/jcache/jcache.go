package jcache

import (
	"github.com/pkg/errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type (
	jCache struct {
		args             ParsedArgs
		cachePath        string
		sourceInfoPath   string
		compilerInfoPath string
		classesCachePath string
		includeCachePath string
		log              Logger
		compileFunc      CompileFunc
	}
	CompileFunc func(string, ...string) (*ExecInfo, error)
)

func NewCache(basePath string, compileFunc CompileFunc, logger Logger, osArgs []string) (*jCache, error) {
	args, err := ParseArgs(osArgs)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	cachePath := filepath.Join(basePath, args.UUID)
	classesCachePath := filepath.Join(cachePath, "classes")
	includeCachePath := filepath.Join(cachePath, "include")

	jc := &jCache{
		compileFunc:      compileFunc,
		args:             args,
		cachePath:        cachePath,
		sourceInfoPath:   filepath.Join(cachePath, "source-info.json"),
		compilerInfoPath: filepath.Join(cachePath, "compiler-info.json"),
		classesCachePath: classesCachePath,
		includeCachePath: includeCachePath,
		log:              logger,
	}

	jc.mkDirs()

	return jc, nil
}

func (j *jCache) Execute() (info *ExecInfo, err error) {
	executeStart := time.Now()

	j.log.Info("%v", j.args.OriginalArgs)
	defer func() {
		elapsed := time.Since(executeStart)
		j.log.Info("jCache finished in %+v\n.\n.\n.", elapsed)
	}()

	start := time.Now()
	needCompilation := j.needCompilation()
	j.log.Info("determining cache state finished in %v", time.Since(start))

	if needCompilation {
		info, err = j.compile()
		if err != nil {
			return
		}
	} else {
		j.log.Info("cache hit")
	}

	// here we'll just copy
	copyStart := time.Now()
	nFiles, nBytes, err := j.copyCachedFiles()
	j.log.Info("copying files finished in %v", time.Since(copyStart))
	if err != nil {
		return
	}

	j.log.Info("served %d bytes compiled from %d source files", nBytes, nFiles)

	if info == nil {
		// load compiler-info from disk if we had a cache hit
		info, err = UnmarshalExecInfo(j.compilerInfoPath)
		if err != nil {
			return
		}
	}
	return
}

func (j *jCache) compile() (info *ExecInfo, err error) {
	if err = validateCompiler(j.args.CompilerPath); err != nil {
		return nil, errors.WithStack(err)
	}

	j.log.Info("cache miss")
	j.log.Debug("unlink %s", j.cachePath)
	os.RemoveAll(j.cachePath)
	j.mkDirs()

	err = MarshalFileInfoSlice(j.args.Sources, j.sourceInfoPath)
	if err != nil {
		return
	}

	start := time.Now()
	var ci *ExecInfo
	if len(j.args.FlatArgs) == 0 {
		// zero-arg invocation. we'll need to keep this as a special case
		ci, err = j.compileNoArgs()
	} else {
		ci, err = j.compileWithArgs()
	}

	if err != nil {
		return nil, err
	}

	j.log.Info("javac finished in %v", time.Since(start))

	err = MarshalExecInfo(ci, j.compilerInfoPath)
	if err != nil {
		return nil, err
	}

	return ci, nil
}
func (j *jCache) compileWithArgs() (*ExecInfo, error) {
	filename, err := j.writeArgsToTmpFile()
	if err != nil {
		return nil, err
	}
	defer os.Remove(filename) // not needed after javac command has finished

	j.log.Info("%s %s\n", j.args.CompilerPath, "@"+filename)
	return j.compileFunc(j.args.CompilerPath, "@"+filename)
}
func (j *jCache) compileNoArgs() (*ExecInfo, error) {
	j.log.Info("%s\n", j.args.CompilerPath)
	return j.compileFunc(j.args.CompilerPath)
}
func (j *jCache) writeArgsToTmpFile() (filename string, err error) {
	repackedArgs := j.repackArgs()
	j.log.Info("REPACKED ARGS::\n\n%s\n", strings.Join(repackedArgs, "\n"))
	return writeArgsToTmpFile(repackedArgs)
}
func (j *jCache) repackArgs() []string {
	repacked := j.args.FlatArgs
	repacked = redirectArgOption(repacked, "-d", j.classesCachePath, true)
	// adding -h changes the behaviour of javac (v1.8+). We don't want that
	repacked = redirectArgOption(repacked, "-h", j.includeCachePath, false)
	return repacked
}
func (j *jCache) copyCachedFiles() (nFiles int, nBytes int64, err error) {
	const N = 2

	wg := sync.WaitGroup{}
	wg.Add(N)
	f := make([]int, N)
	b := make([]int64, N)
	e := make([]error, N)

	go func() {
		f[0], b[0], e[0] = copyAll(j.classesCachePath, j.args.DstDir)
		wg.Done()
	}()
	go func() {
		f[1], b[1], e[1] = copyAll(j.includeCachePath, j.args.IncDir)
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

func (j *jCache) needCompilation() bool {
	if j.anyFileNotExists(j.cachePath, j.sourceInfoPath, j.compilerInfoPath) {
		return true
	}

	// see if any modified.....
	infoSlice, err := UnmarshalFileInfoSlice(j.sourceInfoPath)
	if err != nil {
		j.log.Info("failed to unmarshal %s - %+v", j.sourceInfoPath, err)
		return true
	}

	for _, info := range infoSlice {
		stat, err := os.Stat(info.Path)
		if err != nil {
			j.log.Info("failed to stat %s - %+v", info.Path, err)
			return true
		}

		tStat := stat.ModTime().UTC()
		tInfo := info.ModTime.UTC()
		if !tStat.Equal(tInfo) {
			j.log.Info("modified time mismatch %s\n"+
				"modified: %v\n"+
				"cached:   %v",
				info.Path, tStat, tInfo)

			hash, err := Sha256File(info.Path)
			if err != nil {
				j.log.Info("failed to sha256 sum %s - %+v", info.Path, err)
				return true
			}

			if hash == info.Sha256 {
				j.log.Info("found identical digest for %s.", info.Path)
				return false
			}

			return true
		}
	}

	return false
}
func (j *jCache) anyFileNotExists(filenames ...string) bool {
	anyNotExists := false
	for _, filename := range filenames {
		if DoesNotExist(filename) {
			j.log.Info("%s does not exist", filename)
			anyNotExists = true
		}
	}
	return anyNotExists
}

func (j *jCache) mkDirs() error {
	err := os.MkdirAll(j.classesCachePath, os.ModePerm)
	if err != nil {
		return errors.WithStack(err)
	}
	err = os.MkdirAll(j.includeCachePath, os.ModePerm)
	if err != nil {
		return errors.WithStack(err)
	}

	if j.args.DstDir != "" {
		err = os.MkdirAll(j.args.DstDir, os.ModePerm)
		if err != nil {
			return errors.WithStack(err)
		}
	}
	if j.args.IncDir != "" {
		err = os.MkdirAll(j.args.IncDir, os.ModePerm)
		if err != nil {
			return errors.WithStack(err)
		}
	}
	if j.args.GenDir != "" {
		err = os.MkdirAll(j.args.GenDir, os.ModePerm)
		if err != nil {
			return errors.WithStack(err)
		}
	}

	return nil
}
