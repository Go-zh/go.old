// Copyright 2012 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Initialization for any invocation.

// The usual variables.
var (
	goarch           string
	gobin            string
	gohostarch       string
	gohostchar       string
	gohostos         string
	goos             string
	goarm            string
	go386            string
	goroot           string
	goroot_final     string
	goextlinkenabled string
	workdir          string
	tooldir          string
	gochar           string
	oldgoos          string
	oldgoarch        string
	oldgochar        string
	slash            string
	exe              string
	defaultcc        string
	defaultcflags    string
	defaultldflags   string
	defaultcxxtarget string
	defaultcctarget  string
	rebuildall       bool
	defaultclang     bool

	sflag bool // build static binaries
	vflag int  // verbosity
)

// The known architecture letters.
var gochars = "5667899"

// The known architectures.
var okgoarch = []string{
	// same order as gochars
	"arm",
	"amd64",
	"amd64p32",
	"arm64",
	"386",
	"ppc64",
	"ppc64le",
}

// The known operating systems.
var okgoos = []string{
	"darwin",
	"dragonfly",
	"linux",
	"android",
	"solaris",
	"freebsd",
	"nacl",
	"netbsd",
	"openbsd",
	"plan9",
	"windows",
}

// find reports the first index of p in l[0:n], or else -1.
func find(p string, l []string) int {
	for i, s := range l {
		if p == s {
			return i
		}
	}
	return -1
}

// xinit handles initialization of the various global state, like goroot and goarch.
func xinit() {
	goroot = os.Getenv("GOROOT")
	if slash == "/" && len(goroot) > 1 || slash == `\` && len(goroot) > 3 {
		// if not "/" or "c:\", then strip trailing path separator
		goroot = strings.TrimSuffix(goroot, slash)
	}
	if goroot == "" {
		fatal("$GOROOT must be set")
	}

	goroot_final = os.Getenv("GOROOT_FINAL")
	if goroot_final == "" {
		goroot_final = goroot
	}

	b := os.Getenv("GOBIN")
	if b == "" {
		b = goroot + slash + "bin"
	}
	gobin = b

	b = os.Getenv("GOOS")
	if b == "" {
		b = gohostos
	}
	goos = b
	if find(goos, okgoos) < 0 {
		fatal("unknown $GOOS %s", goos)
	}

	b = os.Getenv("GOARM")
	if b == "" {
		b = xgetgoarm()
	}
	goarm = b

	b = os.Getenv("GO386")
	if b == "" {
		if cansse2() {
			b = "sse2"
		} else {
			b = "387"
		}
	}
	go386 = b

	p := pathf("%s/src/all.bash", goroot)
	if !isfile(p) {
		fatal("$GOROOT is not set correctly or not exported\n"+
			"\tGOROOT=%s\n"+
			"\t%s does not exist", goroot, p)
	}

	b = os.Getenv("GOHOSTARCH")
	if b != "" {
		gohostarch = b
	}

	i := find(gohostarch, okgoarch)
	if i < 0 {
		fatal("unknown $GOHOSTARCH %s", gohostarch)
	}
	gohostchar = gochars[i : i+1]

	b = os.Getenv("GOARCH")
	if b == "" {
		b = gohostarch
	}
	goarch = b
	i = find(goarch, okgoarch)
	if i < 0 {
		fatal("unknown $GOARCH %s", goarch)
	}
	gochar = gochars[i : i+1]

	b = os.Getenv("GO_EXTLINK_ENABLED")
	if b != "" {
		if b != "0" && b != "1" {
			fatal("unknown $GO_EXTLINK_ENABLED %s", b)
		}
		goextlinkenabled = b
	}

	b = os.Getenv("CC")
	if b == "" {
		// Use clang on OS X, because gcc is deprecated there.
		// Xcode for OS X 10.9 Mavericks will ship a fake "gcc" binary that
		// actually runs clang. We prepare different command
		// lines for the two binaries, so it matters what we call it.
		// See golang.org/issue/5822.
		if defaultclang {
			b = "clang"
		} else {
			b = "gcc"
		}
	}
	defaultcc = b

	defaultcflags = os.Getenv("CFLAGS")

	defaultldflags = os.Getenv("LDFLAGS")

	b = os.Getenv("CC_FOR_TARGET")
	if b == "" {
		b = defaultcc
	}
	defaultcctarget = b

	b = os.Getenv("CXX_FOR_TARGET")
	if b == "" {
		b = os.Getenv("CXX")
		if b == "" {
			if defaultclang {
				b = "clang++"
			} else {
				b = "g++"
			}
		}
	}
	defaultcxxtarget = b

	// For tools being invoked but also for os.ExpandEnv.
	os.Setenv("GO386", go386)
	os.Setenv("GOARCH", goarch)
	os.Setenv("GOARM", goarm)
	os.Setenv("GOHOSTARCH", gohostarch)
	os.Setenv("GOHOSTOS", gohostos)
	os.Setenv("GOOS", goos)
	os.Setenv("GOROOT", goroot)
	os.Setenv("GOROOT_FINAL", goroot_final)

	// Make the environment more predictable.
	os.Setenv("LANG", "C")
	os.Setenv("LANGUAGE", "en_US.UTF8")

	workdir = xworkdir()
	xatexit(rmworkdir)

	tooldir = pathf("%s/pkg/tool/%s_%s", goroot, gohostos, gohostarch)
}

// rmworkdir deletes the work directory.
func rmworkdir() {
	if vflag > 1 {
		errprintf("rm -rf %s\n", workdir)
	}
	xremoveall(workdir)
}

// Remove trailing spaces.
func chomp(s string) string {
	return strings.TrimRight(s, " \t\r\n")
}

func branchtag(branch string) (tag string, precise bool) {
	b := run(goroot, CheckExit, "git", "log", "--decorate=full", "--format=format:%d", "master.."+branch)
	tag = branch
	for _, line := range splitlines(b) {
		// Each line is either blank, or looks like
		//	  (tag: refs/tags/go1.4rc2, refs/remotes/origin/release-branch.go1.4, refs/heads/release-branch.go1.4)
		// We need to find an element starting with refs/tags/.
		i := strings.Index(line, " refs/tags/")
		if i < 0 {
			continue
		}
		i += len(" refs/tags/")
		// The tag name ends at a comma or paren (prefer the first).
		j := strings.Index(line[i:], ",")
		if j < 0 {
			j = strings.Index(line[i:], ")")
		}
		if j < 0 {
			continue // malformed line; ignore it
		}
		tag = line[i : i+j]
		if i == 0 {
			precise = true // tag denotes HEAD
		}
		break
	}
	return
}

// findgoversion determines the Go version to use in the version string.
func findgoversion() string {
	// The $GOROOT/VERSION file takes priority, for distributions
	// without the source repo.
	path := pathf("%s/VERSION", goroot)
	if isfile(path) {
		b := chomp(readfile(path))
		// Commands such as "dist version > VERSION" will cause
		// the shell to create an empty VERSION file and set dist's
		// stdout to its fd. dist in turn looks at VERSION and uses
		// its content if available, which is empty at this point.
		// Only use the VERSION file if it is non-empty.
		if b != "" {
			return b
		}
	}

	// The $GOROOT/VERSION.cache file is a cache to avoid invoking
	// git every time we run this command.  Unlike VERSION, it gets
	// deleted by the clean command.
	path = pathf("%s/VERSION.cache", goroot)
	if isfile(path) {
		return chomp(readfile(path))
	}

	// Show a nicer error message if this isn't a Git repo.
	if !isGitRepo() {
		fatal("FAILED: not a Git repo; must put a VERSION file in $GOROOT")
	}

	// Otherwise, use Git.
	// What is the current branch?
	branch := chomp(run(goroot, CheckExit, "git", "rev-parse", "--abbrev-ref", "HEAD"))

	// What are the tags along the current branch?
	tag := "devel"
	precise := false

	// If we're on a release branch, use the closest matching tag
	// that is on the release branch (and not on the master branch).
	if strings.HasPrefix(branch, "release-branch.") {
		tag, precise = branchtag(branch)
	}

	if !precise {
		// Tag does not point at HEAD; add hash and date to version.
		tag += chomp(run(goroot, CheckExit, "git", "log", "-n", "1", "--format=format: +%h %cd", "HEAD"))
	}

	// Cache version.
	writefile(tag, path, 0)

	return tag
}

// isGitRepo reports whether the working directory is inside a Git repository.
func isGitRepo() bool {
	p := ".git"
	for {
		fi, err := os.Stat(p)
		if os.IsNotExist(err) {
			p = filepath.Join("..", p)
			continue
		}
		if err != nil || !fi.IsDir() {
			return false
		}
		return true
	}
}

/*
 * Initial tree setup.
 */

// The old tools that no longer live in $GOBIN or $GOROOT/bin.
var oldtool = []string{
	"5a", "5c", "5g", "5l",
	"6a", "6c", "6g", "6l",
	"8a", "8c", "8g", "8l",
	"9a", "9c", "9g", "9l",
	"6cov",
	"6nm",
	"6prof",
	"cgo",
	"ebnflint",
	"goapi",
	"gofix",
	"goinstall",
	"gomake",
	"gopack",
	"gopprof",
	"gotest",
	"gotype",
	"govet",
	"goyacc",
	"quietgcc",
}

// Unreleased directories (relative to $GOROOT) that should
// not be in release branches.
var unreleased = []string{
	"src/cmd/link",
	"src/cmd/objwriter",
	"src/debug/goobj",
	"src/old",
}

// setup sets up the tree for the initial build.
func setup() {
	// Create bin directory.
	if p := pathf("%s/bin", goroot); !isdir(p) {
		xmkdir(p)
	}

	// Create package directory.
	if p := pathf("%s/pkg", goroot); !isdir(p) {
		xmkdir(p)
	}

	p := pathf("%s/pkg/%s_%s", goroot, gohostos, gohostarch)
	if rebuildall {
		xremoveall(p)
	}
	xmkdirall(p)

	if goos != gohostos || goarch != gohostarch {
		p := pathf("%s/pkg/%s_%s", goroot, goos, goarch)
		if rebuildall {
			xremoveall(p)
		}
		xmkdirall(p)
	}

	// Create object directory.
	// We keep it in pkg/ so that all the generated binaries
	// are in one tree.  If pkg/obj/libgc.a exists, it is a dreg from
	// before we used subdirectories of obj.  Delete all of obj
	// to clean up.
	if p := pathf("%s/pkg/obj/libgc.a", goroot); isfile(p) {
		xremoveall(pathf("%s/pkg/obj", goroot))
	}
	p = pathf("%s/pkg/obj/%s_%s", goroot, gohostos, gohostarch)
	if rebuildall {
		xremoveall(p)
	}
	xmkdirall(p)

	// Create tool directory.
	// We keep it in pkg/, just like the object directory above.
	if rebuildall {
		xremoveall(tooldir)
	}
	xmkdirall(tooldir)

	// Remove tool binaries from before the tool/gohostos_gohostarch
	xremoveall(pathf("%s/bin/tool", goroot))

	// Remove old pre-tool binaries.
	for _, old := range oldtool {
		xremove(pathf("%s/bin/%s", goroot, old))
	}

	// If $GOBIN is set and has a Go compiler, it must be cleaned.
	for _, char := range gochars {
		if isfile(pathf("%s%s%c%s", gobin, slash, char, "g")) {
			for _, old := range oldtool {
				xremove(pathf("%s/%s", gobin, old))
			}
			break
		}
	}

	// For release, make sure excluded things are excluded.
	goversion := findgoversion()
	if strings.HasPrefix(goversion, "release.") || (strings.HasPrefix(goversion, "go") && !strings.Contains(goversion, "beta")) {
		for _, dir := range unreleased {
			if p := pathf("%s/%s", goroot, dir); isdir(p) {
				fatal("%s should not exist in release build", p)
			}
		}
	}
}

/*
 * Tool building
 */

// deptab lists changes to the default dependencies for a given prefix.
// deps ending in /* read the whole directory; deps beginning with -
// exclude files with that prefix.
var deptab = []struct {
	prefix string   // prefix of target
	dep    []string // dependency tweaks for targets with that prefix
}{
	{"cmd/go", []string{
		"zdefaultcc.go",
	}},
	{"runtime", []string{
		"zversion.go",
	}},
}

// depsuffix records the allowed suffixes for source files.
var depsuffix = []string{
	".s",
	".go",
}

// gentab records how to generate some trivial files.
var gentab = []struct {
	nameprefix string
	gen        func(string, string)
}{
	{"zdefaultcc.go", mkzdefaultcc},
	{"zversion.go", mkzversion},

	// not generated anymore, but delete the file if we see it
	{"enam.c", nil},
	{"anames5.c", nil},
	{"anames6.c", nil},
	{"anames8.c", nil},
	{"anames9.c", nil},
}

// install installs the library, package, or binary associated with dir,
// which is relative to $GOROOT/src.
func install(dir string) {
	if vflag > 0 {
		if goos != gohostos || goarch != gohostarch {
			errprintf("%s (%s/%s)\n", dir, goos, goarch)
		} else {
			errprintf("%s\n", dir)
		}
	}

	var clean []string
	defer func() {
		for _, name := range clean {
			xremove(name)
		}
	}()

	// path = full path to dir.
	path := pathf("%s/src/%s", goroot, dir)
	name := filepath.Base(dir)

	ispkg := !strings.HasPrefix(dir, "cmd/") || strings.HasPrefix(dir, "cmd/internal/") || strings.HasPrefix(dir, "cmd/asm/internal/")

	// Start final link command line.
	// Note: code below knows that link.p[targ] is the target.
	var (
		link      []string
		targ      int
		ispackcmd bool
	)
	if ispkg {
		// Go library (package).
		ispackcmd = true
		link = []string{"pack", pathf("%s/pkg/%s_%s/%s.a", goroot, goos, goarch, dir)}
		targ = len(link) - 1
		xmkdirall(filepath.Dir(link[targ]))
	} else {
		// Go command.
		elem := name
		if elem == "go" {
			elem = "go_bootstrap"
		}
		link = []string{fmt.Sprintf("%s/%sl", tooldir, gochar), "-o", pathf("%s/%s%s", tooldir, elem, exe)}
		targ = len(link) - 1
	}
	ttarg := mtime(link[targ])

	// Gather files that are sources for this target.
	// Everything in that directory, and any target-specific
	// additions.
	files := xreaddir(path)

	// Remove files beginning with . or _,
	// which are likely to be editor temporary files.
	// This is the same heuristic build.ScanDir uses.
	// There do exist real C files beginning with _,
	// so limit that check to just Go files.
	files = filter(files, func(p string) bool {
		return !strings.HasPrefix(p, ".") && (!strings.HasPrefix(p, "_") || !strings.HasSuffix(p, ".go"))
	})

	for _, dt := range deptab {
		if dir == dt.prefix || strings.HasSuffix(dt.prefix, "/") && strings.HasPrefix(dir, dt.prefix) {
			for _, p := range dt.dep {
				p = os.ExpandEnv(p)
				files = append(files, p)
			}
		}
	}
	files = uniq(files)

	// Convert to absolute paths.
	for i, p := range files {
		if !isabs(p) {
			files[i] = pathf("%s/%s", path, p)
		}
	}

	// Is the target up-to-date?
	var gofiles, missing []string
	stale := rebuildall
	files = filter(files, func(p string) bool {
		for _, suf := range depsuffix {
			if strings.HasSuffix(p, suf) {
				goto ok
			}
		}
		return false
	ok:
		t := mtime(p)
		if !t.IsZero() && !strings.HasSuffix(p, ".a") && !shouldbuild(p, dir) {
			return false
		}
		if strings.HasSuffix(p, ".go") {
			gofiles = append(gofiles, p)
		}
		if t.After(ttarg) {
			stale = true
		}
		if t.IsZero() {
			missing = append(missing, p)
		}
		return true
	})

	// If there are no files to compile, we're done.
	if len(files) == 0 {
		return
	}

	if !stale {
		return
	}

	// For package runtime, copy some files into the work space.
	if dir == "runtime" {
		xmkdirall(pathf("%s/pkg/include", goroot))
		// For use by assembly and C files.
		copyfile(pathf("%s/pkg/include/textflag.h", goroot),
			pathf("%s/src/runtime/textflag.h", goroot), 0)
		copyfile(pathf("%s/pkg/include/funcdata.h", goroot),
			pathf("%s/src/runtime/funcdata.h", goroot), 0)
	}

	// Generate any missing files; regenerate existing ones.
	for _, p := range files {
		elem := filepath.Base(p)
		for _, gt := range gentab {
			if gt.gen == nil {
				continue
			}
			if strings.HasPrefix(elem, gt.nameprefix) {
				if vflag > 1 {
					errprintf("generate %s\n", p)
				}
				gt.gen(path, p)
				// Do not add generated file to clean list.
				// In runtime, we want to be able to
				// build the package with the go tool,
				// and it assumes these generated files already
				// exist (it does not know how to build them).
				// The 'clean' command can remove
				// the generated files.
				goto built
			}
		}
		// Did not rebuild p.
		if find(p, missing) >= 0 {
			fatal("missing file %s", p)
		}
	built:
	}

	if goos != gohostos || goarch != gohostarch {
		// We've generated the right files; the go command can do the build.
		if vflag > 1 {
			errprintf("skip build for cross-compile %s\n", dir)
		}
		return
	}

	var archive string
	// The next loop will compile individual non-Go files.
	// Hand the Go files to the compiler en masse.
	// For package runtime, this writes go_asm.h, which
	// the assembly files will need.
	pkg := dir
	if strings.HasPrefix(dir, "cmd/") {
		pkg = "main"
	}
	b := pathf("%s/_go_.a", workdir)
	clean = append(clean, b)
	if !ispackcmd {
		link = append(link, b)
	} else {
		archive = b
	}
	compile := []string{pathf("%s/%sg", tooldir, gochar), "-pack", "-o", b, "-p", pkg}
	if dir == "runtime" {
		compile = append(compile, "-+", "-asmhdr", pathf("%s/go_asm.h", workdir))
	}
	compile = append(compile, gofiles...)
	run(path, CheckExit|ShowOutput, compile...)

	// Compile the files.
	for _, p := range files {
		if !strings.HasSuffix(p, ".s") {
			continue
		}

		var compile []string
		// Assembly file for a Go package.
		compile = []string{
			pathf("%s/asm", tooldir),
			"-I", workdir,
			"-I", pathf("%s/pkg/include", goroot),
			"-D", "GOOS_" + goos,
			"-D", "GOARCH_" + goarch,
			"-D", "GOOS_GOARCH_" + goos + "_" + goarch,
		}

		doclean := true
		b := pathf("%s/%s", workdir, filepath.Base(p))

		// Change the last character of the output file (which was c or s).
		if gohostos == "plan9" {
			b = b[:len(b)-1] + gohostchar
		} else {
			b = b[:len(b)-1] + "o"
		}
		compile = append(compile, "-o", b, p)
		bgrun(path, compile...)

		link = append(link, b)
		if doclean {
			clean = append(clean, b)
		}
	}
	bgwait()

	if ispackcmd {
		xremove(link[targ])
		dopack(link[targ], archive, link[targ+1:])
		return
	}

	// Remove target before writing it.
	xremove(link[targ])
	run("", CheckExit|ShowOutput, link...)
}

// matchfield reports whether the field matches this build.
func matchfield(f string) bool {
	for _, tag := range strings.Split(f, ",") {
		if tag == goos || tag == goarch || tag == "cmd_go_bootstrap" || tag == "go1.1" || (goos == "android" && tag == "linux") {
			continue
		}
		return false
	}
	return true
}

// shouldbuild reports whether we should build this file.
// It applies the same rules that are used with context tags
// in package go/build, except that the GOOS and GOARCH
// can appear anywhere in the file name, not just after _.
// In particular, they can be the entire file name (like windows.c).
// We also allow the special tag cmd_go_bootstrap.
// See ../go/bootstrap.go and package go/build.
func shouldbuild(file, dir string) bool {
	// Check file name for GOOS or GOARCH.
	name := filepath.Base(file)
	excluded := func(list []string, ok string) bool {
		for _, x := range list {
			if x == ok {
				continue
			}
			i := strings.Index(name, x)
			if i < 0 {
				continue
			}
			i += len(x)
			if i == len(name) || name[i] == '.' || name[i] == '_' {
				return true
			}
		}
		return false
	}
	if excluded(okgoos, goos) || excluded(okgoarch, goarch) {
		return false
	}

	// Omit test files.
	if strings.Contains(name, "_test") {
		return false
	}

	// Check file contents for // +build lines.
	for _, p := range splitlines(readfile(file)) {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if strings.Contains(p, "package documentation") {
			return false
		}
		if strings.Contains(p, "package main") && dir != "cmd/go" && dir != "cmd/cgo" {
			return false
		}
		if !strings.HasPrefix(p, "//") {
			break
		}
		if !strings.Contains(p, "+build") {
			continue
		}
		fields := splitfields(p)
		if len(fields) < 2 || fields[1] != "+build" {
			continue
		}
		for _, p := range fields[2:] {
			if (p[0] == '!' && !matchfield(p[1:])) || matchfield(p) {
				goto fieldmatch
			}
		}
		return false
	fieldmatch:
	}

	return true
}

// copy copies the file src to dst, via memory (so only good for small files).
func copyfile(dst, src string, exec int) {
	if vflag > 1 {
		errprintf("cp %s %s\n", src, dst)
	}
	writefile(readfile(src), dst, exec)
}

// dopack copies the package src to dst,
// appending the files listed in extra.
// The archive format is the traditional Unix ar format.
func dopack(dst, src string, extra []string) {
	bdst := bytes.NewBufferString(readfile(src))
	for _, file := range extra {
		b := readfile(file)
		// find last path element for archive member name
		i := strings.LastIndex(file, "/") + 1
		j := strings.LastIndex(file, `\`) + 1
		if i < j {
			i = j
		}
		fmt.Fprintf(bdst, "%-16.16s%-12d%-6d%-6d%-8o%-10d`\n", file[i:], 0, 0, 0, 0644, len(b))
		bdst.WriteString(b)
		if len(b)&1 != 0 {
			bdst.WriteByte(0)
		}
	}
	writefile(bdst.String(), dst, 0)
}

// buildorder records the order of builds for the 'go bootstrap' command.
// The Go packages and commands must be in dependency order,
// maintained by hand, but the order doesn't change often.
var buildorder = []string{
	// Go libraries and programs for bootstrap.
	"runtime",
	"errors",
	"sync/atomic",
	"sync",
	"internal/singleflight",
	"io",
	"unicode",
	"unicode/utf8",
	"unicode/utf16",
	"bytes",
	"math",
	"strings",
	"strconv",
	"bufio",
	"sort",
	"container/heap",
	"encoding/base64",
	"syscall",
	"internal/syscall/windows/registry",
	"time",
	"internal/syscall/windows",
	"os",
	"reflect",
	"fmt",
	"encoding",
	"encoding/binary",
	"encoding/json",
	"flag",
	"path/filepath",
	"path",
	"io/ioutil",
	"log",
	"regexp/syntax",
	"regexp",
	"go/token",
	"go/scanner",
	"go/ast",
	"go/parser",
	"os/exec",
	"os/signal",
	"net/url",
	"text/template/parse",
	"text/template",
	"go/doc",
	"go/build",
	"cmd/go",
}

// cleantab records the directories to clean in 'go clean'.
// It is bigger than the buildorder because we clean all the
// compilers but build only the $GOARCH ones.
var cleantab = []string{
	// Commands and C libraries.
	"cmd/5g",
	"cmd/5l",
	"cmd/6g",
	"cmd/6l",
	"cmd/7g",
	"cmd/7l",
	"cmd/8g",
	"cmd/8l",
	"cmd/9g",
	"cmd/9l",
	"cmd/go",
	"cmd/old5a",
	"cmd/old6a",
	"cmd/old8a",
	"cmd/old9a",

	// Go packages.
	"bufio",
	"bytes",
	"container/heap",
	"encoding",
	"encoding/base64",
	"encoding/json",
	"errors",
	"flag",
	"fmt",
	"go/ast",
	"go/build",
	"go/doc",
	"go/parser",
	"go/scanner",
	"go/token",
	"io",
	"io/ioutil",
	"log",
	"math",
	"net/url",
	"os",
	"os/exec",
	"path",
	"path/filepath",
	"reflect",
	"regexp",
	"regexp/syntax",
	"runtime",
	"sort",
	"strconv",
	"strings",
	"sync",
	"sync/atomic",
	"syscall",
	"text/template",
	"text/template/parse",
	"time",
	"unicode",
	"unicode/utf16",
	"unicode/utf8",
}

var runtimegen = []string{
	"zaexperiment.h",
	"zversion.go",
}

func clean() {
	for _, name := range cleantab {
		path := pathf("%s/src/%s", goroot, name)
		// Remove generated files.
		for _, elem := range xreaddir(path) {
			for _, gt := range gentab {
				if strings.HasPrefix(elem, gt.nameprefix) {
					xremove(pathf("%s/%s", path, elem))
				}
			}
		}
		// Remove generated binary named for directory.
		if strings.HasPrefix(name, "cmd/") {
			xremove(pathf("%s/%s", path, name[4:]))
		}
	}

	// remove runtimegen files.
	path := pathf("%s/src/runtime", goroot)
	for _, elem := range runtimegen {
		xremove(pathf("%s/%s", path, elem))
	}

	if rebuildall {
		// Remove object tree.
		xremoveall(pathf("%s/pkg/obj/%s_%s", goroot, gohostos, gohostarch))

		// Remove installed packages and tools.
		xremoveall(pathf("%s/pkg/%s_%s", goroot, gohostos, gohostarch))
		xremoveall(pathf("%s/pkg/%s_%s", goroot, goos, goarch))
		xremoveall(tooldir)

		// Remove cached version info.
		xremove(pathf("%s/VERSION.cache", goroot))
	}
}

/*
 * command implementations
 */

func usage() {
	xprintf("usage: go tool dist [command]\n" +
		"Commands are:\n" +
		"\n" +
		"banner         print installation banner\n" +
		"bootstrap      rebuild everything\n" +
		"clean          deletes all built files\n" +
		"env [-p]       print environment (-p: include $PATH)\n" +
		"install [dir]  install individual directory\n" +
		"test [-h]      run Go test(s)\n" +
		"version        print Go version\n" +
		"\n" +
		"All commands take -v flags to emit extra information.\n",
	)
	xexit(2)
}

// The env command prints the default environment.
func cmdenv() {
	path := flag.Bool("p", false, "emit updated PATH")
	plan9 := flag.Bool("9", false, "emit plan 9 syntax")
	windows := flag.Bool("w", false, "emit windows syntax")
	xflagparse(0)

	format := "%s=\"%s\"\n"
	switch {
	case *plan9:
		format = "%s='%s'\n"
	case *windows:
		format = "set %s=%s\r\n"
	}

	xprintf(format, "CC", defaultcc)
	xprintf(format, "CC_FOR_TARGET", defaultcctarget)
	xprintf(format, "GOROOT", goroot)
	xprintf(format, "GOBIN", gobin)
	xprintf(format, "GOARCH", goarch)
	xprintf(format, "GOOS", goos)
	xprintf(format, "GOHOSTARCH", gohostarch)
	xprintf(format, "GOHOSTOS", gohostos)
	xprintf(format, "GOTOOLDIR", tooldir)
	xprintf(format, "GOCHAR", gochar)
	if goarch == "arm" {
		xprintf(format, "GOARM", goarm)
	}
	if goarch == "386" {
		xprintf(format, "GO386", go386)
	}

	if *path {
		sep := ":"
		if gohostos == "windows" {
			sep = ";"
		}
		xprintf(format, "PATH", fmt.Sprintf("%s%s%s", gobin, sep, os.Getenv("PATH")))
	}
}

// The bootstrap command runs a build from scratch,
// stopping at having installed the go_bootstrap command.
func cmdbootstrap() {
	flag.BoolVar(&rebuildall, "a", rebuildall, "rebuild all")
	flag.BoolVar(&sflag, "s", sflag, "build static binaries")
	xflagparse(0)

	if isdir(pathf("%s/src/pkg", goroot)) {
		fatal("\n\n"+
			"The Go package sources have moved to $GOROOT/src.\n"+
			"*** %s still exists. ***\n"+
			"It probably contains stale files that may confuse the build.\n"+
			"Please (check what's there and) remove it and try again.\n"+
			"See http://golang.org/s/go14nopkg\n",
			pathf("%s/src/pkg", goroot))
	}

	if rebuildall {
		clean()
	}

	setup()

	bootstrapBuildTools()

	// For the main bootstrap, building for host os/arch.
	oldgoos = goos
	oldgoarch = goarch
	oldgochar = gochar
	goos = gohostos
	goarch = gohostarch
	gochar = gohostchar
	os.Setenv("GOHOSTARCH", gohostarch)
	os.Setenv("GOHOSTOS", gohostos)
	os.Setenv("GOARCH", goarch)
	os.Setenv("GOOS", goos)

	// TODO(rsc): Enable when appropriate.
	// This step is only needed if we believe that the Go compiler built from Go 1.4
	// will produce different object files than the Go compiler built from itself.
	// In the absence of bugs, that should not happen.
	// And if there are bugs, they're more likely in the current development tree
	// than in a standard release like Go 1.4, so don't do this rebuild by default.
	if false {
		xprintf("##### Building Go toolchain using itself.\n")
		for _, pattern := range buildorder {
			if pattern == "cmd/go" {
				break
			}
			dir := pattern
			if strings.Contains(pattern, "%s") {
				dir = fmt.Sprintf(pattern, gohostchar)
			}
			install(dir)
			if oldgochar != gohostchar && strings.Contains(pattern, "%s") {
				install(fmt.Sprintf(pattern, oldgochar))
			}
		}
		xprintf("\n")
	}

	xprintf("##### Building compilers and go_bootstrap for host, %s/%s.\n", gohostos, gohostarch)
	for _, pattern := range buildorder {
		dir := pattern
		if strings.Contains(pattern, "%s") {
			dir = fmt.Sprintf(pattern, gohostchar)
		}
		install(dir)
		if oldgochar != gohostchar && strings.Contains(pattern, "%s") {
			install(fmt.Sprintf(pattern, oldgochar))
		}
	}

	goos = oldgoos
	goarch = oldgoarch
	gochar = oldgochar
	os.Setenv("GOARCH", goarch)
	os.Setenv("GOOS", goos)

	// Build runtime for actual goos/goarch too.
	if goos != gohostos || goarch != gohostarch {
		install("runtime")
	}
}

func defaulttarg() string {
	// xgetwd might return a path with symlinks fully resolved, and if
	// there happens to be symlinks in goroot, then the hasprefix test
	// will never succeed. Instead, we use xrealwd to get a canonical
	// goroot/src before the comparison to avoid this problem.
	pwd := xgetwd()
	src := pathf("%s/src/", goroot)
	real_src := xrealwd(src)
	if !strings.HasPrefix(pwd, real_src) {
		fatal("current directory %s is not under %s", pwd, real_src)
	}
	pwd = pwd[len(real_src):]
	// guard againt xrealwd return the directory without the trailing /
	pwd = strings.TrimPrefix(pwd, "/")

	return pwd
}

// Install installs the list of packages named on the command line.
func cmdinstall() {
	flag.BoolVar(&sflag, "s", sflag, "build static binaries")
	xflagparse(-1)

	if flag.NArg() == 0 {
		install(defaulttarg())
	}

	for _, arg := range flag.Args() {
		install(arg)
	}
}

// Clean deletes temporary objects.
func cmdclean() {
	xflagparse(0)
	clean()
}

// Banner prints the 'now you've installed Go' banner.
func cmdbanner() {
	xflagparse(0)

	xprintf("\n")
	xprintf("---\n")
	xprintf("Installed Go for %s/%s in %s\n", goos, goarch, goroot)
	xprintf("Installed commands in %s\n", gobin)

	if !xsamefile(goroot_final, goroot) {
		// If the files are to be moved, don't check that gobin
		// is on PATH; assume they know what they are doing.
	} else if gohostos == "plan9" {
		// Check that gobin is bound before /bin.
		pid := strings.Replace(readfile("#c/pid"), " ", "", -1)
		ns := fmt.Sprintf("/proc/%s/ns", pid)
		if !strings.Contains(readfile(ns), fmt.Sprintf("bind -b %s /bin", gobin)) {
			xprintf("*** You need to bind %s before /bin.\n", gobin)
		}
	} else {
		// Check that gobin appears in $PATH.
		pathsep := ":"
		if gohostos == "windows" {
			pathsep = ";"
		}
		if !strings.Contains(pathsep+os.Getenv("PATH")+pathsep, pathsep+gobin+pathsep) {
			xprintf("*** You need to add %s to your PATH.\n", gobin)
		}
	}

	if !xsamefile(goroot_final, goroot) {
		xprintf("\n"+
			"The binaries expect %s to be copied or moved to %s\n",
			goroot, goroot_final)
	}
}

// Version prints the Go version.
func cmdversion() {
	xflagparse(0)
	xprintf("%s\n", findgoversion())
}
