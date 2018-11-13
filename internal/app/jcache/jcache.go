package jcache

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
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
		dstCachePath        string
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
	dstCachePath := filepath.Join(cachePath, "classes")

	err = os.MkdirAll(dstCachePath, os.ModePerm)
	if err != nil {
		return nil, err
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
		dstCachePath:        dstCachePath,
		log:                 logger,
	}

	return jc, nil
}
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
func (j *jCache) Execute() (string, string, int, error) {
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

		j.log.Info("javac execution-time: %v", time.Since(javacStart))

		var destinations []string
		err = filepath.Walk(j.dstCachePath, func(path string, info os.FileInfo, err error) error {
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
			err = j.lazyLoadDstHashes()
			if err != nil {
				return err
			}

			if _, err := os.Stat(dst); err == nil {
				if srcHash, ok := j.dstHashes[src]; ok {
					if dstHash, err := Sha256File(dst); err == nil {
						if srcHash == dstHash {
							j.log.Info("skipping %s", src)
							return nil
						}
					}
				}
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
