XML to Go
=========

`xml2go` reads an XML file and generates Go structs that represent the XML
tree.

Installation:

    go install github.com/DeltaTestSoftware/xml2go@latest

Usage:

    xml2go < example.xml > schema.go

This will create a main package file `schema.go` representing the tree of the
XML file `example.xml`.

To change the package name, use the `package` option:

    xml2go -package="schema" < example.xml > schema.go

Leave the package empty to only create a stub Go file without the package and
import encoding/xml clauses.

    xml2go -package="" < example.xml > schema.go
