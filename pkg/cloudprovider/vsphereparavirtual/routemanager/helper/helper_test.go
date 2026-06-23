package helper

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	testClustername = "test-cluster"
	testCIDR        = "100.96.0.0/24"
	testNodeName    = "fakeNode1"
)

func TestGetRouteName(t *testing.T) {
	name := GetRouteName(testNodeName, testCIDR, testClustername)
	expectedName := testNodeName + "-100.96.0.0-24-" + testClustername
	assert.Equal(t, name, expectedName)
}

func TestStripFamilySuffix(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// bare IPv4 name — unchanged
		{"node-1", "node-1"},
		// IPv6 name — suffix stripped
		{"node-1" + SuffixIPv6, "node-1"},
		{"my-node" + SuffixIPv6, "my-node"},
		// a name that happens to end in "-ipv4" is NOT stripped (no SuffixIPv4)
		{"node-1-ipv4", "node-1-ipv4"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := StripFamilySuffix(tt.input)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestIPFamilyLabel(t *testing.T) {
	tests := []struct {
		name     string
		cidr     string
		expected string
	}{
		{"IPv4 CIDR", "100.96.0.0/24", LabelValueIPFamilyIPv4},
		{"IPv6 CIDR", "fd00::/80", LabelValueIPFamilyIPv6},
		{"IPv4 host", "10.0.0.1/32", LabelValueIPFamilyIPv4},
		{"IPv6 host", "::1/128", LabelValueIPFamilyIPv6},
		// Malformed inputs must return "" rather than silently labelling as
		// IPv6, otherwise callers will write a wrong label onto a CR.
		{"empty string", "", ""},
		{"missing prefix", "10.0.0.1", ""},
		{"hostname", "not-a-cidr", ""},
		{"garbage", "/64", ""},
		{"only slash", "/", ""},
		{"non-numeric prefix", "10.0.0.0/foo", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IPFamilyLabel(tt.cidr)
			assert.Equal(t, tt.expected, got)
		})
	}
}
