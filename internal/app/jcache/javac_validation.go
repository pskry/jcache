package jcache

import (
	"github.com/pkg/errors"
	"strings"
)

type (
	ErrInvalidCompiler struct {
		error
		Path        string
		CombinedOut string
	}
)

func validateCompiler(compilerPath string) error {
	info, err := Command(compilerPath, "-version")
	if err != nil {
		return errors.WithStack(err)
	}

	combined := info.Combined()
	if !strings.HasPrefix(combined, "javac") {
		return ErrInvalidCompiler{
			error:       errors.New("unexpected output"),
			Path:        compilerPath,
			CombinedOut: combined,
		}
	}

	return nil
}
