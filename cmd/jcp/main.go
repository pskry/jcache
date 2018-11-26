package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
)

func main() {
	pwd, err := os.Getwd()
	if err != nil {
		log.Fatalln(err)
	}

	srcPath, err := filepath.Abs(os.Args[1])
	if err != nil {
		log.Fatalln(err)
	}

	copyAll(srcPath, filepath.Join(pwd, "jcp"))
}

func copyAll(srcPath, dstPath string) (nFiles int, nBytes int64, err error) {
	err = filepath.Walk(
		srcPath,
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

			// construct fully qualified class path (JVM style)
			fqcp, err := filepath.Rel(srcPath, src)
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

			w, err := copyFile(src, dst)
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
