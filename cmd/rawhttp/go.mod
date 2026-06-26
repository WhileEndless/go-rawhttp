module github.com/WhileEndless/go-rawhttp/cmd/rawhttp

go 1.25.0

require (
	github.com/WhileEndless/go-rawhttp v1.0.0
	github.com/alecthomas/chroma/v2 v2.27.0
	github.com/andybalholm/brotli v1.2.1
	github.com/evanw/esbuild v0.28.1
	github.com/go-xmlfmt/xmlfmt v1.1.3
	github.com/spf13/pflag v1.0.10
	github.com/yosssi/gohtml v0.0.0-20201013000340-ee4748c638f4
	golang.org/x/term v0.44.0
	golang.org/x/text v0.31.0
)

require (
	github.com/dlclark/regexp2/v2 v2.2.1 // indirect
	golang.org/x/net v0.47.0 // indirect
	golang.org/x/sys v0.46.0 // indirect
)

// Build against the library in this repo so the CLI tracks local changes.
replace github.com/WhileEndless/go-rawhttp => ../..
