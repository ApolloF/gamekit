package vdf

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Binary KeyValues type bytes (shortcuts.vdf, appinfo, …).
const (
	BMap    byte = 0x00
	BString byte = 0x01
	BInt32  byte = 0x02
	BFloat  byte = 0x03
	BPtr    byte = 0x04
	BColor  byte = 0x06
	BUint64 byte = 0x07
	BEnd    byte = 0x08
	BInt64  byte = 0x0a
)

// maxDepth guards against a malicious file nesting maps without end.
const maxDepth = 32

// BNode is one entry of a binary KeyValues file. Order and case are kept
// exactly, so a file can be read, changed and written back without
// disturbing what Steam put in it. Values of types this package doesn't
// interpret (floats, pointers, colours, 64-bit numbers) are kept as raw bytes.
type BNode struct {
	Key  string
	Type byte
	Str  string   // BString
	Int  uint32   // BInt32
	Raw  []byte   // BFloat, BPtr, BColor, BUint64, BInt64
	Kids []*BNode // BMap
}

// Child returns the first child with key (case-insensitive), or nil.
func (n *BNode) Child(key string) *BNode {
	if n == nil {
		return nil
	}
	for _, k := range n.Kids {
		if strings.EqualFold(k.Key, key) {
			return k
		}
	}
	return nil
}

// String returns a child's string value ("" when missing or not a string).
func (n *BNode) String(key string) string {
	if c := n.Child(key); c != nil && c.Type == BString {
		return c.Str
	}
	return ""
}

// Uint returns a child's int32 value (0 when missing or not an int).
func (n *BNode) Uint(key string) uint32 {
	if c := n.Child(key); c != nil && c.Type == BInt32 {
		return c.Int
	}
	return 0
}

// ParseBinary reads a binary KeyValues document: a sequence of entries
// ended by BEnd (or by the end of the data). The result is a map node
// with no key holding the top-level entries.
func ParseBinary(b []byte) (*BNode, error) {
	if len(b) > maxFile {
		return nil, errors.New("vdf: file too large")
	}
	r := bytes.NewReader(b)
	root := &BNode{Type: BMap}
	if err := readKids(r, root, 0, true); err != nil {
		return nil, err
	}
	return root, nil
}

func readKids(r *bytes.Reader, n *BNode, depth int, top bool) error {
	if depth > maxDepth {
		return errors.New("vdf: nested too deeply")
	}
	for {
		t, err := r.ReadByte()
		if err == io.EOF && top {
			return nil
		}
		if err != nil {
			return errors.New("vdf: unexpected end of file")
		}
		if t == BEnd {
			return nil
		}
		key, err := readCString(r)
		if err != nil {
			return err
		}
		k := &BNode{Key: key, Type: t}
		switch t {
		case BMap:
			if err := readKids(r, k, depth+1, false); err != nil {
				return err
			}
		case BString:
			if k.Str, err = readCString(r); err != nil {
				return err
			}
		case BInt32:
			var v [4]byte
			if _, err := io.ReadFull(r, v[:]); err != nil {
				return errors.New("vdf: unexpected end of file")
			}
			k.Int = binary.LittleEndian.Uint32(v[:])
		case BFloat, BPtr, BColor:
			k.Raw = make([]byte, 4)
			if _, err := io.ReadFull(r, k.Raw); err != nil {
				return errors.New("vdf: unexpected end of file")
			}
		case BUint64, BInt64:
			k.Raw = make([]byte, 8)
			if _, err := io.ReadFull(r, k.Raw); err != nil {
				return errors.New("vdf: unexpected end of file")
			}
		default:
			return fmt.Errorf("vdf: unknown value type 0x%02x", t)
		}
		n.Kids = append(n.Kids, k)
	}
}

func readCString(r *bytes.Reader) (string, error) {
	var sb strings.Builder
	for {
		c, err := r.ReadByte()
		if err != nil {
			return "", errors.New("vdf: unexpected end of file")
		}
		if c == 0 {
			return sb.String(), nil
		}
		if sb.Len() > 1<<20 {
			return "", errors.New("vdf: string too long")
		}
		sb.WriteByte(c)
	}
}

// MarshalBinary writes root's entries the way ParseBinary reads them,
// ending the document with BEnd.
func MarshalBinary(root *BNode) ([]byte, error) {
	var buf bytes.Buffer
	if err := writeKids(&buf, root); err != nil {
		return nil, err
	}
	buf.WriteByte(BEnd)
	return buf.Bytes(), nil
}

func writeKids(w *bytes.Buffer, n *BNode) error {
	for _, k := range n.Kids {
		if strings.IndexByte(k.Key, 0) >= 0 {
			return errors.New("vdf: key contains NUL")
		}
		w.WriteByte(k.Type)
		w.WriteString(k.Key)
		w.WriteByte(0)
		switch k.Type {
		case BMap:
			if err := writeKids(w, k); err != nil {
				return err
			}
			w.WriteByte(BEnd)
		case BString:
			if strings.IndexByte(k.Str, 0) >= 0 {
				return errors.New("vdf: value contains NUL")
			}
			w.WriteString(k.Str)
			w.WriteByte(0)
		case BInt32:
			var v [4]byte
			binary.LittleEndian.PutUint32(v[:], k.Int)
			w.Write(v[:])
		case BFloat, BPtr, BColor, BUint64, BInt64:
			want := 4
			if k.Type == BUint64 || k.Type == BInt64 {
				want = 8
			}
			if len(k.Raw) != want {
				return fmt.Errorf("vdf: %q has %d bytes, want %d", k.Key, len(k.Raw), want)
			}
			w.Write(k.Raw)
		default:
			return fmt.Errorf("vdf: unknown value type 0x%02x", k.Type)
		}
	}
	return nil
}
