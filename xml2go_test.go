package xml2go_test

import (
	"strings"
	"testing"

	"github.com/DeltaTestSoftware/xml2go"
)

func TestEmptyAttributesAreOmitted(t *testing.T) {
	checkCode(t, `
<?xml version="1.0" encoding="UTF-8"?>
<root name="John" empty="">
	<subnode>This is content</subnode>
</root>
`,
		`
package main

import "encoding/xml"

type Root struct {
	XMLName xml.Name     ´xml:"root"´
	Name    string       ´xml:"name,attr"´
	Subnode Root_Subnode ´xml:"subnode"´
}

type Root_Subnode struct {
	XMLName xml.Name ´xml:"subnode"´
	Content string   ´xml:",innerxml"´
}
`)
}

func TestAttributeCanHaveSameNameAsNode(t *testing.T) {
	checkCode(t, `
<?xml version="1.0" encoding="UTF-8"?>
<root name="John">
	<name>this node is also name</name>
</root>
`,
		`
package main

import "encoding/xml"

type Root struct {
	XMLName xml.Name  ´xml:"root"´
	Name    string    ´xml:"name,attr"´
	Name_   Root_Name ´xml:"name"´
}

type Root_Name struct {
	XMLName xml.Name ´xml:"name"´
	Content string   ´xml:",innerxml"´
}
`)
}

func TestNameSubstituteIsUnique(t *testing.T) {
	checkCode(t, `
<?xml version="1.0" encoding="UTF-8"?>
<root name="John">
	<name>this node is also name</name>
	<name_>we use the trailing underscore ourselves</name_>
</root>
`,
		`
package main

import "encoding/xml"

type Root struct {
	XMLName xml.Name   ´xml:"root"´
	Name    string     ´xml:"name,attr"´
	Name_   Root_Name  ´xml:"name"´
	Name__  Root_Name_ ´xml:"name_"´
}

type Root_Name struct {
	XMLName xml.Name ´xml:"name"´
	Content string   ´xml:",innerxml"´
}

type Root_Name_ struct {
	XMLName xml.Name ´xml:"name_"´
	Content string   ´xml:",innerxml"´
}
`)
}

func TestContentIdentifierCanBeUsedInXML(t *testing.T) {
	checkCode(t, `
<?xml version="1.0" encoding="UTF-8"?>
<content content="John">
	<content content="attribute">content everywhere<content/></content>
</content>
`,
		`
package main

import "encoding/xml"

type Content struct {
	XMLName  xml.Name        ´xml:"content"´
	Content  string          ´xml:"content,attr"´
	Content_ Content_Content ´xml:"content"´
}

type Content_Content struct {
	XMLName   xml.Name                ´xml:"content"´
	Content   string                  ´xml:",innerxml"´
	Content_  string                  ´xml:"content,attr"´
	Content__ Content_Content_Content ´xml:"content"´
}

type Content_Content_Content struct {
	XMLName xml.Name ´xml:"content"´
}
`)
}

func checkCode(t *testing.T, input, want string) {
	t.Helper()

	c := xml2go.New()
	_, err := c.ParseXMLString(input)
	if err != nil {
		t.Fatal(err)
	}
	code := c.GenerateGoCodeString("main")
	want = strings.TrimPrefix(want, "\n")
	want = strings.ReplaceAll(want, "´", "`")
	if code != want {
		diffA, diffB, line, col := diffStrings(code, want)
		t.Error("have\n---", "\n"+code, "\nwant\n---", "\n"+want)
		t.Errorf("diff at %d:%d: %q vs %q", line, col, diffA, diffB)
	}
}

func diffStrings(s1, s2 string) (diffA, diffB string, line, col int) {
	a, b := []rune(s1), []rune(s2)

	line = 1
	col = 1

	for len(a) > 0 && len(b) > 0 && a[0] == b[0] {
		if a[0] == '\n' {
			line++
			col = 1
		} else {
			col++
		}

		a = a[1:]
		b = b[1:]
	}

	for len(a) > 0 && len(b) > 0 && a[len(a)-1] == b[len(b)-1] {
		a = a[:len(a)-1]
		b = b[:len(b)-1]
	}

	return string(a), string(b), line, col
}
