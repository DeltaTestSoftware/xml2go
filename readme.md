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

Example
=======

This XML file:

`<?xml version="1.0" encoding="UTF-8"?>
<TopLevel Name="NameAttribute">
	<nodeWithContent>This is content</nodeWithContent>
	<thisHasNoContent>
		<SubNode/>
	</thisHasNoContent>
</TopLevel>``
```

will generate this Go code:

```
package main

import "encoding/xml"

type TopLevel struct {
	XMLName          xml.Name                  `xml:"TopLevel"`
	Name             string                    `xml:"Name,attr"`
	NodeWithContent  TopLevel_NodeWithContent  `xml:"nodeWithContent"`
	ThisHasNoContent TopLevel_ThisHasNoContent `xml:"thisHasNoContent"`
}

type TopLevel_NodeWithContent struct {
	XMLName xml.Name `xml:"nodeWithContent"`
	Content string   `xml:",innerxml"`
}

type TopLevel_ThisHasNoContent struct {
	XMLName xml.Name                          `xml:"thisHasNoContent"`
	SubNode TopLevel_ThisHasNoContent_SubNode `xml:"SubNode"`
}

type TopLevel_ThisHasNoContent_SubNode struct {
	XMLName xml.Name `xml:"SubNode"`
}
```
