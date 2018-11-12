package jcache

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const MinArgs int = 2

type (
	parser struct {
		compilerPath string
		originalArgs []string
		flatArgs     []string
		sourcePaths  []string
		sources      []string
		dstDir       string
		uuid         string
		parsed       bool
	}
	ParsedArgs struct {
		CompilerPath string
		OriginalArgs []string
		FlatArgs     []string
		SourcePaths  []string
		Sources      []string
		DstDir       string
		UUID         string
	}
)

func ParseArgs(osArgs []string) (ParsedArgs, error) {
	p := parser{}
	if !p.parsed {
		if err := p.parse(osArgs); err != nil {
			return ParsedArgs{}, err
		}
		p.parsed = true
	}

	pa := ParsedArgs{
		CompilerPath: p.compilerPath,
		OriginalArgs: p.originalArgs,
		FlatArgs:     p.flatArgs,
		SourcePaths:  p.sourcePaths,
		Sources:      p.sources,
		DstDir:       p.dstDir,
		UUID:         p.uuid,
	}

	return pa, nil
}

func (p *parser) parse(osArgs []string) error {
	if len(osArgs) < MinArgs {
		return fmt.Errorf("invalid argument length")
	}

	if err := p.parseCompilerPath(osArgs); err != nil {
		return err
	}

	p.parseCompilerArgs(osArgs)
	if err := p.flattenArgs(); err != nil {
		return err
	}

	p.findSourcePaths()
	p.findSourceFiles()
	p.findDstDir()
	if err := p.computeUUID(); err != nil {
		return err
	}

	return nil
}

func (p *parser) parseCompilerPath(osArgs []string) error {
	cp := osArgs[1]
	if _, err := os.Stat(cp); err != nil {
		return err
	}

	cp, err := filepath.EvalSymlinks(cp)
	if err != nil {
		return err
	}

	p.compilerPath = cp
	return nil
}

func (p *parser) parseCompilerArgs(osArgs []string) {
	args := make([]string, len(osArgs)-MinArgs)
	for i := MinArgs; i < len(osArgs); i++ {
		args[i-MinArgs] = cleanArg(osArgs[i])
	}

	p.originalArgs = args
}

func (p *parser) flattenArgs() error {
	var flatArgs []string
	for _, arg := range p.originalArgs {
		arg = strings.TrimSpace(arg)
		if len(arg) == 0 {
			// Shouldn't be possible, since we're parsing
			// command-line arguments here, but just to be sure.
			continue
		}

		if arg[0] == '@' {
			// Reference to an argument file to javac.
			// We'll read the contents and flatten it.
			filePath := arg[1:]
			data, err := ioutil.ReadFile(filePath)
			if err != nil {
				return err
			}

			for _, field := range strings.Fields(string(data)) {
				flatArgs = append(flatArgs, cleanArg(field))
			}
		} else {
			flatArgs = append(flatArgs, arg)
		}
	}

	p.flatArgs = flatArgs
	return nil
}

func (p *parser) findSourcePaths() {
	var sp string
	for i, arg := range p.flatArgs {
		if arg == "--source-path" || arg == "-sourcepath" {
			sp = p.flatArgs[i+1]
			break
		}
	}
	p.sourcePaths = remEmptyStrings(strings.Split(sp, ":"))
}

func (p *parser) findSourceFiles() {
	var sources []string
	for _, arg := range p.flatArgs {
		if arg == "" || arg[0] == '-' {
			continue
		}

		fs, err := os.Stat(arg)
		if err != nil || fs.IsDir() {
			continue
		}

		sources = append(sources, arg)
	}

	p.sources = sources
}

func (p *parser) findDstDir() {
	for i, arg := range p.flatArgs {
		if arg == "-d" {
			p.dstDir = p.flatArgs[i+1]
			break
		}
	}
}

func (p *parser) computeUUID() error {
	ci, err := os.Stat(p.compilerPath)
	if err != nil {
		return err
	}

	hash := sha256.New()
	compilerModTime := ci.ModTime().Format(time.RFC3339)
	hash.Write([]byte(compilerModTime))

	for _, arg := range p.flatArgs {
		hash.Write([]byte(arg))
	}
	sumSlice := hash.Sum(nil)
	p.uuid = hex.EncodeToString(sumSlice)
	return nil
}

func cleanArg(arg string) string {
	return strings.TrimSpace(strings.Replace(arg, "\"", "", -1))
}

func remEmptyStrings(strings []string) []string {
	var res []string
	for _, arg := range strings {
		if arg != "" {
			res = append(res, arg)
		}
	}

	return res
}
