gopherjslib
===========

[![Go Reference](https://pkg.go.dev/badge/github.com/shurcooL/gopherjslib.svg)](https://pkg.go.dev/github.com/shurcooL/gopherjslib)

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

Installation
------------

```sh
go get github.com/shurcooL/gopherjslib
```

License
-------

-	[MIT License](LICENSE)
