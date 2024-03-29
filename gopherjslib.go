/*
Package gopherjslib provides helpers for in-process GopherJS compilation.

All of them take the optional *Options argument. It can be used to set
a different GOROOT or GOPATH directory or to enable minification.

Example compiling Go code:

	import "github.com/shurcooL/gopherjslib"

	...

	code := strings.NewReader(`
		package main
		import "github.com/gopherjs/gopherjs/js"
		func main() { println(js.Global.Get("window")) }
	`)

	var out bytes.Buffer

	err := gopherjslib.Build(code, &out, nil) // <- default options

Example compiling multiple files:

	var out bytes.Buffer

	builder := gopherjslib.NewBuilder(&out, nil)

	fileA := strings.NewReader(`
		package main
		import "github.com/gopherjs/gopherjs/js"
		func a() { println(js.Global.Get("window")) }
	`)

	builder.Add("a.go", fileA)

	// And so on for each file, then:

	err = builder.Build()

Deprecated: The intermediate API layer implemented by this
package has proven to be unhelpful and is now unmaintained.
Use packages [github.com/gopherjs/gopherjs/build]
and [github.com/gopherjs/gopherjs/compiler]
or command [github.com/gopherjs/gopherjs] directly instead.
*/
package gopherjslib

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"

	"github.com/gopherjs/gopherjs/build"
	"github.com/gopherjs/gopherjs/compiler"
)

// Options is the subset of build.Options, that is exposed to the user of gopherjslib
// and is optional.
type Options struct {
	GOROOT string // defaults to build.Default.GOROOT
	GOPATH string // defaults to build.Default.GOPATH
	Minify bool   // should the js be minified
}

// toBuildOptions converts to the real build options in build
func (o *Options) toBuildOptions() *build.Options {
	b := &build.Options{}
	if o != nil {
		b.GOROOT = o.GOROOT
		b.GOPATH = o.GOPATH
		b.Minify = o.Minify
	}
	return b
}

// BuildPackage builds JavaScript based on the go package in dir, writing the result to target.
// Note that dir is not relative to any GOPATH, but relative to the working directory.
// The first error during the built is returned.
// dir must be an existing directory containing at least one go file
//
//   target must not be nil
//   options may be nil (defaults)
func BuildPackage(dir string, target io.Writer, options *Options) error {
	b, err := NewPackageBuilder(dir, target, options)
	if err != nil {
		return err
	}
	return b.Build()
}

// Build builds JavaScript based on the go code in reader, writing the result to target.
// The first error during the built is returned. All errors are typed.
//
//   reader must not be nil
//   target must not be nil
//   options may be nil (defaults)
func Build(reader io.Reader, target io.Writer, options *Options) error {
	pb := NewBuilder(target, options)
	pb.Add("main.go", reader)
	return pb.Build()
}

// Builder builds from added files
type Builder interface {
	// Add adds the content of reader for the given filename
	Add(filename string, reader io.Reader) Builder

	// Build builds and returns the first error during the built or <nil>
	Build() error
}

// builder is an implementation of the Builder interface
type builder struct {
	files   map[string]io.Reader
	options *build.Options
	target  io.Writer
	pkgName string
}

// NewBuilder creates a new Builder that will write to target.
//
//   target must not be nil
//   options may be nil (defaults)
func NewBuilder(target io.Writer, options *Options) Builder {
	return &builder{
		files:   map[string]io.Reader{},
		options: options.toBuildOptions(),
		target:  target,
		pkgName: "main", // default, changed by BuildPackage
	}
}

// NewPackageBuilder creates a new Builder based on the go package in dir.
func NewPackageBuilder(dir string, target io.Writer, options *Options) (Builder, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}

	b := NewBuilder(target, options).(*builder)
	b.pkgName = abs

	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		return nil, err
	}

	var f *os.File

	for _, file := range files {
		f, err = os.Open(filepath.Join(dir, filepath.Base(file)))
		if err != nil {
			return nil, err
		}

		// make a copy in order to be able to close the file
		var buf bytes.Buffer
		_, err = io.Copy(&buf, f)
		if err != nil {
			f.Close()
			return nil, err
		}

		b.Add(filepath.Base(file), &buf)
		f.Close()
	}
	return b, nil
}

// Add adds a file with the given filename and the content of reader to the builder
func (b *builder) Add(filename string, reader io.Reader) Builder {
	b.files[filename] = reader
	return b
}

// Build creates a build and returns the first error happening. All errors are typed.
func (b *builder) Build() error {
	if b.target == nil {
		return ErrorMissingTarget{}
	}
	fileSet := token.NewFileSet()

	files := []*ast.File{}

	for name, reader := range b.files {
		if reader == nil {
			return ErrorParsing{name, "reader must not be nil"}
		}
		f, err := parser.ParseFile(fileSet, name, reader, 0)
		if err != nil {
			return ErrorParsing{name, err.Error()}
		}
		files = append(files, f)
	}

	s, err := build.NewSession(b.options)
	if err != nil {
		return err
	}

	importContext := &compiler.ImportContext{
		Packages: s.Types,
		Import:   s.BuildImportPath,
	}
	archive, err := compiler.Compile(b.pkgName, files, fileSet, importContext, b.options.Minify)
	if err != nil {
		return ErrorCompiling(err.Error())
	}

	deps, err := compiler.ImportDependencies(archive, s.BuildImportPath)
	if err != nil {
		return ErrorImportingDependencies(err.Error())
	}

	return compiler.WriteProgramCode(deps, &compiler.SourceMapFilter{Writer: b.target}, s.GoRelease())
}
