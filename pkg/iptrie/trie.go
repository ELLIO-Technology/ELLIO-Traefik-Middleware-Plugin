package iptrie

import (
	"encoding/binary"
	"net/netip"
	"sync"
)

// TrieNode represents a node in the binary trie
type TrieNode struct {
	children [2]*TrieNode // 0 and 1 children
	isEnd    bool         // marks end of a valid prefix
	depth    uint8        // depth in the trie for optimization
}

// Trie is a binary trie for fast IP prefix lookups
type Trie struct {
	mu     sync.RWMutex
	count  int64
	rootV4 *TrieNode
	rootV6 *TrieNode
}

// NewTrie creates a new IP trie
func NewTrie() *Trie {
	return &Trie{
		rootV4: &TrieNode{depth: 0},
		rootV6: &TrieNode{depth: 0},
	}
}

// Insert adds a prefix to the trie
func (t *Trie) Insert(prefix netip.Prefix) {
	t.mu.Lock()
	defer t.mu.Unlock()

	addr := prefix.Addr()
	bits := prefix.Bits()

	// Choose root and insert
	if addr.Is4() {
		insertV4(t.rootV4, addr, bits)
	} else {
		insertV6(t.rootV6, addr, bits)
	}

	t.count++
}

// insertV4 inserts an IPv4 address/prefix into the trie
func insertV4(root *TrieNode, addr netip.Addr, prefixLen int) {
	// Convert IPv4 to uint32 for easy bit extraction
	bytes := addr.As4()
	ip := binary.BigEndian.Uint32(bytes[:])

	current := root
	for i := 0; i < prefixLen; i++ {
		// Extract bit at position i (MSB first)
		bitPos := uint(31 - i) //nolint:G115 // i ranges 0 to prefixLen-1, result always positive
		bit := (ip >> bitPos) & 1

		// Create child if needed
		if current.children[bit] == nil {
			current.children[bit] = &TrieNode{depth: uint8(i + 1)} //nolint:G115 // max depth is 32/128, fits in uint8
		}
		current = current.children[bit]
	}
	current.isEnd = true
}

// insertV6 inserts an IPv6 address/prefix into the trie
func insertV6(root *TrieNode, addr netip.Addr, prefixLen int) {
	bytes := addr.As16()

	// Process IPv6 as two uint64s for easier bit manipulation
	high := binary.BigEndian.Uint64(bytes[0:8])
	low := binary.BigEndian.Uint64(bytes[8:16])

	current := root
	for i := 0; i < prefixLen; i++ {
		var bit uint64
		if i < 64 {
			// First 64 bits from high
			bitPos := uint(63 - i) //nolint:G115 // i < 64, result always positive
			bit = (high >> bitPos) & 1
		} else {
			// Next 64 bits from low
			bitPos := uint(127 - i) //nolint:G115 // 64 <= i < 128, result always positive
			bit = (low >> bitPos) & 1
		}

		// Create child if needed
		if current.children[bit] == nil {
			current.children[bit] = &TrieNode{depth: uint8(i + 1)} //nolint:G115 // max depth is 32/128, fits in uint8
		}
		current = current.children[bit]
	}
	current.isEnd = true
}

// Contains checks if an IP address is contained in any prefix in the trie
func (t *Trie) Contains(addr netip.Addr) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if addr.Is4() {
		return containsV4(t.rootV4, addr)
	}
	return containsV6(t.rootV6, addr)
}

// containsV4 checks if an IPv4 address matches any prefix in the trie
func containsV4(root *TrieNode, addr netip.Addr) bool {
	bytes := addr.As4()
	ip := binary.BigEndian.Uint32(bytes[:])

	current := root
	// Early exit if root marks a /0 prefix
	if current.isEnd {
		return true
	}

	// Unroll first few iterations for common cases
	for i := 0; i < 8; i++ {
		bitPos := uint(31 - i) //nolint:G115 // i ranges 0-7, result always positive
		bit := (ip >> bitPos) & 1

		if current.children[bit] == nil {
			return false
		}
		current = current.children[bit]
		if current.isEnd {
			return true
		}
	}

	// Continue with remaining bits
	for i := 8; i < 32; i++ {
		bitPos := uint(31 - i) //nolint:G115 // i ranges 8-31, result always positive
		bit := (ip >> bitPos) & 1

		if current.children[bit] == nil {
			return false
		}
		current = current.children[bit]
		if current.isEnd {
			return true
		}
	}

	return false
}

// containsV6 checks if an IPv6 address matches any prefix in the trie
func containsV6(root *TrieNode, addr netip.Addr) bool {
	bytes := addr.As16()
	high := binary.BigEndian.Uint64(bytes[0:8])
	low := binary.BigEndian.Uint64(bytes[8:16])

	current := root
	// Early exit if root marks a /0 prefix
	if current.isEnd {
		return true
	}

	// Process high 64 bits
	for i := 0; i < 64; i++ {
		bitPos := uint(63 - i) //nolint:G115 // i ranges 0-63, result always positive
		bit := (high >> bitPos) & 1

		if current.children[bit] == nil {
			return false
		}
		current = current.children[bit]
		if current.isEnd {
			return true
		}
	}

	// Process low 64 bits
	for i := 64; i < 128; i++ {
		bitPos := uint(127 - i) //nolint:G115 // i ranges 64-127, result always positive
		bit := (low >> bitPos) & 1

		if current.children[bit] == nil {
			return false
		}
		current = current.children[bit]
		if current.isEnd {
			return true
		}
	}

	return false
}

// Count returns the number of prefixes in the trie
func (t *Trie) Count() int64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.count
}

// ContainsUnsafe performs a lockless lookup - ONLY use when trie is read-only
func (t *Trie) ContainsUnsafe(addr netip.Addr) bool {
	if addr.Is4() {
		return containsV4(t.rootV4, addr)
	}
	return containsV6(t.rootV6, addr)
}

// BulkLoad creates a new trie from a list of prefixes
// ASSUMES: Input data is already sorted (IPv4 first, then IPv6, both in ascending order)
func BulkLoad(prefixes []netip.Prefix) *Trie {
	// Use actual binary trie - optimized for sorted input
	t := &Trie{
		rootV4: &TrieNode{depth: 0},
		rootV6: &TrieNode{depth: 0},
		count:  int64(len(prefixes)),
	}

	// Since data is sorted, we can process sequentially without separation
	// IPv4 entries come first, then IPv6
	for _, p := range prefixes {
		addr := p.Addr()
		bits := p.Bits()

		if addr.Is4() {
			bytes := addr.As4()
			ip := binary.BigEndian.Uint32(bytes[:])
			insertV4Optimized(t.rootV4, ip, bits)
		} else if addr.Is6() {
			bytes := addr.As16()
			high := binary.BigEndian.Uint64(bytes[0:8])
			low := binary.BigEndian.Uint64(bytes[8:16])
			insertV6Optimized(t.rootV6, high, low, bits)
		}
	}

	return t
}

// insertV4Optimized inserts IPv4 with pre-converted value
func insertV4Optimized(root *TrieNode, ip uint32, prefixLen int) {
	current := root
	for i := 0; i < prefixLen; i++ {
		bitPos := uint(31 - i) //nolint:G115 // i < prefixLen <= 32, result always positive
		bit := (ip >> bitPos) & 1

		if current.children[bit] == nil {
			current.children[bit] = &TrieNode{depth: uint8(i + 1)} //nolint:G115 // max depth is 32/128, fits in uint8
		}
		current = current.children[bit]
	}
	current.isEnd = true
}

// insertV6Optimized inserts IPv6 with pre-converted values
func insertV6Optimized(root *TrieNode, high, low uint64, prefixLen int) {
	current := root
	for i := 0; i < prefixLen; i++ {
		var bit uint64
		if i < 64 {
			bitPos := uint(63 - i) //nolint:G115 // i < 64, result always positive
			bit = (high >> bitPos) & 1
		} else {
			bitPos := uint(127 - i) //nolint:G115 // 64 <= i < prefixLen <= 128, result always positive
			bit = (low >> bitPos) & 1
		}

		if current.children[bit] == nil {
			current.children[bit] = &TrieNode{depth: uint8(i + 1)} //nolint:G115 // max depth is 32/128, fits in uint8
		}
		current = current.children[bit]
	}
	current.isEnd = true
}
