package iptrie

import (
	"net/netip"
	"testing"
)

func TestNewTrie(t *testing.T) {
	trie := NewTrie()
	if trie == nil {
		t.Fatal("NewTrie returned nil")
	}
	if trie.rootV4 == nil {
		t.Fatal("rootV4 is nil")
	}
	if trie.rootV6 == nil {
		t.Fatal("rootV6 is nil")
	}
	if trie.Count() != 0 {
		t.Errorf("expected count 0, got %d", trie.Count())
	}
}

func TestInsertAndContainsIPv4(t *testing.T) {
	tests := []struct {
		name        string
		insertCIDRs []string
		checkIPs    map[string]bool // IP -> expected result
	}{
		{
			name:        "single /32 address",
			insertCIDRs: []string{"192.168.1.1/32"},
			checkIPs: map[string]bool{
				"192.168.1.1": true,
				"192.168.1.2": false,
				"192.168.2.1": false,
			},
		},
		{
			name:        "single /24 network",
			insertCIDRs: []string{"192.168.1.0/24"},
			checkIPs: map[string]bool{
				"192.168.1.0":   true,
				"192.168.1.1":   true,
				"192.168.1.255": true,
				"192.168.2.0":   false,
				"192.167.1.1":   false,
			},
		},
		{
			name:        "multiple networks",
			insertCIDRs: []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"},
			checkIPs: map[string]bool{
				"10.0.0.1":        true,
				"10.255.255.255":  true,
				"172.16.0.1":      true,
				"172.31.255.255":  true,
				"172.32.0.0":      false,
				"192.168.0.1":     true,
				"192.168.255.255": true,
				"192.169.0.0":     false,
				"8.8.8.8":         false,
			},
		},
		{
			name:        "overlapping networks",
			insertCIDRs: []string{"10.0.0.0/8", "10.1.0.0/16", "10.1.2.0/24"},
			checkIPs: map[string]bool{
				"10.0.0.1": true, // Matches /8
				"10.1.0.1": true, // Matches /8 and /16
				"10.1.2.3": true, // Matches all three
				"10.2.0.0": true, // Matches /8
				"11.0.0.0": false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trie := NewTrie()

			// Insert CIDRs
			for _, cidr := range tt.insertCIDRs {
				prefix, err := netip.ParsePrefix(cidr)
				if err != nil {
					t.Fatalf("failed to parse CIDR %s: %v", cidr, err)
				}
				trie.Insert(prefix)
			}

			// Check count
			if trie.Count() != int64(len(tt.insertCIDRs)) {
				t.Errorf("expected count %d, got %d", len(tt.insertCIDRs), trie.Count())
			}

			// Test Contains
			for ip, expected := range tt.checkIPs {
				addr, err := netip.ParseAddr(ip)
				if err != nil {
					t.Fatalf("failed to parse IP %s: %v", ip, err)
				}
				result := trie.Contains(addr)
				if result != expected {
					t.Errorf("Contains(%s) = %v, expected %v", ip, result, expected)
				}
			}
		})
	}
}

func TestInsertAndContainsIPv6(t *testing.T) {
	tests := []struct {
		name        string
		insertCIDRs []string
		checkIPs    map[string]bool
	}{
		{
			name:        "single /128 address",
			insertCIDRs: []string{"2001:db8::1/128"},
			checkIPs: map[string]bool{
				"2001:db8::1":   true,
				"2001:db8::2":   false,
				"2001:db8:1::1": false,
			},
		},
		{
			name:        "single /64 network",
			insertCIDRs: []string{"2001:db8::/64"},
			checkIPs: map[string]bool{
				"2001:db8::1":     true,
				"2001:db8::ffff":  true,
				"2001:db8:0:1::1": false,
				"2001:db9::1":     false,
			},
		},
		{
			name:        "multiple networks",
			insertCIDRs: []string{"2001:db8::/32", "fc00::/7"},
			checkIPs: map[string]bool{
				"2001:db8::1":      true,
				"2001:db8:ffff::1": true,
				"2001:db9::1":      false,
				"fc00::1":          true,
				"fd00::1":          true,
				"fe00::1":          false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trie := NewTrie()

			// Insert CIDRs
			for _, cidr := range tt.insertCIDRs {
				prefix, err := netip.ParsePrefix(cidr)
				if err != nil {
					t.Fatalf("failed to parse CIDR %s: %v", cidr, err)
				}
				trie.Insert(prefix)
			}

			// Test Contains
			for ip, expected := range tt.checkIPs {
				addr, err := netip.ParseAddr(ip)
				if err != nil {
					t.Fatalf("failed to parse IP %s: %v", ip, err)
				}
				result := trie.Contains(addr)
				if result != expected {
					t.Errorf("Contains(%s) = %v, expected %v", ip, result, expected)
				}
			}
		})
	}
}

func TestTrieMixedIPs(t *testing.T) {
	trie := NewTrie()

	// Insert both IPv4 and IPv6 prefixes
	prefixes := []string{
		"192.168.0.0/16",
		"10.0.0.0/8",
		"2001:db8::/32",
		"fc00::/7",
	}

	for _, p := range prefixes {
		prefix, _ := netip.ParsePrefix(p)
		trie.Insert(prefix)
	}

	// Test IPv4
	ipv4Tests := map[string]bool{
		"192.168.1.1": true,
		"10.1.2.3":    true,
		"8.8.8.8":     false,
	}

	for ip, expected := range ipv4Tests {
		addr, _ := netip.ParseAddr(ip)
		if result := trie.Contains(addr); result != expected {
			t.Errorf("Contains(%s) = %v, expected %v", ip, result, expected)
		}
	}

	// Test IPv6
	ipv6Tests := map[string]bool{
		"2001:db8::1": true,
		"fc00::1":     true,
		"2001:db9::1": false,
	}

	for ip, expected := range ipv6Tests {
		addr, _ := netip.ParseAddr(ip)
		if result := trie.Contains(addr); result != expected {
			t.Errorf("Contains(%s) = %v, expected %v", ip, result, expected)
		}
	}

	// Check count
	if trie.Count() != int64(len(prefixes)) {
		t.Errorf("expected count %d, got %d", len(prefixes), trie.Count())
	}
}

func TestTrieReinitialization(t *testing.T) {
	trie := NewTrie()

	// Insert some prefixes
	prefixes := []string{
		"192.168.0.0/16",
		"10.0.0.0/8",
		"2001:db8::/32",
	}

	for _, p := range prefixes {
		prefix, _ := netip.ParsePrefix(p)
		trie.Insert(prefix)
	}

	// Verify count
	if trie.Count() != 3 {
		t.Errorf("expected count 3, got %d", trie.Count())
	}

	// Create a new trie (simulating reinit)
	trie = NewTrie()

	// Verify it's empty
	if trie.Count() != 0 {
		t.Errorf("expected count 0 for new trie, got %d", trie.Count())
	}

	// Verify no IPs are contained
	testIPs := []string{"192.168.1.1", "10.1.1.1", "2001:db8::1"}
	for _, ip := range testIPs {
		addr, _ := netip.ParseAddr(ip)
		if trie.Contains(addr) {
			t.Errorf("new trie contains %s", ip)
		}
	}
}

func TestTrieConcurrentAccess(t *testing.T) {
	trie := NewTrie()

	// Run concurrent inserts and lookups
	done := make(chan bool)

	// Goroutine for inserting
	go func() {
		for i := 0; i < 100; i++ {
			prefix, _ := netip.ParsePrefix("10.0.0.0/8")
			trie.Insert(prefix)
			prefix, _ = netip.ParsePrefix("192.168.0.0/16")
			trie.Insert(prefix)
		}
		done <- true
	}()

	// Goroutine for checking contains
	go func() {
		for i := 0; i < 100; i++ {
			addr, _ := netip.ParseAddr("10.1.1.1")
			trie.Contains(addr)
			addr, _ = netip.ParseAddr("8.8.8.8")
			trie.Contains(addr)
		}
		done <- true
	}()

	// Goroutine for getting count
	go func() {
		for i := 0; i < 100; i++ {
			_ = trie.Count()
		}
		done <- true
	}()

	// Wait for all goroutines
	for i := 0; i < 3; i++ {
		<-done
	}

	// Basic verification that trie still works
	addr, _ := netip.ParseAddr("10.1.1.1")
	if !trie.Contains(addr) {
		t.Error("trie should contain 10.1.1.1")
	}
}

func BenchmarkInsertIPv4(b *testing.B) {
	trie := NewTrie()
	prefix, _ := netip.ParsePrefix("192.168.1.0/24")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		trie.Insert(prefix)
	}
}

func BenchmarkContainsIPv4(b *testing.B) {
	trie := NewTrie()
	// Insert some prefixes
	for i := 0; i < 256; i++ {
		prefix, _ := netip.ParsePrefix("10.0.0.0/8")
		trie.Insert(prefix)
	}

	addr, _ := netip.ParseAddr("10.1.2.3")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		trie.Contains(addr)
	}
}

func BenchmarkContainsIPv4Miss(b *testing.B) {
	trie := NewTrie()
	// Insert some prefixes
	prefix, _ := netip.ParsePrefix("10.0.0.0/8")
	trie.Insert(prefix)

	addr, _ := netip.ParseAddr("192.168.1.1")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		trie.Contains(addr)
	}
}
