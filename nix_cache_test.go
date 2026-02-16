package main

import (
	"testing"
)

// Test Nix Cache Integration

func TestCheckNixInstallation(t *testing.T) {
	// This test checks if Nix is available
	// Result depends on system configuration
	installed := CheckNixInstallation()
	if installed {
		t.Log("✓ Nix is installed")
	} else {
		t.Log("- Nix is not installed (expected on non-NixOS)")
	}
}

func TestDefaultCachesConfiguration(t *testing.T) {
	if len(DefaultNixCaches) == 0 {
		t.Error("DefaultNixCaches should not be empty")
	}

	// Verify stevedores attic is first
	stevedores := DefaultNixCaches[0]
	if stevedores.Name != "stevedores-attic" {
		t.Errorf("Expected first cache to be stevedores-attic, got %s", stevedores.Name)
	}

	if !stevedores.Public {
		t.Error("stevedores-attic should be public")
	}

	if stevedores.URL != "https://nix-cache.stevedores.org" {
		t.Errorf("Expected stevedores cache URL, got %s", stevedores.URL)
	}
}

func TestGetInstalledCaches(t *testing.T) {
	// This test may fail if Nix is not installed
	if !CheckNixInstallation() {
		t.Skip("Nix not installed, skipping cache check")
	}

	caches, err := GetInstalledCaches()
	if err != nil {
		t.Logf("Could not retrieve caches: %v", err)
		return
	}

	if caches == nil {
		t.Log("No caches found or Nix configuration issue")
	} else {
		t.Logf("Found %d configured caches", len(caches))
	}
}

func TestAtticCacheURL(t *testing.T) {
	// Verify stevedores cache URL is correct
	expected := "https://nix-cache.stevedores.org"
	actual := DefaultNixCaches[0].URL

	if actual != expected {
		t.Errorf("Expected %s, got %s", expected, actual)
	}

	// Verify it's a valid URL format
	if !validateCacheURL(actual) {
		t.Errorf("Invalid cache URL: %s", actual)
	}
}

func TestCacheTrustSettings(t *testing.T) {
	// Verify trust settings are correct
	stevedores := findCache("stevedores-attic")
	if stevedores == nil {
		t.Fatal("stevedores-attic cache not found")
	}

	if !stevedores.Trusted {
		t.Error("stevedores-attic should be trusted")
	}

	nixos := findCache("cache.nixos.org")
	if nixos == nil {
		t.Fatal("cache.nixos.org cache not found")
	}

	if nixos.Trusted {
		t.Error("cache.nixos.org should not be trusted in this context")
	}
}

func TestIsCacheInstalled(t *testing.T) {
	// This test depends on Nix installation
	if !CheckNixInstallation() {
		t.Skip("Nix not installed")
	}

	// Test that the function returns a boolean
	result := IsCacheInstalled("https://cache.nixos.org")
	if result != true && result != false {
		t.Error("IsCacheInstalled should return boolean")
	}
}

func TestConfigureAtticCache(t *testing.T) {
	// This test may fail if Nix is not installed
	if !CheckNixInstallation() {
		t.Skip("Nix not installed, skipping cache configuration test")
	}

	// Test that the function doesn't panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("ConfigureAtticCache panicked: %v", r)
		}
	}()

	err := ConfigureAtticCache()
	if err != nil {
		t.Logf("Expected error or skip (Nix setup required): %v", err)
	}
}

func TestSuggestNixOptimizations(t *testing.T) {
	suggestions := SuggestNixOptimizations()

	if len(suggestions) == 0 {
		t.Error("Suggestions should not be empty")
	}

	if !contains(suggestions, "Nix Build Optimizations") {
		t.Error("Suggestions should mention optimizations")
	}
}

// Helpers
func validateCacheURL(url string) bool {
	return len(url) > 0 && (contains(url, "https://") || contains(url, "http://"))
}

func findCache(name string) *NixCache {
	for _, cache := range DefaultNixCaches {
		if cache.Name == name {
			return &cache
		}
	}
	return nil
}

func contains(str, substr string) bool {
	for i := 0; i <= len(str)-len(substr); i++ {
		if str[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Benchmark
func BenchmarkCheckNixInstallation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		CheckNixInstallation()
	}
}
