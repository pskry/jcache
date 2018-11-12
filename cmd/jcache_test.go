package main

import (
	"bytes"
	"fmt"
	"github.com/baeda/jcache/internal/app/jcache"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEmptyTopLevelClass(t *testing.T) {
	systemTest(t,
		"jcache/EmptyTopLevelClass",
		func(stdout string) (string, bool) {
			return "must be empty", stdout == ""
		},
		func(stderr string) (string, bool) {
			return "must be empty", stderr == ""
		},
		func(exit int) (string, bool) {
			return "must be 0", exit == 0
		},
	)
}

func TestMissingFinalBrace(t *testing.T) {
	systemTest(t,
		"jcache/MissingFinalBrace",
		func(stdout string) (string, bool) {
			return "must be empty", stdout == ""
		},
		func(stderr string) (string, bool) {
			return "must contain error location", strings.Contains(stderr, "MissingFinalBrace.java:3: error:")
		},
		func(exit int) (string, bool) {
			return "must not be 0", exit != 0
		},
	)
}

func TestRawType(t *testing.T) {
	systemTest(t,
		"jcache/RawType",
		func(stdout string) (string, bool) {
			return "must be empty", stdout == ""
		},
		func(stderr string) (string, bool) {
			return "must be empty",
				strings.Contains(stderr, "RawType.java:9: warning: [rawtypes]") &&
					strings.Contains(stderr, "RawType.java:10: warning: [unchecked]")
		},
		func(exit int) (string, bool) {
			return "must be 0", exit == 0
		},
	)
}

func systemTest(t *testing.T, fqcn string, pStdout, pStderr func(string) (string, bool), pExit func(int) (string, bool)) {
	tmpDir, err := ioutil.TempDir("", "jcache_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cacheDir := filepath.Join(tmpDir, "cache")
	outDir := filepath.Join(tmpDir, "out")

	compileCalled := false

	// first run. must call compile
	stdout, stderr, exit := mainExitCode(
		cacheDir,
		func(name string, args ...string) (fOut string, fErr string, exit int, err error) {
			compileCalled = true
			return jcache.Command(name, args...)
		},
		"_$self",
		"/usr/bin/javac",
		"-Xlint:all",
		"-d", outDir,
		"../test/testdata/java/"+fqcn+".java",
	)

	desc, ok := pStdout(stdout)
	if !ok {
		t.Fatalf("pStdout <%v> failed. stdout=%v", desc, stdout)
	}
	desc, ok = pStderr(stderr)
	if !ok {
		t.Fatalf("pStderr <%v> failed. stderr=%v", desc, stderr)
	}
	desc, ok = pExit(exit)
	if !ok {
		t.Fatalf("pExit <%v> failed. exit=%d", desc, exit)
	}

	if !compileCalled {
		t.Fatalf("compiler not called.")
	}

	// grab output
	data0, readErr0 := ioutil.ReadFile(filepath.Join(outDir, fqcn+".class"))

	// second run. must not compile
	// delete all output
	os.RemoveAll(outDir)

	stdout, stderr, exit = mainExitCode(
		cacheDir,
		func(name string, args ...string) (fOut string, fErr string, exit int, err error) {
			panic(fmt.Sprintf("compile called! %s(%v)", name, args))
		},
		"_$self",
		"/usr/bin/javac",
		"-Xlint:all",
		"-d", outDir,
		"../test/testdata/java/"+fqcn+".java",
	)

	desc, ok = pStdout(stdout)
	if !ok {
		t.Fatalf("pStdout <%v> failed. stdout=%v", desc, stdout)
	}
	desc, ok = pStderr(stderr)
	if !ok {
		t.Fatalf("pStderr <%v> failed. stderr=%v", desc, stderr)
	}
	desc, ok = pExit(exit)
	if !ok {
		t.Fatalf("pExit <%v> failed. exit=%d", desc, exit)
	}

	data1, readErr1 := ioutil.ReadFile(filepath.Join(outDir, fqcn+".class"))

	// both jcache invocations must produce the EXACT same output file.
	if readErr0 != nil && readErr1 != nil && readErr0.Error() != readErr1.Error() {
		t.Fatalf("The two file-read errors are not identical!")
	}

	if data0 != nil && data1 != nil && !bytes.Equal(data0, data1) {
		t.Fatalf("The two output files are not identical!")
	}
}
