package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// NixCache represents a Nix binary cache configuration
type NixCache struct {
	Name    string // Display name
	URL     string // Cache URL (e.g., https://nix-cache.stevedores.org)
	Public  bool   // Whether to add to public binary caches
	Trusted bool   // Whether to trust the cache
}

// DefaultNixCaches are recommended caches for stevedores-org ecosystem
var DefaultNixCaches = []NixCache{
	{
		Name:    "stevedores-attic",
		URL:     "https://nix-cache.stevedores.org",
		Public:  true,
		Trusted: true,
	},
	{
		Name:    "cache.nixos.org",
		URL:     "https://cache.nixos.org",
		Public:  true,
		Trusted: false,
	},
}

// CheckNixInstallation checks if Nix is installed
func CheckNixInstallation() bool {
	cmd := exec.Command("nix", "--version")
	return cmd.Run() == nil
}

// GetInstalledCaches returns list of currently configured Nix substituters
// by parsing nix.conf files (user and system level).
func GetInstalledCaches() ([]string, error) {
	var caches []string

	// Check user-level config
	homeDir, err := os.UserHomeDir()
	if err == nil {
		userConf := filepath.Join(homeDir, ".config", "nix", "nix.conf")
		caches = append(caches, parseSubstitutersFromConf(userConf)...)
	}

	// Check system-level config
	caches = append(caches, parseSubstitutersFromConf("/etc/nix/nix.conf")...)

	return caches, nil
}

// parseSubstitutersFromConf extracts substituter URLs from a nix.conf file.
func parseSubstitutersFromConf(confPath string) []string {
	data, err := os.ReadFile(confPath)
	if err != nil {
		return nil
	}

	var caches []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			continue
		}
		// Match "substituters = ..." or "extra-substituters = ..."
		for _, key := range []string{"substituters", "extra-substituters"} {
			if strings.HasPrefix(line, key) {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					for _, url := range strings.Fields(strings.TrimSpace(parts[1])) {
						caches = append(caches, url)
					}
				}
			}
		}
	}
	return caches
}

// IsCacheInstalled checks if a specific cache is already configured
func IsCacheInstalled(cacheURL string) bool {
	installed, err := GetInstalledCaches()
	if err != nil {
		return false
	}

	for _, cache := range installed {
		if strings.Contains(cache, cacheURL) || strings.Contains(cacheURL, cache) {
			return true
		}
	}

	return false
}

// AddNixCache adds a binary cache to Nix configuration
func AddNixCache(cache NixCache) error {
	// Check if Nix is installed
	if !CheckNixInstallation() {
		return fmt.Errorf("nix not installed")
	}

	// Skip if already installed
	if IsCacheInstalled(cache.URL) {
		return nil
	}

	// Build nix.conf addition
	cacheEntry := fmt.Sprintf("extra-substituters = %s\n", cache.URL)
	if cache.Trusted {
		cacheEntry += fmt.Sprintf("trusted-public-keys = %s-1:key\n", cache.Name)
	}

	// Write to ~/.config/nix/nix.conf or /etc/nix/nix.conf
	// This requires elevated privileges for system-wide installation
	// For user-level, add to ~/.config/nix/nix.conf

	warnf("To add cache manually, add to ~/.config/nix/nix.conf:\n")
	warnf("  extra-substituters = %s\n", cache.URL)

	return nil
}

// ConfigureAtticCache specifically configures the stevedores attic cache
func ConfigureAtticCache() error {
	if !CheckNixInstallation() {
		warnf("Nix not installed. Skipping attic cache configuration.\n")
		warnf("To enable Nix binary caching, install Nix: https://nixos.org/download.html\n")
		return fmt.Errorf("nix not installed")
	}

	atticCache := DefaultNixCaches[0] // stevedores-attic
	if IsCacheInstalled(atticCache.URL) {
		successf("✅ Attic cache %s already configured\n", atticCache.URL)
		return nil
	}

	printf("📦 Configuring Nix binary cache: %s\n", atticCache.URL)

	// Attempt to add cache
	if err := AddNixCache(atticCache); err != nil {
		return err
	}

	printf("💡 To complete setup, add to ~/.config/nix/nix.conf:\n")
	printf("   extra-substituters = %s\n", atticCache.URL)
	printf("   trusted-public-keys = stevedores-attic-1:your-public-key\n\n")

	return nil
}

// SuggestNixOptimizations provides recommendations for Nix builds
func SuggestNixOptimizations() string {
	var suggestions strings.Builder

	suggestions.WriteString("\n💡 Nix Build Optimizations:\n")

	if !CheckNixInstallation() {
		return suggestions.String()
	}

	suggestions.WriteString("  - Add attic cache for faster builds\n")
	suggestions.WriteString("  - Use direnv for automatic Nix shell environment\n")
	suggestions.WriteString("  - Enable Nix daemon for parallel builds\n")
	suggestions.WriteString("  - Configure garbage collection schedule\n")
	suggestions.WriteString("  - Use flakes for reproducible environments\n")

	return suggestions.String()
}
