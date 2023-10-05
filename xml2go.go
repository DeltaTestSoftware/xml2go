package xml2go

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"go/format"
	"io"
	"os"
	"strings"
	"unicode"
)

func New() *XMLConverter {
	return &XMLConverter{}
}

type XMLConverter struct {
	root NodeDesc
}

func (c *XMLConverter) AllNodes() []*NodeDesc {
	return c.root.Children
}

func (c *XMLConverter) ParseXMLString(data string) (*NodeDesc, error) {
	return c.ParseXMLBytes([]byte(data))
}

func (c *XMLConverter) ParseXMLFile(path string) (*NodeDesc, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return c.ParseXMLBytes(data)
}

func (c *XMLConverter) ParseXMLBytes(data []byte) (*NodeDesc, error) {
	return c.ParseXMLReader(bytes.NewReader(data))
}

func (c *XMLConverter) ParseXMLReader(r io.Reader) (*NodeDesc, error) {
	var n node
	err := xml.NewDecoder(r).Decode(&n)
	if err != nil {
		return nil, err
	}
	node := c.parse(n, &c.root)
	node.Parent = nil
	return node, nil
}

type node struct {
	XMLName xml.Name
	Attrs   []xml.Attr `xml:",any,attr"`
	Content []byte     `xml:",innerxml"`
	Nodes   []node     `xml:",any"`
}

func (n *node) countChildrenWithName(name string) int {
	count := 0
	for i := range n.Nodes {
		if n.Nodes[i].XMLName.Local == name {
			count++
		}
	}
	return count
}

func (c *XMLConverter) parse(n node, parent *NodeDesc) *NodeDesc {
	child := parent.getOrAddChild(n.XMLName.Local)

	// A node might contain raw text, like:
	//
	//   <Node>Some raw text</Node>
	//
	// and in that case we insert a Content field (hoping this name is not
	// taken by any attribute or child-node).
	//
	// The way Go's XML parsing handles the ",innerxml" tag is that it will put
	// all contents of a node in it, so a node like this:
	//
	//   <Node>
	//     <SubNode/>
	//   </Node>
	//
	// will contain this content: "\n    <SubNode/>\n  ". We simply get all
	// content, even the XML nodes.
	// Since we only want to add a Content string if there is raw textual
	// content, we check if it starts with "<", in which case it is probably
	// another node.
	content := bytes.TrimSpace(n.Content)
	if !child.HasCharacterData {
		child.HasCharacterData = len(content) > 0 &&
			!bytes.HasPrefix(content, []byte{'<'})
	}

	for _, attr := range n.Attrs {
		name := attr.Name.Local
		if !child.containsAttribute(name) {
			child.Attributes = append(child.Attributes, name)
		}
	}

	for i := range n.Nodes {
		grandChild := c.parse(n.Nodes[i], child)
		if !grandChild.IsArray {
			grandChild.IsArray = n.countChildrenWithName(grandChild.Name) > 1
		}
	}

	return child
}

type NodeDesc struct {
	Parent           *NodeDesc
	Name             string
	IsArray          bool
	HasCharacterData bool
	Attributes       []string
	Children         []*NodeDesc
}

func (n *NodeDesc) getOrAddChild(name string) *NodeDesc {
	for _, child := range n.Children {
		if child.Name == name {
			return child
		}
	}

	child := &NodeDesc{
		Parent: n,
		Name:   name,
	}
	n.Children = append(n.Children, child)
	return child
}

func (n *NodeDesc) containsAttribute(name string) bool {
	for _, attr := range n.Attributes {
		if attr == name {
			return true
		}
	}
	return false
}

func (c *XMLConverter) GenerateGoCodeString(packageName string) string {
	return string(c.GenerateGoCodeBytes(packageName))
}

func (c *XMLConverter) GenerateGoCodeBytes(packageName string) []byte {
	var buf bytes.Buffer
	if packageName != "" {
		fmt.Fprintf(&buf, "package %s\n\nimport \"encoding/xml\"\n\n", packageName)
	}
	for _, child := range c.root.Children {
		c.generate(&buf, child)
	}
	code, err := format.Source(buf.Bytes())
	if err != nil {
		// Panic here, this would be a developer error.
		panic(err)
	}
	return code
}

func (c *XMLConverter) GenerateGoCodeWriter(packageName string, w io.Writer) error {
	code := c.GenerateGoCodeBytes(packageName)
	_, err := w.Write(code)
	return err
}

func (c *XMLConverter) generate(buf *bytes.Buffer, n *NodeDesc) {
	// To not create two structs with the same name (which will be a compile
	// error), we fully qualify all nodes with their complete hierarchy. For
	// example these nodes:
	//
	//   <Top>
	//     <Other/>
	//     <SecondLevel>
	//       <Other/>
	//     <SecondLevel>
	//   </Top>
	//
	// will generate structs `Top`, `Top_Other`, `Top_SecondLevel` and
	// `Top_SecondLevel_Other`.
	// If we just used the node name, `Other` would be created twice.
	nodeName := goIdent(n.Name)
	p := n.Parent
	for p != nil {
		nodeName = goIdent(p.Name) + "_" + nodeName
		p = p.Parent
	}

	// Create the Go struct for this node.
	fmt.Fprintf(buf, "type %s struct {\n", nodeName)
	fmt.Fprintf(buf, "\tXMLName xml.Name `xml:\"%s\"`\n", n.Name)

	if n.HasCharacterData {
		// NOTE that we always use the name Content for this varialbe. This
		// might conflict with an existing attribute, we just hope it does not.
		fmt.Fprint(buf, "\tContent string `xml:\",innerxml\"`\n")
	}

	// Write all attributes as strings. We do not convert to integers here,
	// that can be done by the user.
	for _, attr := range n.Attributes {
		fmt.Fprintf(buf, "\t%s string `xml:\"%s,attr\"`\n", goIdent(attr), attr)
	}

	// Write all child nodes. Some node names appear more than once, in that
	// case we make the struct field a slice.
	for _, child := range n.Children {
		slice := ""
		if child.IsArray {
			slice = "[]"
		}
		childIdent := goIdent(child.Name)
		childType := nodeName + "_" + childIdent
		fmt.Fprintf(
			buf,
			"\t%s %s%s `xml:\"%s\"`\n",
			childIdent, slice, childType, child.Name,
		)
	}
	fmt.Fprint(buf, "}\n\n")

	// Now create node structs for all the children of this node, recursively.
	for _, child := range n.Children {
		c.generate(buf, child)
	}
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
