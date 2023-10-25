package xml2go_test

import (
	"strings"
	"testing"

	"github.com/DeltaTestSoftware/xml2go"
)

func TestEmptyAttributesAreOmitted(t *testing.T) {
	checkXML(t, `
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
	Content string ´xml:",innerxml"´
}
`)
}

func TestAttributeCanHaveSameNameAsNode(t *testing.T) {
	checkXML(t, `
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
	Content string ´xml:",innerxml"´
}
`)
}

func TestNameSubstituteIsUnique(t *testing.T) {
	checkXML(t, `
<?xml version="1.0" encoding="UTF-8"?>
<root name="John">
	<name>this node is also name</name>
	<name_ a="we use the trailing underscore ourselves"/>
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
	Content string ´xml:",innerxml"´
}

type Root_Name_ struct {
	A string ´xml:"a,attr"´
}
`)
}

func TestContentIdentifierCanBeUsedInXML(t *testing.T) {
	checkXML(t, `
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
	Content   string                  ´xml:",innerxml"´
	Content_  string                  ´xml:"content,attr"´
	Content__ Content_Content_Content ´xml:"content"´
}

type Content_Content_Content struct {
}
`)
}

func TestMultipleXMLsAreCombined(t *testing.T) {
	checkXMLs(t, []string{
		`
<?xml version="1.0" encoding="UTF-8"?>
<a/>
`, `
<?xml version="1.0" encoding="UTF-8"?>
<a name="a"/>
`, `
<?xml version="1.0" encoding="UTF-8"?>
<a>
	<sub/>
</a>
`, `
<?xml version="1.0" encoding="UTF-8"?>
<a>
	<sub>content</sub>
</a>
`, `
<?xml version="1.0" encoding="UTF-8"?>
<b/>
`,
	},
		`
package main

import "encoding/xml"

type A struct {
	XMLName xml.Name ´xml:"a"´
	Name    string   ´xml:"name,attr"´
	Sub     A_Sub    ´xml:"sub"´
}

type A_Sub struct {
	Content string ´xml:",innerxml"´
}

type B struct {
	XMLName xml.Name ´xml:"b"´
}
`)
}

func TestAttributesAndNodesAreSorted(t *testing.T) {
	checkXML(t, `
<?xml version="1.0" encoding="UTF-8"?>
<root b="b" a="a">
	<y yy="yy"/>
	<x xx="xx"/>
</root>
`,
		`
package main

import "encoding/xml"

type Root struct {
	XMLName xml.Name ´xml:"root"´
	A       string   ´xml:"a,attr"´
	B       string   ´xml:"b,attr"´
	X       Root_X   ´xml:"x"´
	Y       Root_Y   ´xml:"y"´
}

type Root_X struct {
	Xx string ´xml:"xx,attr"´
}

type Root_Y struct {
	Yy string ´xml:"yy,attr"´
}
`)
}

func TestNodeTypesAreDeduplicated(t *testing.T) {
	checkXML(t, `
<?xml version="1.0" encoding="UTF-8"?>
<root>
	<a name="Name"/>
	<b name="Name"/>
	<c name="Name"/>
	<d>
		<e name="Name"/>
	</d>
	<f>
		<g name="1"/>
		<g name="2"/>
		<g name="3"/>
	</f>
</root>
`,
		`
package main

import "encoding/xml"

type Root struct {
	XMLName xml.Name ´xml:"root"´
	A       Root_A   ´xml:"a"´
	B       Root_A   ´xml:"b"´
	C       Root_A   ´xml:"c"´
	D       Root_D   ´xml:"d"´
	F       Root_F   ´xml:"f"´
}

type Root_A struct {
	Name string ´xml:"name,attr"´
}

type Root_D struct {
	E Root_A ´xml:"e"´
}

type Root_F struct {
	G []Root_A ´xml:"g"´
}
`)
}

func checkXML(t *testing.T, input, want string) {
	t.Helper()
	checkXMLs(t, []string{input}, want)
}

func checkXMLs(t *testing.T, inputs []string, want string) {
	t.Helper()

	// Parse all XMLs into one converter.
	c := xml2go.New()
	for _, input := range inputs {
		_, err := c.ParseXMLString(input)
		if err != nil {
			t.Fatal(err)
		}
	}
	checkCode(t, c, want)

	// Parse each XML into its own converter and combine them afterwards.
	all := xml2go.New()
	for _, input := range inputs {
		c := xml2go.New()
		_, err := c.ParseXMLString(input)
		if err != nil {
			t.Fatal(err)
		}
		all = xml2go.Combine(all, c)
	}
	checkCode(t, all, want)
}

func checkCode(t *testing.T, c *xml2go.XMLConverter, want string) {
	t.Helper()
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
