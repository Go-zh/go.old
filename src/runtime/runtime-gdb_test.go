package runtime_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"
)

func checkGdbPython(t *testing.T) {
	cmd := exec.Command("gdb", "-nx", "-q", "--batch", "-iex", "python import sys; print('go gdb python support')")
	out, err := cmd.CombinedOutput()

	if err != nil {
		t.Skipf("skipping due to issue running gdb: %v", err)
	}
	if string(out) != "go gdb python support\n" {
		t.Skipf("skipping due to lack of python gdb support: %s", out)
	}
}

const helloSource = `
package main
import "fmt"
func main() {
	mapvar := make(map[string]string,5)
	mapvar["abc"] = "def"
	mapvar["ghi"] = "jkl"
	strvar := "abc"
	ptrvar := &strvar
	fmt.Println("hi") // line 10
	_ = ptrvar
}
`

func TestGdbPython(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("gdb does not work on darwin")
	}

	checkGdbPython(t)

	dir, err := ioutil.TempDir("", "go-build")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(dir)

	src := filepath.Join(dir, "main.go")
	err = ioutil.WriteFile(src, []byte(helloSource), 0644)
	if err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	cmd := exec.Command("go", "build", "-o", "a.exe")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("building source %v\n%s", err, out)
	}

	args := []string{"-nx", "-q", "--batch", "-iex",
		fmt.Sprintf("add-auto-load-safe-path %s/src/runtime", runtime.GOROOT()),
		"-ex", "br main.go:10",
		"-ex", "run",
		"-ex", "echo BEGIN info goroutines\n",
		"-ex", "info goroutines",
		"-ex", "echo END\n",
		"-ex", "echo BEGIN print mapvar\n",
		"-ex", "print mapvar",
		"-ex", "echo END\n",
		"-ex", "echo BEGIN print strvar\n",
		"-ex", "print strvar",
		"-ex", "echo END\n",
		"-ex", "echo BEGIN print ptrvar\n",
		"-ex", "print ptrvar",
		"-ex", "echo END\n"}

	// without framepointer, gdb cannot backtrace our non-standard
	// stack frames on RISC architectures.
	canBackTrace := false
	switch runtime.GOARCH {
	case "amd64", "386", "ppc64", "ppc64le", "arm", "arm64":
		canBackTrace = true
		args = append(args,
			"-ex", "echo BEGIN goroutine 2 bt\n",
			"-ex", "goroutine 2 bt",
			"-ex", "echo END\n")
	}

	args = append(args, filepath.Join(dir, "a.exe"))
	got, _ := exec.Command("gdb", args...).CombinedOutput()

	firstLine := bytes.SplitN(got, []byte("\n"), 2)[0]
	if string(firstLine) != "Loading Go Runtime support." {
		t.Fatalf("failed to load Go runtime support: %s", firstLine)
	}

	// Extract named BEGIN...END blocks from output
	partRe := regexp.MustCompile(`(?ms)^BEGIN ([^\n]*)\n(.*?)\nEND`)
	blocks := map[string]string{}
	for _, subs := range partRe.FindAllSubmatch(got, -1) {
		blocks[string(subs[1])] = string(subs[2])
	}

	infoGoroutinesRe := regexp.MustCompile(`\*\s+\d+\s+running\s+`)
	if bl := blocks["info goroutines"]; !infoGoroutinesRe.MatchString(bl) {
		t.Fatalf("info goroutines failed: %s", bl)
	}

	printMapvarRe := regexp.MustCompile(`\Q = map[string]string = {["abc"] = "def", ["ghi"] = "jkl"}\E$`)
	if bl := blocks["print mapvar"]; !printMapvarRe.MatchString(bl) {
		t.Fatalf("print mapvar failed: %s", bl)
	}

	strVarRe := regexp.MustCompile(`\Q = "abc"\E$`)
	if bl := blocks["print strvar"]; !strVarRe.MatchString(bl) {
		t.Fatalf("print strvar failed: %s", bl)
	}

	if bl := blocks["print ptrvar"]; !strVarRe.MatchString(bl) {
		t.Fatalf("print ptrvar failed: %s", bl)
	}

	btGoroutineRe := regexp.MustCompile(`^#0\s+runtime.+at`)
	if bl := blocks["goroutine 2 bt"]; canBackTrace && !btGoroutineRe.MatchString(bl) {
		t.Fatalf("goroutine 2 bt failed: %s", bl)
	} else if !canBackTrace {
		t.Logf("gdb cannot backtrace for GOARCH=%s, skipped goroutine backtrace test", runtime.GOARCH)
	}
}
