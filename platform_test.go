package main

import (
	"runtime"
	"testing"
)

func TestPlatformDetectionUsesGOOS(t *testing.T) {
	if isMacOS() != (runtime.GOOS == "darwin") {
		t.Errorf("isMacOS()=%v but GOOS=%q", isMacOS(), runtime.GOOS)
	}
	if isLinux() != (runtime.GOOS == "linux") {
		t.Errorf("isLinux()=%v but GOOS=%q", isLinux(), runtime.GOOS)
	}
}

func TestDetectPlatformSmoke(t *testing.T) {
	p := DetectPlatform()
	if p == "" {
		t.Error("Platform should not be empty")
	}
	t.Logf("Detected platform: %s", p)
}

func TestPlatformFlags(t *testing.T) {
	cases := []struct {
		p       Platform
		isWSL   bool
		isNixOS bool
	}{
		{PlatformMacOS, false, false},
		{PlatformLinux, false, false},
		{PlatformNixOS, false, true},
		{PlatformNixOSWSL, true, true},
		{PlatformGenericLinux, false, false},
	}

	for _, c := range cases {
		if c.p.IsWSL() != c.isWSL {
			t.Errorf("Expected IsWSL() to be %v for platform %s", c.isWSL, c.p)
		}
		if c.p.IsNixOS() != c.isNixOS {
			t.Errorf("Expected IsNixOS() to be %v for platform %s", c.isNixOS, c.p)
		}
	}
}
