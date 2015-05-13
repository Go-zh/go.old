// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/build"
	"go/doc"
	"go/format"
	"go/parser"
	"go/token"
	"log"
	"os"
	"unicode"
	"unicode/utf8"
)

type Package struct {
	name     string       // Package name, json for encoding/json.
	userPath string       // String the user used to find this package.
	pkg      *ast.Package // Parsed package.
	file     *ast.File    // Merged from all files in the package
	doc      *doc.Package
	build    *build.Package
	fs       *token.FileSet // Needed for printing.
	buf      bytes.Buffer
}

// parsePackage turns the build package we found into a parsed package
// we can then use to generate documentation.
func parsePackage(pkg *build.Package, userPath string) *Package {
	fs := token.NewFileSet()
	// include tells parser.ParseDir which files to include.
	// That means the file must be in the build package's GoFiles or CgoFiles
	// list only (no tag-ignored files, tests, swig or other non-Go files).
	include := func(info os.FileInfo) bool {
		for _, name := range pkg.GoFiles {
			if name == info.Name() {
				return true
			}
		}
		for _, name := range pkg.CgoFiles {
			if name == info.Name() {
				return true
			}
		}
		return false
	}
	pkgs, err := parser.ParseDir(fs, pkg.Dir, include, parser.ParseComments)
	if err != nil {
		log.Fatal(err)
	}
	// Make sure they are all in one package.
	if len(pkgs) != 1 {
		log.Fatalf("multiple packages directory %s", pkg.Dir)
	}
	astPkg := pkgs[pkg.Name]

	// TODO: go/doc does not include typed constants in the constants
	// list, which is what we want. For instance, time.Sunday is of type
	// time.Weekday, so it is defined in the type but not in the
	// Consts list for the package. This prevents
	//	go doc time.Sunday
	// from finding the symbol. Work around this for now, but we
	// should fix it in go/doc.
	// A similar story applies to factory functions.
	docPkg := doc.New(astPkg, pkg.ImportPath, doc.AllDecls)
	for _, typ := range docPkg.Types {
		docPkg.Consts = append(docPkg.Consts, typ.Consts...)
		docPkg.Vars = append(docPkg.Vars, typ.Vars...)
		docPkg.Funcs = append(docPkg.Funcs, typ.Funcs...)
	}

	return &Package{
		name:     pkg.Name,
		userPath: userPath,
		pkg:      astPkg,
		file:     ast.MergePackageFiles(astPkg, 0),
		doc:      docPkg,
		build:    pkg,
		fs:       fs,
	}
}

func (pkg *Package) Printf(format string, args ...interface{}) {
	fmt.Fprintf(&pkg.buf, format, args...)
}

func (pkg *Package) flush() {
	_, err := os.Stdout.Write(pkg.buf.Bytes())
	if err != nil {
		log.Fatal(err)
	}
	pkg.buf.Reset() // Not needed, but it's a flush.
}

var newlineBytes = []byte("\n\n") // We never ask for more than 2.

// newlines guarantees there are n newlines at the end of the buffer.
func (pkg *Package) newlines(n int) {
	for !bytes.HasSuffix(pkg.buf.Bytes(), newlineBytes[:n]) {
		pkg.buf.WriteRune('\n')
	}
}

// emit prints the node.
func (pkg *Package) emit(comment string, node ast.Node) {
	if node != nil {
		err := format.Node(&pkg.buf, pkg.fs, node)
		if err != nil {
			log.Fatal(err)
		}
		if comment != "" {
			pkg.newlines(1)
			doc.ToText(&pkg.buf, comment, "    ", "\t", 80)
		}
		pkg.newlines(1)
	}
}

var formatBuf bytes.Buffer // Reusable to avoid allocation.

// formatNode is a helper function for printing.
func (pkg *Package) formatNode(node ast.Node) []byte {
	formatBuf.Reset()
	format.Node(&formatBuf, pkg.fs, node)
	return formatBuf.Bytes()
}

// oneLineFunc prints a function declaration as a single line.
func (pkg *Package) oneLineFunc(decl *ast.FuncDecl) {
	decl.Doc = nil
	decl.Body = nil
	pkg.emit("", decl)
}

// oneLineValueGenDecl prints a var or const declaration as a single line.
func (pkg *Package) oneLineValueGenDecl(decl *ast.GenDecl) {
	decl.Doc = nil
	dotDotDot := ""
	if len(decl.Specs) > 1 {
		dotDotDot = " ..."
	}
	// Find the first relevant spec.
	for i, spec := range decl.Specs {
		valueSpec := spec.(*ast.ValueSpec) // Must succeed; we can't mix types in one genDecl.
		if !isExported(valueSpec.Names[0].Name) {
			continue
		}
		typ := ""
		if valueSpec.Type != nil {
			typ = fmt.Sprintf(" %s", pkg.formatNode(valueSpec.Type))
		}
		val := ""
		if i < len(valueSpec.Values) && valueSpec.Values[i] != nil {
			val = fmt.Sprintf(" = %s", pkg.formatNode(valueSpec.Values[i]))
		}
		pkg.Printf("%s %s%s%s%s\n", decl.Tok, valueSpec.Names[0], typ, val, dotDotDot)
		break
	}
}

// oneLineTypeDecl prints a type declaration as a single line.
func (pkg *Package) oneLineTypeDecl(spec *ast.TypeSpec) {
	spec.Doc = nil
	spec.Comment = nil
	switch spec.Type.(type) {
	case *ast.InterfaceType:
		pkg.Printf("type %s interface { ... }\n", spec.Name)
	case *ast.StructType:
		pkg.Printf("type %s struct { ... }\n", spec.Name)
	default:
		pkg.Printf("type %s %s\n", spec.Name, pkg.formatNode(spec.Type))
	}
}

// packageDoc prints the docs for the package (package doc plus one-liners of the rest).
func (pkg *Package) packageDoc() {
	defer pkg.flush()
	pkg.packageClause(false)

	doc.ToText(&pkg.buf, pkg.doc.Doc, "", "\t", 80)
	pkg.newlines(2)

	pkg.valueSummary(pkg.doc.Consts)
	pkg.valueSummary(pkg.doc.Vars)
	pkg.funcSummary(pkg.doc.Funcs)
	pkg.typeSummary()
}

// packageClause prints the package clause.
// The argument boolean, if true, suppresses the output if the
// user's argument is identical to the actual package path or
// is empty, meaning it's the current directory.
func (pkg *Package) packageClause(checkUserPath bool) {
	if checkUserPath {
		if pkg.userPath == "" || pkg.userPath == pkg.build.ImportPath {
			return
		}
	}
	importPath := pkg.build.ImportComment
	if importPath == "" {
		importPath = pkg.build.ImportPath
	}
	pkg.Printf("package %s // import %q\n\n", pkg.name, importPath)
	if importPath != pkg.build.ImportPath {
		pkg.Printf("WARNING: package source is installed in %q\n", pkg.build.ImportPath)
	}
}

// valueSummary prints a one-line summary for each set of values and constants.
func (pkg *Package) valueSummary(values []*doc.Value) {
	for _, value := range values {
		// Only print first item in spec, show ... to stand for the rest.
		spec := value.Decl.Specs[0].(*ast.ValueSpec) // Must succeed.
		exported := true
		for _, name := range spec.Names {
			if !isExported(name.Name) {
				exported = false
				break
			}
		}
		if exported {
			pkg.oneLineValueGenDecl(value.Decl)
		}
	}
}

// funcSummary prints a one-line summary for each function.
func (pkg *Package) funcSummary(funcs []*doc.Func) {
	for _, fun := range funcs {
		decl := fun.Decl
		// Exported functions only. The go/doc package does not include methods here.
		if isExported(fun.Name) {
			pkg.oneLineFunc(decl)
		}
	}
}

// typeSummary prints a one-line summary for each type.
func (pkg *Package) typeSummary() {
	for _, typ := range pkg.doc.Types {
		for _, spec := range typ.Decl.Specs {
			typeSpec := spec.(*ast.TypeSpec) // Must succeed.
			if isExported(typeSpec.Name.Name) {
				pkg.oneLineTypeDecl(typeSpec)
			}
		}
	}
}

// findValues finds the doc.Values that describe the symbol.
func (pkg *Package) findValues(symbol string, docValues []*doc.Value) (values []*doc.Value) {
	for _, value := range docValues {
		for _, name := range value.Names {
			if match(symbol, name) {
				values = append(values, value)
			}
		}
	}
	return
}

// findFuncs finds the doc.Funcs that describes the symbol.
func (pkg *Package) findFuncs(symbol string) (funcs []*doc.Func) {
	for _, fun := range pkg.doc.Funcs {
		if match(symbol, fun.Name) {
			funcs = append(funcs, fun)
		}
	}
	return
}

// findTypes finds the doc.Types that describes the symbol.
// If symbol is empty, it finds all exported types.
func (pkg *Package) findTypes(symbol string) (types []*doc.Type) {
	for _, typ := range pkg.doc.Types {
		if symbol == "" && isExported(typ.Name) || match(symbol, typ.Name) {
			types = append(types, typ)
		}
	}
	return
}

// findTypeSpec returns the ast.TypeSpec within the declaration that defines the symbol.
// The name must match exactly.
func (pkg *Package) findTypeSpec(decl *ast.GenDecl, symbol string) *ast.TypeSpec {
	for _, spec := range decl.Specs {
		typeSpec := spec.(*ast.TypeSpec) // Must succeed.
		if symbol == typeSpec.Name.Name {
			return typeSpec
		}
	}
	return nil
}

// symbolDoc prints the docs for symbol. There may be multiple matches.
// If symbol matches a type, output includes its methods factories and associated constants.
// If there is no top-level symbol, symbolDoc looks for methods that match.
func (pkg *Package) symbolDoc(symbol string) {
	defer pkg.flush()
	found := false
	// Functions.
	for _, fun := range pkg.findFuncs(symbol) {
		if !found {
			pkg.packageClause(true)
		}
		// Symbol is a function.
		decl := fun.Decl
		decl.Body = nil
		pkg.emit(fun.Doc, decl)
		found = true
	}
	// Constants and variables behave the same.
	values := pkg.findValues(symbol, pkg.doc.Consts)
	values = append(values, pkg.findValues(symbol, pkg.doc.Vars)...)
	for _, value := range values {
		if !found {
			pkg.packageClause(true)
		}
		pkg.emit(value.Doc, value.Decl)
		found = true
	}
	// Types.
	for _, typ := range pkg.findTypes(symbol) {
		if !found {
			pkg.packageClause(true)
		}
		decl := typ.Decl
		spec := pkg.findTypeSpec(decl, typ.Name)
		trimUnexportedFields(spec)
		// If there are multiple types defined, reduce to just this one.
		if len(decl.Specs) > 1 {
			decl.Specs = []ast.Spec{spec}
		}
		pkg.emit(typ.Doc, decl)
		// Show associated methods, constants, etc.
		pkg.valueSummary(typ.Consts)
		pkg.valueSummary(typ.Vars)
		pkg.funcSummary(typ.Funcs)
		pkg.funcSummary(typ.Methods)
		found = true
	}
	if !found {
		// See if there are methods.
		if !pkg.printMethodDoc("", symbol) {
			log.Printf("symbol %s not present in package %s installed in %q", symbol, pkg.name, pkg.build.ImportPath)
		}
	}
}

// trimUnexportedFields modifies spec in place to elide unexported fields (unless
// the unexported flag is set). If spec is not a structure declartion, nothing happens.
func trimUnexportedFields(spec *ast.TypeSpec) {
	if *unexported {
		// We're printing all fields.
		return
	}
	// It must be a struct for us to care. (We show unexported methods in interfaces.)
	structType, ok := spec.Type.(*ast.StructType)
	if !ok {
		return
	}
	trimmed := false
	list := make([]*ast.Field, 0, len(structType.Fields.List))
	for _, field := range structType.Fields.List {
		// Trims if any is unexported. Fine in practice.
		ok := true
		for _, name := range field.Names {
			if !isExported(name.Name) {
				trimmed = true
				ok = false
				break
			}
		}
		if ok {
			list = append(list, field)
		}
	}
	if trimmed {
		unexportedField := &ast.Field{
			Type: ast.NewIdent(""), // Hack: printer will treat this as a field with a named type.
			Comment: &ast.CommentGroup{
				List: []*ast.Comment{
					&ast.Comment{
						Text: "// Has unexported fields.\n",
					},
				},
			},
		}
		list = append(list, unexportedField)
		structType.Fields.List = list
	}
}

// printMethodDoc prints the docs for matches of symbol.method.
// If symbol is empty, it prints all methods that match the name.
// It reports whether it found any methods.
func (pkg *Package) printMethodDoc(symbol, method string) bool {
	defer pkg.flush()
	types := pkg.findTypes(symbol)
	if types == nil {
		if symbol == "" {
			return false
		}
		log.Fatalf("symbol %s is not a type in package %s installed in %q", symbol, pkg.name, pkg.build.ImportPath)
	}
	found := false
	for _, typ := range types {
		for _, meth := range typ.Methods {
			if match(method, meth.Name) {
				decl := meth.Decl
				decl.Body = nil
				pkg.emit(meth.Doc, decl)
				found = true
			}
		}
	}
	return found
}

// methodDoc prints the docs for matches of symbol.method.
func (pkg *Package) methodDoc(symbol, method string) {
	defer pkg.flush()
	if !pkg.printMethodDoc(symbol, method) {
		log.Fatalf("no method %s.%s in package %s installed in %q", symbol, method, pkg.name, pkg.build.ImportPath)
	}
}

// match reports whether the user's symbol matches the program's.
// A lower-case character in the user's string matches either case in the program's.
// The program string must be exported.
func match(user, program string) bool {
	if !isExported(program) {
		return false
	}
	if *matchCase {
		return user == program
	}
	for _, u := range user {
		p, w := utf8.DecodeRuneInString(program)
		program = program[w:]
		if u == p {
			continue
		}
		if unicode.IsLower(u) && simpleFold(u) == simpleFold(p) {
			continue
		}
		return false
	}
	return program == ""
}

// simpleFold returns the minimum rune equivalent to r
// under Unicode-defined simple case folding.
func simpleFold(r rune) rune {
	for {
		r1 := unicode.SimpleFold(r)
		if r1 <= r {
			return r1 // wrapped around, found min
		}
		r = r1
	}
}
