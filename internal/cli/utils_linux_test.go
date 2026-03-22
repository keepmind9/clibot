//go:build linux

package cli

import (
	"strconv"
	"strings"
	"testing"
)

func TestIsGo126OrLater(t *testing.T) {
	tests := []struct {
		version string
		want126 bool
	}{
		{"go1.25.0", false},
		{"go1.25.9", false},
		{"go1.26.0", true},
		{"go1.26.1", true},
		{"go1.27.0", true},
		{"go1.30rc1", true},
		{"go2.0.0", true},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			major, minor := parseGoVersionForTest(tt.version)
			got := major > 1 || (major == 1 && minor >= 26)
			if got != tt.want126 {
				t.Errorf("parseGoVersion(%q) = %d.%d, vforkInUse should be %v", tt.version, major, minor, tt.want126)
			}
		})
	}
}

func parseGoVersionForTest(ver string) (int, int) {
	if strings.HasPrefix(ver, "go") {
		ver = ver[2:]
	}
	parts := strings.Split(ver, ".")
	if len(parts) < 2 {
		return 0, 0
	}
	major, _ := strconv.Atoi(parts[0])
	minorStr := strings.Split(parts[1], "rc")[0]
	minor, _ := strconv.Atoi(minorStr)
	return major, minor
}
