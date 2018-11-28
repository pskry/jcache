package main

import (
	"bytes"
	"fmt"
	"github.com/baeda/jcache/internal/app/jcache"
	"io/ioutil"
	"os"
	"os/exec"
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
		func(javac, jcache error) bool {
			return javac == nil && jcache == nil
		},
		nil,
	)
}

func TestMissingFinalBrace(t *testing.T) {
	systemTest(t,
		"jcache/MissingFinalBrace",
		func(stdout string) (string, bool) {
			return "must be empty", stdout == ""
		},
		func(stderr string) (string, bool) {
			return "must contain error location",
				strings.Contains(stderr, "MissingFinalBrace.java:3: error:")
		},
		func(exit int) (string, bool) {
			return "must not be 0", exit != 0
		},
		func(javac, jcache error) bool {
			return javac.Error() == jcache.Error() // TODO weak. I know oh well
		},
		nil,
	)
}

func TestRawType(t *testing.T) {
	systemTest(t,
		"jcache/RawType",
		func(stdout string) (string, bool) {
			return "must be empty", stdout == ""
		},
		func(stderr string) (string, bool) {
			return "must contain warnings",
				strings.Contains(stderr, "RawType.java:9: warning: [rawtypes]") &&
					strings.Contains(stderr, "RawType.java:10: warning: [unchecked]")
		},
		func(exit int) (string, bool) {
			return "must be 0", exit == 0
		},
		func(javac, jcache error) bool {
			return javac == nil && jcache == nil
		},
		nil,
	)
}

func TestJni(t *testing.T) {
	systemTest(t,
		"jni/Jni",
		func(stdout string) (string, bool) {
			return "must be empty", stdout == ""
		},
		func(stderr string) (string, bool) {
			return "must be empty", stderr == ""
		},
		func(exit int) (string, bool) {
			return "must be 0", exit == 0
		},
		func(javac, jcache error) bool {
			return javac == nil && jcache == nil
		},
		func(javac, jcache error) bool {
			return javac == nil && jcache == nil
		},
	)
}

func systemTest(t *testing.T, fqcn string, pStdout, pStderr func(string) (string, bool), pExit func(int) (string, bool), readErrCmp func(error, error) bool, incErrCmp func(error, error) bool) {
	tmpDir, err := ioutil.TempDir("", "jcache_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cacheDir := filepath.Join(tmpDir, "cache")
	outDir := filepath.Join(tmpDir, "out")
	incDir := filepath.Join(tmpDir, "include")

	compileCalled := false

	// first run. must call compile
	jc, err := jcache.NewCache(
		cacheDir,
		func(name string, args ...string) (info *jcache.ExecInfo, err error) {
			compileCalled = true
			return jcache.Command(name, args...)
		},
		jcache.NewLogger(os.Stdout),
		asSlice(findJavac(),
			"-Xlint:all",
			"-d", outDir,
			"-h", incDir,
			"../../test/testdata/java/"+fqcn+".java"),
	)
	panicOnErr(err)
	info, err := jc.Execute()
	panicOnErr(err)

	desc, ok := pStdout(info.Stdout)
	if !ok {
		t.Fatalf("pStdout <%v> failed. stdout=%v", desc, info.Stdout)
	}
	desc, ok = pStderr(info.Stderr)
	if !ok {
		t.Fatalf("pStderr <%v> failed. stderr=%v", desc, info.Stderr)
	}
	desc, ok = pExit(info.Exit)
	if !ok {
		t.Fatalf("pExit <%v> failed. exit=%d", desc, info.Exit)
	}

	if !compileCalled {
		t.Fatalf("compiler not called.")
	}

	// grab output
	data0, readErr0 := ioutil.ReadFile(filepath.Join(outDir, fqcn+".class"))
	incDat0, incErr0 := ioutil.ReadFile(filepath.Join(incDir, strings.Replace(fqcn, "/", "_", -1)+".h"))

	// second run. must not compile
	// delete all output
	os.RemoveAll(outDir)

	jc, err = jcache.NewCache(
		cacheDir,
		func(name string, args ...string) (info *jcache.ExecInfo, err error) {
			panic(fmt.Sprintf("compile called! %s(%v)", name, args))
		},
		jcache.NewLogger(os.Stdout),
		asSlice(findJavac(),
			"-Xlint:all",
			"-d", outDir,
			"-h", incDir,
			"../../test/testdata/java/"+fqcn+".java"),
	)
	panicOnErr(err)
	info, err = jc.Execute()
	panicOnErr(err)

	desc, ok = pStdout(info.Stdout)
	if !ok {
		t.Fatalf("pStdout <%v> failed. stdout=%v", desc, info.Stdout)
	}
	desc, ok = pStderr(info.Stderr)
	if !ok {
		t.Fatalf("pStderr <%v> failed. stderr=%v", desc, info.Stderr)
	}
	desc, ok = pExit(info.Exit)
	if !ok {
		t.Fatalf("pExit <%v> failed. exit=%d", desc, info.Exit)
	}

	data1, readErr1 := ioutil.ReadFile(filepath.Join(outDir, fqcn+".class"))
	incDat1, incErr1 := ioutil.ReadFile(filepath.Join(incDir, strings.Replace(fqcn, "/", "_", -1)+".h"))

	// both jcache invocations must produce the EXACT same output file.
	if !bytes.Equal(data0, data1) {
		t.Fatalf("The two output files are not identical!")
	}

	if !bytes.Equal(incDat0, incDat1) {
		t.Fatalf("The two header files are not identical!")
	}

	if readErrCmp != nil && !readErrCmp(readErr0, readErr1) {
		t.Fatalf("Read errors: ... javac:\n%#v\n\njcache:\n%#v", readErr0, readErr1)
	}

	if incErrCmp != nil && !incErrCmp(incErr0, incErr1) {
		t.Fatalf("Inc errors: ... javac:\n%#v\n\njcache:\n%#v", readErr0, readErr1)
	}
}

func panicOnErr(err error) {
	if err != nil {
		panic(err)
	}
}

func asSlice(args ...string) []string {
	return args
}

func findJavac() string {
	path, err := exec.LookPath("javac")
	panicOnErr(err)
	return path
}
