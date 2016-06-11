// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This program generates a test to verify that the standard arithmetic
// operators properly handle const cases. The test file should be
// generated with a known working version of go.
// launch with `go run arithConstGen.go` a file called arithConst_ssa.go
// will be written into the parent directory containing the tests

package main

import (
	"bytes"
	"fmt"
	"go/format"
	"io/ioutil"
	"log"
	"strings"
	"text/template"
)

type op struct {
	name, symbol string
}
type szD struct {
	name string
	sn   string
	u    []uint64
	i    []int64
}

var szs []szD = []szD{
	szD{name: "uint64", sn: "64", u: []uint64{0, 1, 4294967296, 0xffffFFFFffffFFFF}},
	szD{name: "int64", sn: "64", i: []int64{-0x8000000000000000, -0x7FFFFFFFFFFFFFFF,
		-4294967296, -1, 0, 1, 4294967296, 0x7FFFFFFFFFFFFFFE, 0x7FFFFFFFFFFFFFFF}},

	szD{name: "uint32", sn: "32", u: []uint64{0, 1, 4294967295}},
	szD{name: "int32", sn: "32", i: []int64{-0x80000000, -0x7FFFFFFF, -1, 0,
		1, 0x7FFFFFFF}},

	szD{name: "uint16", sn: "16", u: []uint64{0, 1, 65535}},
	szD{name: "int16", sn: "16", i: []int64{-32768, -32767, -1, 0, 1, 32766, 32767}},

	szD{name: "uint8", sn: "8", u: []uint64{0, 1, 255}},
	szD{name: "int8", sn: "8", i: []int64{-128, -127, -1, 0, 1, 126, 127}},
}

var ops []op = []op{op{"add", "+"}, op{"sub", "-"}, op{"div", "/"}, op{"mul", "*"},
	op{"lsh", "<<"}, op{"rsh", ">>"}, op{"mod", "%"}}

// compute the result of i op j, cast as type t.
func ansU(i, j uint64, t, op string) string {
	var ans uint64
	switch op {
	case "+":
		ans = i + j
	case "-":
		ans = i - j
	case "*":
		ans = i * j
	case "/":
		if j != 0 {
			ans = i / j
		}
	case "%":
		if j != 0 {
			ans = i % j
		}
	case "<<":
		ans = i << j
	case ">>":
		ans = i >> j
	}
	switch t {
	case "uint32":
		ans = uint64(uint32(ans))
	case "uint16":
		ans = uint64(uint16(ans))
	case "uint8":
		ans = uint64(uint8(ans))
	}
	return fmt.Sprintf("%d", ans)
}

// compute the result of i op j, cast as type t.
func ansS(i, j int64, t, op string) string {
	var ans int64
	switch op {
	case "+":
		ans = i + j
	case "-":
		ans = i - j
	case "*":
		ans = i * j
	case "/":
		if j != 0 {
			ans = i / j
		}
	case "%":
		if j != 0 {
			ans = i % j
		}
	case "<<":
		ans = i << uint64(j)
	case ">>":
		ans = i >> uint64(j)
	}
	switch t {
	case "int32":
		ans = int64(int32(ans))
	case "int16":
		ans = int64(int16(ans))
	case "int8":
		ans = int64(int8(ans))
	}
	return fmt.Sprintf("%d", ans)
}

func main() {

	w := new(bytes.Buffer)

	fmt.Fprintf(w, "package main;\n")
	fmt.Fprintf(w, "import \"fmt\"\n")

	fncCnst1, err := template.New("fnc").Parse(
		`//go:noinline
		func {{.Name}}_{{.Type_}}_{{.FNumber}}_ssa(a {{.Type_}}) {{.Type_}} {
	return a {{.Symbol}} {{.Number}}
}
`)
	if err != nil {
		panic(err)
	}
	fncCnst2, err := template.New("fnc").Parse(
		`//go:noinline
		func {{.Name}}_{{.FNumber}}_{{.Type_}}_ssa(a {{.Type_}}) {{.Type_}} {
	return {{.Number}} {{.Symbol}} a
}

`)
	if err != nil {
		panic(err)
	}

	type fncData struct {
		Name, Type_, Symbol, FNumber, Number string
	}

	for _, s := range szs {
		for _, o := range ops {
			fd := fncData{o.name, s.name, o.symbol, "", ""}

			// unsigned test cases
			if len(s.u) > 0 {
				for _, i := range s.u {
					fd.Number = fmt.Sprintf("%d", i)
					fd.FNumber = strings.Replace(fd.Number, "-", "Neg", -1)

					// avoid division by zero
					if o.name != "mod" && o.name != "div" || i != 0 {
						fncCnst1.Execute(w, fd)
					}

					fncCnst2.Execute(w, fd)
				}
			}

			// signed test cases
			if len(s.i) > 0 {
				// don't generate tests for shifts by signed integers
				if o.name == "lsh" || o.name == "rsh" {
					continue
				}
				for _, i := range s.i {
					fd.Number = fmt.Sprintf("%d", i)
					fd.FNumber = strings.Replace(fd.Number, "-", "Neg", -1)

					// avoid division by zero
					if o.name != "mod" && o.name != "div" || i != 0 {
						fncCnst1.Execute(w, fd)
					}
					fncCnst2.Execute(w, fd)
				}
			}
		}
	}

	fmt.Fprintf(w, "var failed bool\n\n")
	fmt.Fprintf(w, "func main() {\n\n")

	vrf1, _ := template.New("vrf1").Parse(`
  if got := {{.Name}}_{{.FNumber}}_{{.Type_}}_ssa({{.Input}}); got != {{.Ans}} {
  	fmt.Printf("{{.Name}}_{{.Type_}} {{.Number}}%s{{.Input}} = %d, wanted {{.Ans}}\n", ` + "`{{.Symbol}}`" + `, got)
  	failed = true
  }
`)

	vrf2, _ := template.New("vrf2").Parse(`
  if got := {{.Name}}_{{.Type_}}_{{.FNumber}}_ssa({{.Input}}); got != {{.Ans}} {
    fmt.Printf("{{.Name}}_{{.Type_}} {{.Input}}%s{{.Number}} = %d, wanted {{.Ans}}\n", ` + "`{{.Symbol}}`" + `, got)
    failed = true
  }
`)

	type cfncData struct {
		Name, Type_, Symbol, FNumber, Number string
		Ans, Input                           string
	}
	for _, s := range szs {
		if len(s.u) > 0 {
			for _, o := range ops {
				fd := cfncData{o.name, s.name, o.symbol, "", "", "", ""}
				for _, i := range s.u {
					fd.Number = fmt.Sprintf("%d", i)
					fd.FNumber = strings.Replace(fd.Number, "-", "Neg", -1)

					// unsigned
					for _, j := range s.u {

						if o.name != "mod" && o.name != "div" || j != 0 {
							fd.Ans = ansU(i, j, s.name, o.symbol)
							fd.Input = fmt.Sprintf("%d", j)
							err = vrf1.Execute(w, fd)
							if err != nil {
								panic(err)
							}
						}

						if o.name != "mod" && o.name != "div" || i != 0 {
							fd.Ans = ansU(j, i, s.name, o.symbol)
							fd.Input = fmt.Sprintf("%d", j)
							err = vrf2.Execute(w, fd)
							if err != nil {
								panic(err)
							}
						}

					}
				}

			}
		}

		// signed
		if len(s.i) > 0 {
			for _, o := range ops {
				// don't generate tests for shifts by signed integers
				if o.name == "lsh" || o.name == "rsh" {
					continue
				}
				fd := cfncData{o.name, s.name, o.symbol, "", "", "", ""}
				for _, i := range s.i {
					fd.Number = fmt.Sprintf("%d", i)
					fd.FNumber = strings.Replace(fd.Number, "-", "Neg", -1)
					for _, j := range s.i {
						if o.name != "mod" && o.name != "div" || j != 0 {
							fd.Ans = ansS(i, j, s.name, o.symbol)
							fd.Input = fmt.Sprintf("%d", j)
							err = vrf1.Execute(w, fd)
							if err != nil {
								panic(err)
							}
						}

						if o.name != "mod" && o.name != "div" || i != 0 {
							fd.Ans = ansS(j, i, s.name, o.symbol)
							fd.Input = fmt.Sprintf("%d", j)
							err = vrf2.Execute(w, fd)
							if err != nil {
								panic(err)
							}
						}

					}
				}

			}
		}
	}

	fmt.Fprintf(w, `if failed {
        panic("tests failed")
    }
`)
	fmt.Fprintf(w, "}\n")

	// gofmt result
	b := w.Bytes()
	src, err := format.Source(b)
	if err != nil {
		fmt.Printf("%s\n", b)
		panic(err)
	}

	// write to file
	err = ioutil.WriteFile("../arithConst_ssa.go", src, 0666)
	if err != nil {
		log.Fatalf("can't write output: %v\n", err)
	}
}
