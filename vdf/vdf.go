// Package vdf reads Valve's KeyValues formats: text (libraryfolders.vdf,
// appmanifest_*.acf, loginusers.vdf, …) and binary (shortcuts.vdf). Binary
// files can also be written back unchanged apart from the caller's edits.
//
// Both parsers take untrusted input: sizes and nesting are capped, and the
// text parser never fails, it returns whatever it could read.
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
	Values   map[string]string // "key" "value" pairs
	Children map[string]*Node  // "key" { … } objects
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

// maxNesting caps how deep objects nest; deeper objects are skipped whole.
const maxNesting = 64

// token kinds returned by next.
const (
	tokOpen   = iota // {
	tokClose         // }
	tokQuoted        // "string"
	tokBare          // string without quotes
)

// Parse reads text VDF. It is lenient: malformed input yields whatever was
// parsed up to that point. A leading UTF-8 byte order mark and conditionals
// such as [$WIN32] after a value are ignored, and objects nested deeper than
// 64 levels are skipped.
func Parse(r io.Reader) *Node {
	br := bufio.NewReader(r)
	if p, _ := br.Peek(3); string(p) == "\xef\xbb\xbf" {
		_, _ = br.Discard(3)
	}
	root := newNode()
	stack := []*Node{root}
	skip := 0           // levels entered beyond maxNesting
	var pending *string // key waiting for its value or '{'
	for {
		tok, kind, err := next(br)
		if err != nil {
			return root
		}
		cur := stack[len(stack)-1]
		switch {
		case kind == tokOpen && (skip > 0 || len(stack) >= maxNesting):
			skip++
			pending = nil
		case kind == tokClose && skip > 0:
			skip--
		case skip > 0:
		case kind == tokOpen:
			child := newNode()
			if pending != nil {
				cur.Children[strings.ToLower(*pending)] = child
				pending = nil
			}
			stack = append(stack, child)
		case kind == tokClose:
			pending = nil
			if len(stack) > 1 {
				stack = stack[:len(stack)-1]
			}
		case kind == tokBare && isConditional(tok):
		case pending == nil:
			k := tok
			pending = &k
		default:
			cur.Values[strings.ToLower(*pending)] = tok
			pending = nil
		}
	}
}

// isConditional reports whether a bare token is a platform conditional
// such as [$WIN32] or [!$X360].
func isConditional(tok string) bool {
	return len(tok) >= 2 && tok[0] == '[' && tok[len(tok)-1] == ']'
}

// next returns the next token: a quoted or bare string, or '{' / '}'.
func next(br *bufio.Reader) (string, int, error) {
	for {
		c, err := br.ReadByte()
		if err != nil {
			return "", 0, err
		}
		switch {
		case c == ' ' || c == '\t' || c == '\r' || c == '\n':
		case c == '/' && peekIs(br, '/'):
			// "// comment" to end of line.
			if _, err := br.ReadString('\n'); err != nil {
				return "", 0, err
			}
		case c == '{':
			return "{", tokOpen, nil
		case c == '}':
			return "}", tokClose, nil
		case c == '"':
			var sb strings.Builder
			for {
				c, err := br.ReadByte()
				if err != nil {
					return "", 0, err
				}
				if c == '"' {
					return sb.String(), tokQuoted, nil
				}
				if c == '\\' {
					e, err := br.ReadByte()
					if err != nil {
						return "", 0, err
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
					return sb.String(), tokBare, nil
				}
				b, _ := br.ReadByte()
				sb.WriteByte(b)
			}
		}
	}
}

// peekIs reports whether the next byte is c, without consuming it.
func peekIs(br *bufio.Reader, c byte) bool {
	p, _ := br.Peek(1)
	return len(p) == 1 && p[0] == c
}
