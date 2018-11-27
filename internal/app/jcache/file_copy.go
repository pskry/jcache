package jcache

import (
	"fmt"
	"github.com/karrick/godirwalk"
	"github.com/pkg/errors"
	"io"
	"os"
	"path/filepath"
)

func copyAll(srcPath, dstPath string) (nFiles int, nBytes int64, err error) {
	err = godirwalk.Walk(srcPath, &godirwalk.Options{
		FollowSymbolicLinks: true,
		Unsorted:            true,
		Callback: func(src string, srcInfo *godirwalk.Dirent) error {
			// abort walking on first error encountered
			if err != nil {
				return errors.WithStack(err)
			}

			if srcInfo.IsDir() {
				return nil
			}

			if !srcInfo.IsRegular() {
				return fmt.Errorf("file not regular: %s", src)
			}

			// construct fully qualified class path (JVM style)
			fqcp, err := filepath.Rel(srcPath, src)
			if err != nil {
				return errors.WithStack(err)
			}

			dst := filepath.Join(dstPath, fqcp)
			if err != nil {
				return errors.WithStack(err)
			}

			err = os.MkdirAll(filepath.Dir(dst), os.ModePerm)
			if err != nil {
				return errors.WithStack(err)
			}

			w, err := copyFile(src, dst)
			if err != nil {
				return errors.WithStack(err)
			}

			nBytes += w
			nFiles++

			return nil
		},
	})

	return
}

func copyFile(from, to string) (int64, error) {
	src, err := os.Open(from)
	if err != nil {
		return 0, err
	}
	defer src.Close()

	dst, err := os.Create(to)
	if err != nil {
		return 0, err
	}
	defer dst.Close()

	return io.Copy(dst, src)
}
