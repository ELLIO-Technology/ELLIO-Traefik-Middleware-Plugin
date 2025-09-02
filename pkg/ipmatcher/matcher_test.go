package ipmatcher

import (
	"net/netip"
	"testing"

	"github.com/ELLIO-Technology/ELLIO-Traefik-Middleware-Plugin/pkg/iptrie"
)

func TestNewMatcher(t *testing.T) {
	matcher := New()
	if matcher == nil {
		t.Fatal("New returned nil")
	}
	if matcher.Count() != 0 {
		t.Errorf("expected count 0, got %d", matcher.Count())
	}
}

func TestContains(t *testing.T) {
	matcher := New()

	// Create a new trie with some IPs
	trie := iptrie.NewTrie()
	trie.Insert(netip.MustParsePrefix("192.168.1.0/24"))
	trie.Insert(netip.MustParsePrefix("10.0.0.0/8"))
	trie.Insert(netip.MustParsePrefix("2001:db8::/32"))

	matcher.Update(trie, 3)

	tests := []struct {
		ip       string
		expected bool
	}{
		{"192.168.1.1", true},
		{"192.168.1.255", true},
		{"192.168.2.1", false},
		{"10.1.2.3", true},
		{"10.255.255.255", true},
		{"11.0.0.0", false},
		{"8.8.8.8", false},
		{"2001:db8::1", true},
		{"2001:db9::1", false},
		{"invalid", false}, // Invalid IP should return false
	}

	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			result := matcher.Contains(tt.ip)
			if result != tt.expected {
				t.Errorf("Contains(%s) = %v, expected %v", tt.ip, result, tt.expected)
			}
		})
	}
}

func TestContainsAddr(t *testing.T) {
	matcher := New()

	// Create a new trie with some IPs
	trie := iptrie.NewTrie()
	trie.Insert(netip.MustParsePrefix("192.168.0.0/16"))
	trie.Insert(netip.MustParsePrefix("fc00::/7"))

	matcher.Update(trie, 2)

	tests := []struct {
		ip       string
		expected bool
	}{
		{"192.168.1.1", true},
		{"192.168.255.255", true},
		{"192.169.0.0", false},
		{"fc00::1", true},
		{"fd00::1", true},
		{"fe00::1", false},
	}

	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			addr := netip.MustParseAddr(tt.ip)
			result := matcher.ContainsAddr(addr)
			if result != tt.expected {
				t.Errorf("ContainsAddr(%s) = %v, expected %v", tt.ip, result, tt.expected)
			}
		})
	}
}

func TestUpdate(t *testing.T) {
	matcher := New()

	// Initial state
	if matcher.Count() != 0 {
		t.Errorf("expected initial count 0, got %d", matcher.Count())
	}

	// First update
	trie1 := iptrie.NewTrie()
	trie1.Insert(netip.MustParsePrefix("10.0.0.0/8"))
	matcher.Update(trie1, 1)

	if matcher.Count() != 1 {
		t.Errorf("expected count 1, got %d", matcher.Count())
	}
	if !matcher.Contains("10.1.1.1") {
		t.Error("expected 10.1.1.1 to be contained after first update")
	}

	// Second update (replace)
	trie2 := iptrie.NewTrie()
	trie2.Insert(netip.MustParsePrefix("192.168.0.0/16"))
	trie2.Insert(netip.MustParsePrefix("172.16.0.0/12"))
	matcher.Update(trie2, 2)

	if matcher.Count() != 2 {
		t.Errorf("expected count 2, got %d", matcher.Count())
	}
	if matcher.Contains("10.1.1.1") {
		t.Error("10.1.1.1 should not be contained after second update")
	}
	if !matcher.Contains("192.168.1.1") {
		t.Error("expected 192.168.1.1 to be contained after second update")
	}
	if !matcher.Contains("172.16.1.1") {
		t.Error("expected 172.16.1.1 to be contained after second update")
	}
}

func TestCount(t *testing.T) {
	matcher := New()

	// Check initial count
	if matcher.Count() != 0 {
		t.Errorf("expected initial count 0, got %d", matcher.Count())
	}

	// Update with specific count
	trie := iptrie.NewTrie()
	for i := 0; i < 5; i++ {
		prefix := netip.MustParsePrefix("10.0.0.0/8")
		trie.Insert(prefix)
	}

	matcher.Update(trie, 42) // Use explicit count

	if matcher.Count() != 42 {
		t.Errorf("expected count 42, got %d", matcher.Count())
	}
}

func TestEmptyMatcher(t *testing.T) {
	matcher := New()

	// Test with various IPs - empty matcher should not contain any
	testIPs := []string{
		"192.168.1.1",
		"10.0.0.1",
		"8.8.8.8",
		"2001:db8::1",
		"::1",
	}

	for _, ip := range testIPs {
		if matcher.Contains(ip) {
			t.Errorf("empty matcher should not contain %s", ip)
		}
	}
}

func TestInvalidIPs(t *testing.T) {
	matcher := New()

	// Add some valid prefixes
	trie := iptrie.NewTrie()
	trie.Insert(netip.MustParsePrefix("192.168.0.0/16"))
	matcher.Update(trie, 1)

	// Test invalid IPs - should all return false
	invalidIPs := []string{
		"",
		"invalid",
		"256.256.256.256",
		"192.168.1",
		"192.168.1.1.1",
		"gggg::1",
		"not-an-ip",
	}

	for _, ip := range invalidIPs {
		t.Run(ip, func(t *testing.T) {
			if matcher.Contains(ip) {
				t.Errorf("Contains(%q) should return false for invalid IP", ip)
			}
		})
	}
}

func TestMatcherConcurrentAccess(t *testing.T) {
	matcher := New()

	// Initial setup
	trie := iptrie.NewTrie()
	trie.Insert(netip.MustParsePrefix("10.0.0.0/8"))
	trie.Insert(netip.MustParsePrefix("192.168.0.0/16"))
	matcher.Update(trie, 2)

	done := make(chan bool)

	// Concurrent updates
	go func() {
		for i := 0; i < 100; i++ {
			newTrie := iptrie.NewTrie()
			newTrie.Insert(netip.MustParsePrefix("172.16.0.0/12"))
			matcher.Update(newTrie, 1)
		}
		done <- true
	}()

	// Concurrent Contains checks
	go func() {
		for i := 0; i < 100; i++ {
			matcher.Contains("10.1.1.1")
			matcher.Contains("8.8.8.8")
			matcher.Contains("192.168.1.1")
		}
		done <- true
	}()

	// Concurrent ContainsAddr checks
	go func() {
		addr1 := netip.MustParseAddr("10.1.1.1")
		addr2 := netip.MustParseAddr("8.8.8.8")
		for i := 0; i < 100; i++ {
			matcher.ContainsAddr(addr1)
			matcher.ContainsAddr(addr2)
		}
		done <- true
	}()

	// Concurrent Count checks
	go func() {
		for i := 0; i < 100; i++ {
			_ = matcher.Count()
		}
		done <- true
	}()

	// Wait for all goroutines
	for i := 0; i < 4; i++ {
		<-done
	}

	// Verify matcher still works
	count := matcher.Count()
	if count < 0 {
		t.Errorf("count should not be negative: %d", count)
	}
}

func TestMatcherMixedIPs(t *testing.T) {
	matcher := New()

	trie := iptrie.NewTrie()
	// Add IPv4 prefixes
	trie.Insert(netip.MustParsePrefix("192.168.0.0/16"))
	trie.Insert(netip.MustParsePrefix("10.0.0.0/8"))
	// Add IPv6 prefixes
	trie.Insert(netip.MustParsePrefix("2001:db8::/32"))
	trie.Insert(netip.MustParsePrefix("fc00::/7"))

	matcher.Update(trie, 4)

	// Test IPv4
	ipv4Tests := map[string]bool{
		"192.168.1.1": true,
		"10.1.2.3":    true,
		"8.8.8.8":     false,
		"172.16.0.1":  false,
	}

	for ip, expected := range ipv4Tests {
		if result := matcher.Contains(ip); result != expected {
			t.Errorf("Contains(%s) = %v, expected %v", ip, result, expected)
		}
	}

	// Test IPv6
	ipv6Tests := map[string]bool{
		"2001:db8::1":      true,
		"2001:db8:ffff::1": true,
		"fc00::1":          true,
		"fd00::1":          true,
		"2001:db9::1":      false,
		"fe00::1":          false,
	}

	for ip, expected := range ipv6Tests {
		if result := matcher.Contains(ip); result != expected {
			t.Errorf("Contains(%s) = %v, expected %v", ip, result, expected)
		}
	}
}

func BenchmarkContains(b *testing.B) {
	matcher := New()

	// Add some prefixes
	trie := iptrie.NewTrie()
	trie.Insert(netip.MustParsePrefix("10.0.0.0/8"))
	trie.Insert(netip.MustParsePrefix("192.168.0.0/16"))
	trie.Insert(netip.MustParsePrefix("172.16.0.0/12"))
	matcher.Update(trie, 3)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matcher.Contains("10.1.2.3")
	}
}

func BenchmarkContainsMiss(b *testing.B) {
	matcher := New()

	// Add some prefixes
	trie := iptrie.NewTrie()
	trie.Insert(netip.MustParsePrefix("10.0.0.0/8"))
	trie.Insert(netip.MustParsePrefix("192.168.0.0/16"))
	trie.Insert(netip.MustParsePrefix("172.16.0.0/12"))
	matcher.Update(trie, 3)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matcher.Contains("8.8.8.8")
	}
}

func BenchmarkContainsAddr(b *testing.B) {
	matcher := New()

	// Add some prefixes
	trie := iptrie.NewTrie()
	trie.Insert(netip.MustParsePrefix("10.0.0.0/8"))
	trie.Insert(netip.MustParsePrefix("192.168.0.0/16"))
	trie.Insert(netip.MustParsePrefix("172.16.0.0/12"))
	matcher.Update(trie, 3)

	addr := netip.MustParseAddr("10.1.2.3")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matcher.ContainsAddr(addr)
	}
}
