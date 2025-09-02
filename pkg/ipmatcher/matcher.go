package ipmatcher

import (
	"net/netip"
	"sync/atomic"

	"github.com/ELLIO-Technology/ELLIO-Traefik-Middleware-Plugin/pkg/iptrie"
)

// trieData holds the trie and count together for atomic updates
type trieData struct {
	trie  *iptrie.Trie
	count int64
}

// Matcher provides thread-safe IP address matching using lock-free reads
type Matcher struct {
	data atomic.Value // holds *trieData
}

// New creates a new IP matcher
func New() *Matcher {
	m := &Matcher{}
	m.data.Store(&trieData{
		trie:  iptrie.NewTrie(),
		count: 0,
	})
	return m
}

// Contains checks if the given IP address is in the set
func (m *Matcher) Contains(ipStr string) bool {
	// Parse the IP address using netip
	addr, err := netip.ParseAddr(ipStr)
	if err != nil {
		return false
	}

	return m.ContainsAddr(addr)
}

// ContainsAddr checks if the given parsed IP address is in the set
func (m *Matcher) ContainsAddr(addr netip.Addr) bool {
	// Lock-free read via atomic.Value
	data := m.data.Load().(*trieData)

	// Single trie lookup - handles both individual IPs and CIDR blocks
	// Use ContainsUnsafe since trie is immutable once created
	return data.trie.ContainsUnsafe(addr)
}

// Update atomically replaces the IP data with new data
func (m *Matcher) Update(newTrie *iptrie.Trie, count int64) {
	// Atomic update - no locks needed
	m.data.Store(&trieData{
		trie:  newTrie,
		count: count,
	})
}

// Count returns the number of entries in the current IP set
func (m *Matcher) Count() int64 {
	// Lock-free read
	data := m.data.Load().(*trieData)
	return data.count
}
