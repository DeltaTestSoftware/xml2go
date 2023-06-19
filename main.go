package main

import (
	"bytes"
	"encoding/xml"
	"flag"
	"fmt"
	"go/format"
	"io"
	"os"
	"strings"
	"unicode"
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

	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return err
	}

	var n Node
	err = xml.NewDecoder(bytes.NewBuffer(data)).Decode(&n)
	if err != nil {
		return err
	}

	return generateCode(n)
}

type Node struct {
	XMLName xml.Name
	Attrs   []xml.Attr `xml:",any,attr"`
	Content []byte     `xml:",innerxml"`
	Nodes   []Node     `xml:",any"`
}

func generateCode(n Node) error {
	var w bytes.Buffer

	if *packageName != "" {
		fmt.Fprintf(&w, "package %s\n\nimport \"encoding/xml\"\n\n", *packageName)
	}

	convert(&w, n, nil)

	code, err := format.Source(w.Bytes())
	if err != nil {
		return err
	}

	_, err = os.Stdout.Write(code)
	return err
}

func convert(w io.Writer, n Node, parents []string) {
	nodeName := goIdent(n.XMLName.Local)

	allNames := append(parents, nodeName)
	qualifiedName := strings.Join(allNames, "_")

	type node struct {
		name string
		node Node
	}

	var nodes []node
	counts := make(map[string]int)
	for i := range n.Nodes {
		name := n.Nodes[i].XMLName.Local
		if counts[name] == 0 {
			nodes = append(nodes, node{
				name: name,
				node: n.Nodes[i],
			})
		}
		counts[name]++
	}

	fmt.Fprintf(w, "type %s struct {\n", qualifiedName)
	fmt.Fprintf(w, "\tXMLName xml.Name `xml:\"%s\"`\n", n.XMLName.Local)
	for _, attr := range n.Attrs {
		fmt.Fprintf(w, "\t%s string `xml:\"%s,attr\"`\n", goIdent(attr.Name.Local), attr.Name.Local)
	}
	for _, n := range nodes {
		array := ""
		if counts[n.name] > 1 {
			array = "[]"
		}
		typeName := goIdent(n.name)
		qualifiedTypeName := qualifiedName + "_" + typeName
		fmt.Fprintf(w, "\t%s %s%s `xml:\"%s\"`\n", typeName, array, qualifiedTypeName, n.name)
	}
	fmt.Fprint(w, "}\n\n")

	parents = append(parents, nodeName)
	for _, n := range nodes {
		convert(w, n.node, parents)
	}
	parents = parents[:len(parents)-1]
}

func goIdent(s string) string {
	// Keep only letters, digits and underscores.
	s = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			return r
		}
		return -1
	}, s)

	r := []rune(s)

	// Remove digits from the start of the identifier.
	for len(r) >= 0 && unicode.IsDigit(r[0]) {
		r = r[1:]
	}

	// Capitalize the first letter.
	if len(r) > 0 {
		r[0] = unicode.ToUpper(r[0])
	}

	return string(r)
}
