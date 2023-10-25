package xml2go

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"go/format"
	"io"
	"os"
	"sort"
	"strings"
	"unicode"
)

func New() *XMLConverter {
	return &XMLConverter{root: &NodeDesc{}}
}

type XMLConverter struct {
	root *NodeDesc
}

func (c *XMLConverter) AllNodes() []*NodeDesc {
	return c.root.Children
}

func Combine(a, b *XMLConverter) *XMLConverter {
	return combine(a, b)
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
	node := c.parse(n, c.root)
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
		if attr.Value == "" {
			continue // Skip empty attributes.
		}
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
	i := n.getChildIndex(name)

	if i == -1 {
		n.Children = append(n.Children, &NodeDesc{
			Parent: n,
			Name:   name,
		})
		i = len(n.Children) - 1
	}

	return n.Children[i]
}

func (n *NodeDesc) getChildIndex(name string) int {
	for i, child := range n.Children {
		if child.Name == name {
			return i
		}
	}
	return -1
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

	root := copyNode(c.root) // Do not sort the original.
	sortNode(root)
	knownTypes := map[*NodeDesc]string{}
	for _, child := range root.Children {
		c.generate(&buf, child, knownTypes)
	}

	code, err := format.Source(buf.Bytes())
	if err != nil {
		// Panic here, this would be a developer error.
		panic(err)
	}

	return code
}

func (c *XMLConverter) GenerateGoCodeFile(packageName string, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return c.GenerateGoCodeWriter(packageName, f)
}

func (c *XMLConverter) GenerateGoCodeWriter(packageName string, w io.Writer) error {
	code := c.GenerateGoCodeBytes(packageName)
	_, err := w.Write(code)
	return err
}

func (c *XMLConverter) generate(
	buf *bytes.Buffer,
	n *NodeDesc,
	knownTypes map[*NodeDesc]string,
) {
	getKnownType := func(n *NodeDesc) (string, bool) {
		for typ, name := range knownTypes {
			if sameStructure(n, typ) {
				return name, true
			}
		}
		return "", false
	}

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
	isRoot := n.Parent == nil
	if isRoot {
		// Only the top-level types have XMLName set.
		fmt.Fprintf(buf, "\tXMLName xml.Name `xml:\"%s\"`\n", n.Name)
	}

	// We prevent naming conflicts that can occur when an attribute has the
	// same name as a node by appending one or more underscores to the
	// identifiers.
	hasName := map[string]bool{}
	uniqueGoIdent := func(name string) string {
		id := goIdent(name)
		for hasName[id] {
			id += "_"
		}
		hasName[id] = true
		return id
	}

	if n.HasCharacterData {
		// We always call it Content. If an attribute or child node has this
		// name as well, those will be renamed (because we use uniqueGoIdent).
		id := uniqueGoIdent("Content")
		fmt.Fprintf(buf, "\t%s string `xml:\",innerxml\"`\n", id)
	}

	// Write all attributes as strings. We do not convert to integers here,
	// that can be done by the user.
	for _, attr := range n.Attributes {
		id := uniqueGoIdent(attr)
		fmt.Fprintf(buf, "\t%s string `xml:\"%s,attr\"`\n", id, attr)
	}

	// Write all child nodes. Some node names appear more than once, in that
	// case we make the struct field a slice.
	skipChildGen := make([]bool, len(n.Children))
	for i, child := range n.Children {
		slice := ""
		if child.IsArray {
			slice = "[]"
		}
		childIdent := uniqueGoIdent(child.Name)
		childType := nodeName + "_" + goIdent(child.Name)
		if typeName, ok := getKnownType(child); ok {
			childType = typeName
			skipChildGen[i] = true
		} else {
			knownTypes[child] = childType
		}
		fmt.Fprintf(
			buf,
			"\t%s %s%s `xml:\"%s\"`\n",
			childIdent, slice, childType, child.Name,
		)
	}
	fmt.Fprint(buf, "}\n\n")

	// Now create node structs for all the children of this node, recursively.
	for i, child := range n.Children {
		if !skipChildGen[i] {
			c.generate(buf, child, knownTypes)
		}
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
	for len(r) > 0 && unicode.IsDigit(r[0]) {
		r = r[1:]
	}

	// Capitalize the first letter.
	if len(r) > 0 {
		r[0] = unicode.ToUpper(r[0])
	}

	return string(r)
}

func combine(a, b *XMLConverter) *XMLConverter {
	c := &XMLConverter{
		root: combineNodes(
			copyNode(a.root),
			copyNode(b.root),
		),
	}
	for _, child := range c.root.Children {
		child.Parent = nil
	}
	return c
}

func copyNode(n *NodeDesc) *NodeDesc {
	m := &NodeDesc{
		Parent:           nil, // This is not part of n's Parent's children.
		Name:             n.Name,
		IsArray:          n.IsArray,
		HasCharacterData: n.HasCharacterData,
	}

	m.Attributes = make([]string, len(n.Attributes))
	copy(m.Attributes, n.Attributes)

	m.Children = make([]*NodeDesc, len(n.Children))
	for i := range m.Children {
		m.Children[i] = copyNode(n.Children[i])
		if n.Children[i].Parent != nil {
			m.Children[i].Parent = m
		}
	}

	return m
}

func combineNodes(a, b *NodeDesc) *NodeDesc {
	if a.Name != b.Name {
		panic("developer error: only combineNodes with equal Name")
	}

	// We change a, integrating b into it.

	a.IsArray = a.IsArray || b.IsArray
	a.HasCharacterData = a.HasCharacterData || b.HasCharacterData

	for _, attrB := range b.Attributes {
		if !a.containsAttribute(attrB) {
			a.Attributes = append(a.Attributes, attrB)
		}
	}

	for _, childB := range b.Children {
		i := a.getChildIndex(childB.Name)
		if i == -1 {
			a.Children = append(a.Children, childB)
			childB.Parent = a
		} else {
			a.Children[i] = combineNodes(a.Children[i], childB)
			a.Children[i].Parent = a
		}
	}

	return a
}

func sortNode(n *NodeDesc) {
	sort.Strings(n.Attributes)
	sort.Slice(n.Children, func(i, j int) bool {
		return n.Children[i].Name < n.Children[j].Name
	})
	for _, child := range n.Children {
		sortNode(child)
	}
}

func sameStructure(a, b *NodeDesc) bool {
	if a.HasCharacterData != b.HasCharacterData {
		return false
	}
	if len(a.Attributes) != len(b.Attributes) {
		return false
	}
	for i := range a.Attributes {
		if a.Attributes[i] != b.Attributes[i] {
			return false
		}
	}
	if len(a.Children) != len(b.Children) {
		return false
	}
	for i := range a.Children {
		if a.Children[i].Name != b.Children[i].Name {
			return false
		}
	}
	for i := range a.Children {
		if !sameStructure(a.Children[i], b.Children[i]) {
			return false
		}
	}
	return true
}
