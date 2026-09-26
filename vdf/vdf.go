// Package vdf reads Valve's KeyValues formats: text (libraryfolders.vdf,
// appmanifest_*.acf, loginusers.vdf, …) and binary (shortcuts.vdf).
package vdf

import (
	"bufio"
	"io"
	"os"
	"strings"
)

// maxFile caps how much of a file is read; Steam's own files are far smaller.
const maxFile = 32 << 20

// Node is a parsed KeyValues object. Keys are lower-cased because Steam isn't
// consistent about casing ("CloudEnabled" vs "cloudenabled").
type Node struct {
	Values   map[string]string
	Children map[string]*Node
}

func newNode() *Node { return &Node{Values: map[string]string{}, Children: map[string]*Node{}} }

// Get follows a path of child keys (case-insensitive); nil if any is missing.
func (n *Node) Get(keys ...string) *Node {
	for _, k := range keys {
		if n == nil {
			return nil
		}
		n = n.Children[strings.ToLower(k)]
	}
	return n
}

// Kids returns the child objects; nil-safe.
func (n *Node) Kids() map[string]*Node {
	if n == nil {
		return nil
	}
	return n.Children
}

// Value returns a value of this node (case-insensitive); "" if n is nil.
func (n *Node) Value(key string) string {
	if n == nil {
		return ""
	}
	return n.Values[strings.ToLower(key)]
}

// ReadFile parses the file at p; an empty node if it can't be read.
func ReadFile(p string) *Node {
	f, err := os.Open(p)
	if err != nil {
		return newNode()
	}
	defer f.Close()
	return Parse(io.LimitReader(f, maxFile))
}

// Parse reads text VDF. It is lenient: malformed input yields whatever was
// parsed up to that point.
func Parse(r io.Reader) *Node {
	br := bufio.NewReader(r)
	root := newNode()
	stack := []*Node{root}
	var pending *string // key waiting for its value or '{'
	for {
		tok, isStr, err := next(br)
		if err != nil {
			return root
		}
		cur := stack[len(stack)-1]
		switch {
		case !isStr && tok == "{":
			child := newNode()
			if pending != nil {
				cur.Children[strings.ToLower(*pending)] = child
				pending = nil
			}
			if len(stack) < 64 {
				stack = append(stack, child)
			}
		case !isStr && tok == "}":
			pending = nil
			if len(stack) > 1 {
				stack = stack[:len(stack)-1]
			}
		case pending == nil:
			k := tok
			pending = &k
		default:
			cur.Values[strings.ToLower(*pending)] = tok
			pending = nil
		}
	}
}

// next returns the next token: a quoted or bare string, or '{' / '}'.
func next(br *bufio.Reader) (string, bool, error) {
	for {
		c, err := br.ReadByte()
		if err != nil {
			return "", false, err
		}
		switch {
		case c == ' ' || c == '\t' || c == '\r' || c == '\n':
		case c == '/':
			// "// comment" to end of line.
			if p, _ := br.Peek(1); len(p) == 1 && p[0] == '/' {
				if _, err := br.ReadString('\n'); err != nil {
					return "", false, err
				}
			}
		case c == '{' || c == '}':
			return string(c), false, nil
		case c == '"':
			var sb strings.Builder
			for {
				c, err := br.ReadByte()
				if err != nil {
					return "", false, err
				}
				if c == '"' {
					return sb.String(), true, nil
				}
				if c == '\\' {
					e, err := br.ReadByte()
					if err != nil {
						return "", false, err
					}
					switch e {
					case 'n':
						sb.WriteByte('\n')
					case 't':
						sb.WriteByte('\t')
					default:
						sb.WriteByte(e)
					}
					continue
				}
				sb.WriteByte(c)
			}
		default:
			var sb strings.Builder
			sb.WriteByte(c)
			for {
				p, err := br.Peek(1)
				if err != nil || strings.ContainsRune(" \t\r\n{}\"", rune(p[0])) {
					return sb.String(), true, nil
				}
				b, _ := br.ReadByte()
				sb.WriteByte(b)
			}
		}
	}
}
