package iptrie

import (
	"encoding/binary"
	"errors"
	"io"
	"time"

	"github.com/ELLIO-Technology/ELLIO-Traefik-Middleware-Plugin/pkg/logger"
)

const (
	// MagicHeader identifies ELLIO pre-computed trie format
	MagicHeader = "ELLIOTRIE"
	// FormatVersion of the trie format
	FormatVersion uint16 = 2
)

var (
	// ErrInvalidMagic indicates the file doesn't have the ELLIOTRIE header
	ErrInvalidMagic = errors.New("invalid magic header, not an ELLIOTRIE format file")
	// ErrUnsupportedVersion indicates an unsupported format version
	ErrUnsupportedVersion = errors.New("unsupported ELLIOTRIE format version")
)

// TrieHeader represents the pre-computed trie file header
type TrieHeader struct {
	Magic      [9]byte
	Version    uint16
	Flags      uint8
	TotalNodes uint32
	IPv4Root   uint32 // Index of IPv4 root node, 0xFFFFFFFF if none
	IPv6Root   uint32 // Index of IPv6 root node, 0xFFFFFFFF if none
}

// SerializedNode represents a node in the serialized trie format
type SerializedNode struct {
	LeftChild  uint32 // Index of left child, 0xFFFFFFFF if none
	RightChild uint32 // Index of right child, 0xFFFFFFFF if none
	Flags      uint8  // Bit 0: isEnd, Bits 1-7: depth
}

// LoadBinaryTrie loads a pre-computed trie from ELLIOTRIE format
func LoadBinaryTrie(r io.Reader) (*Trie, int64, error) {
	return LoadPrecomputedTrie(r)
}

// LoadPrecomputedTrie loads a pre-computed trie structure from binary format
func LoadPrecomputedTrie(r io.Reader) (*Trie, int64, error) {
	start := time.Now()

	// Read header
	var header TrieHeader
	if err := binary.Read(r, binary.BigEndian, &header); err != nil {
		return nil, 0, err
	}

	// Validate magic
	if string(header.Magic[:]) != MagicHeader {
		return nil, 0, ErrInvalidMagic
	}

	// Validate version
	if header.Version != FormatVersion {
		return nil, 0, ErrUnsupportedVersion
	}

	// Read all serialized nodes at once
	serializedNodes := make([]SerializedNode, header.TotalNodes)
	if err := binary.Read(r, binary.BigEndian, &serializedNodes); err != nil {
		return nil, 0, err
	}

	// Allocate all trie nodes in a single slice - this is THE key optimization
	nodes := make([]TrieNode, header.TotalNodes)

	// Reconstruct the trie by setting up pointers
	for i := uint32(0); i < header.TotalNodes; i++ {
		sNode := &serializedNodes[i]
		node := &nodes[i]

		// Set children pointers
		if sNode.LeftChild != 0xFFFFFFFF {
			node.children[0] = &nodes[sNode.LeftChild]
		}
		if sNode.RightChild != 0xFFFFFFFF {
			node.children[1] = &nodes[sNode.RightChild]
		}

		// Set flags
		node.isEnd = (sNode.Flags & 0x01) != 0
		node.depth = sNode.Flags >> 1
	}

	// Create the trie structure with pre-built roots
	trie := &Trie{
		count: int64(header.TotalNodes), // This is an approximation
	}

	// Set root pointers
	if header.IPv4Root != 0xFFFFFFFF {
		trie.rootV4 = &nodes[header.IPv4Root]
	} else {
		trie.rootV4 = &TrieNode{depth: 0}
	}

	if header.IPv6Root != 0xFFFFFFFF {
		trie.rootV6 = &nodes[header.IPv6Root]
	} else {
		trie.rootV6 = &TrieNode{depth: 0}
	}

	duration := time.Since(start)
	logger.Infof("Loaded pre-computed trie: %d nodes in %v", header.TotalNodes, duration)

	// Return approximation of prefix count (we don't have exact count in this format)
	// Could be enhanced by having backend send actual prefix count in header
	return trie, int64(header.TotalNodes / 7), nil // Rough estimate: ~7 nodes per prefix
}
