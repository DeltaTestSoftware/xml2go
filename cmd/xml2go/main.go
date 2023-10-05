package main

import (
	"flag"
	"os"

	"github.com/DeltaTestSoftware/xml2go"
)

var packageName = flag.String(
	"package",
	"main",
	"package name, if empty, no package and import are generated",
)

func main() {
	if err := run(); err != nil {
		panic(err)
	}
}

func run() error {
	flag.Parse()

	p := xml2go.New()
	_, err := p.ParseXMLReader(os.Stdin)
	if err != nil {
		return err
	}

	return p.GenerateGoCodeWriter(*packageName, os.Stdout)
}
