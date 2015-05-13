// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Doc (usually run as go doc) accepts zero, one or two arguments.
//
// Zero arguments:
//	go doc
// Show the documentation for the package in the current directory.
//
// One argument:
//	go doc <pkg>
//	go doc <sym>[.<method>]
//	go doc [<pkg>].<sym>[.<method>]
// The first item in this list that succeeds is the one whose documentation
// is printed. If there is a symbol but no package, the package in the current
// directory is chosen.
//
// Two arguments:
//	go doc <pkg> <sym>[.<method>]
//
// Show the documentation for the package, symbol, and method. The
// first argument must be a full package path. This is similar to the
// command-line usage for the godoc command.
//
// For complete documentation, run "go help doc".
package main

import (
	"flag"
	"fmt"
	"go/build"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	unexported = flag.Bool("u", false, "show unexported symbols as well as exported")
	matchCase  = flag.Bool("c", false, "symbol matching honors case (paths not affected)")
)

// usage is a replacement usage function for the flags package.
func usage() {
	fmt.Fprintf(os.Stderr, "Usage of [go] doc:\n")
	fmt.Fprintf(os.Stderr, "\tgo doc\n")
	fmt.Fprintf(os.Stderr, "\tgo doc <pkg>\n")
	fmt.Fprintf(os.Stderr, "\tgo doc <sym>[.<method>]\n")
	fmt.Fprintf(os.Stderr, "\tgo doc [<pkg>].<sym>[.<method>]\n")
	fmt.Fprintf(os.Stderr, "\tgo doc <pkg> <sym>[.<method>]\n")
	fmt.Fprintf(os.Stderr, "For more information run\n")
	fmt.Fprintf(os.Stderr, "\tgo help doc\n\n")
	fmt.Fprintf(os.Stderr, "Flags:\n")
	flag.PrintDefaults()
	os.Exit(2)
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("doc: ")
	flag.Usage = usage
	flag.Parse()
	buildPackage, userPath, symbol := parseArgs()
	symbol, method := parseSymbol(symbol)
	pkg := parsePackage(buildPackage, userPath)
	switch {
	case symbol == "":
		pkg.packageDoc()
		return
	case method == "":
		pkg.symbolDoc(symbol)
	default:
		pkg.methodDoc(symbol, method)
	}
}

// parseArgs analyzes the arguments (if any) and returns the package
// it represents, the part of the argument the user used to identify
// the path (or "" if it's the current package) and the symbol
// (possibly with a .method) within that package.
// parseSymbol is used to analyze the symbol itself.
func parseArgs() (*build.Package, string, string) {
	switch flag.NArg() {
	default:
		usage()
	case 0:
		// Easy: current directory.
		return importDir(pwd()), "", ""
	case 1:
		// Done below.
	case 2:
		// Package must be importable.
		pkg, err := build.Import(flag.Arg(0), "", build.ImportComment)
		if err != nil {
			log.Fatal(err)
		}
		return pkg, flag.Arg(0), flag.Arg(1)
	}
	// Usual case: one argument.
	arg := flag.Arg(0)
	// If it contains slashes, it begins with a package path.
	// First, is it a complete package path as it is? If so, we are done.
	// This avoids confusion over package paths that have other
	// package paths as their prefix.
	pkg, err := build.Import(arg, "", build.ImportComment)
	if err == nil {
		return pkg, arg, ""
	}
	// Another disambiguator: If the symbol starts with an upper
	// case letter, it can only be a symbol in the current directory.
	// Kills the problem caused by case-insensitive file systems
	// matching an upper case name as a package name.
	if isUpper(arg) {
		pkg, err := build.ImportDir(".", build.ImportComment)
		if err == nil {
			return pkg, "", arg
		}
	}
	// If it has a slash, it must be a package path but there is a symbol.
	// It's the last package path we care about.
	slash := strings.LastIndex(arg, "/")
	// There may be periods in the package path before or after the slash
	// and between a symbol and method.
	// Split the string at various periods to see what we find.
	// In general there may be ambiguities but this should almost always
	// work.
	var period int
	// slash+1: if there's no slash, the value is -1 and start is 0; otherwise
	// start is the byte after the slash.
	for start := slash + 1; start < len(arg); start = period + 1 {
		period = start + strings.Index(arg[start:], ".")
		symbol := ""
		if period < 0 {
			period = len(arg)
		} else {
			symbol = arg[period+1:]
		}
		// Have we identified a package already?
		pkg, err := build.Import(arg[0:period], "", build.ImportComment)
		if err == nil {
			return pkg, arg[0:period], symbol
		}
		// See if we have the basename or tail of a package, as in json for encoding/json
		// or ivy/value for robpike.io/ivy/value.
		path := findPackage(arg[0:period])
		if path != "" {
			return importDir(path), arg[0:period], symbol
		}
	}
	// If it has a slash, we've failed.
	if slash >= 0 {
		log.Fatalf("no such package %s", arg[0:period])
	}
	// Guess it's a symbol in the current directory.
	return importDir(pwd()), "", arg
}

// importDir is just an error-catching wrapper for build.ImportDir.
func importDir(dir string) *build.Package {
	pkg, err := build.ImportDir(dir, build.ImportComment)
	if err != nil {
		log.Fatal(err)
	}
	return pkg
}

// parseSymbol breaks str apart into a symbol and method.
// Both may be missing or the method may be missing.
// If present, each must be a valid Go identifier.
func parseSymbol(str string) (symbol, method string) {
	if str == "" {
		return
	}
	elem := strings.Split(str, ".")
	switch len(elem) {
	case 1:
	case 2:
		method = elem[1]
		isIdentifier(method)
	default:
		log.Printf("too many periods in symbol specification")
		usage()
	}
	symbol = elem[0]
	isIdentifier(symbol)
	return
}

// isIdentifier checks that the name is valid Go identifier, and
// logs and exits if it is not.
func isIdentifier(name string) {
	if len(name) == 0 {
		log.Fatal("empty symbol")
	}
	for i, ch := range name {
		if unicode.IsLetter(ch) || ch == '_' || i > 0 && unicode.IsDigit(ch) {
			continue
		}
		log.Fatalf("invalid identifier %q", name)
	}
}

// isExported reports whether the name is an exported identifier.
// If the unexported flag (-u) is true, isExported returns true because
// it means that we treat the name as if it is exported.
func isExported(name string) bool {
	return *unexported || isUpper(name)
}

// isUpper reports whether the name starts with an upper case letter.
func isUpper(name string) bool {
	ch, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(ch)
}

// findPackage returns the full file name path specified by the
// (perhaps partial) package path pkg.
func findPackage(pkg string) string {
	if pkg == "" {
		return ""
	}
	if isUpper(pkg) {
		return "" // Upper case symbol cannot be a package name.
	}
	path := pathFor(build.Default.GOROOT, pkg)
	if path != "" {
		return path
	}
	for _, root := range splitGopath() {
		path = pathFor(root, pkg)
		if path != "" {
			return path
		}
	}
	return ""
}

// splitGopath splits $GOPATH into a list of roots.
func splitGopath() []string {
	return filepath.SplitList(build.Default.GOPATH)
}

// pathsFor recursively walks the tree at root looking for possible directories for the package:
// those whose package path is pkg or which have a proper suffix pkg.
func pathFor(root, pkg string) (result string) {
	root = path.Join(root, "src")
	slashDot := string(filepath.Separator) + "."
	// We put a slash on the pkg so can use simple string comparison below
	// yet avoid inadvertent matches, like /foobar matching bar.
	pkgString := filepath.Clean(string(filepath.Separator) + pkg)

	// We use panic/defer to short-circuit processing at the first match.
	// A nil panic reports that the path has been found.
	defer func() {
		err := recover()
		if err != nil {
			panic(err)
		}
	}()

	visit := func(pathName string, f os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		// One package per directory. Ignore the files themselves.
		if !f.IsDir() {
			return nil
		}
		// No .git or other dot nonsense please.
		if strings.Contains(pathName, slashDot) {
			return filepath.SkipDir
		}
		// Is the tail of the path correct?
		if strings.HasSuffix(pathName, pkgString) {
			result = pathName
			panic(nil)
		}
		return nil
	}

	filepath.Walk(root, visit)
	return "" // Call to panic above sets the real value.
}

// pwd returns the current directory.
func pwd() string {
	wd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	return wd
}
