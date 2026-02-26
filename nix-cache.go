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
	Name      string // Display name
	URL       string // Cache URL (e.g., https://nix-cache.stevedores.org)
	PublicKey string // Trusted public key (base64-encoded ed25519). Empty means unknown.
	Public    bool   // Whether to add to public binary caches
	Trusted   bool   // Whether to trust the cache
}

// DefaultNixCaches are recommended caches for stevedores-org ecosystem
var DefaultNixCaches = []NixCache{
	{
		Name:      "stevedores-attic",
		URL:       "https://nix-cache.stevedores.org",
		PublicKey: "oxidizedmlx-cache-1:uG3uzexkJno1b3b+dek7tHnHzr1p6MHxIoVTqnp/JBI=",
		Public:    true,
		Trusted:   true,
	},
	{
		Name:      "cache.nixos.org",
		URL:       "https://cache.nixos.org",
		PublicKey: "cache.nixos.org-1:6NCHdD59X431o0gWypQydGvjwydGG2UZTvhjGJNsx6E=",
		Public:    true,
		Trusted:   false,
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
	if cache.Trusted && cache.PublicKey != "" {
		cacheEntry += fmt.Sprintf("trusted-public-keys = %s\n", cache.PublicKey)
	}
	_ = cacheEntry // reserved for future auto-write support

	warnf("To add cache manually, add to ~/.config/nix/nix.conf:\n")
	warnf("  extra-substituters = %s\n", cache.URL)
	if cache.PublicKey != "" {
		warnf("  trusted-public-keys = %s\n", cache.PublicKey)
	} else if cache.Trusted {
		warnf("  ⚠️  No public key available for %s yet — signatures will fail until the key is published.\n", cache.Name)
		warnf("  Check https://github.com/stevedores-org/local-ci for the trusted-public-keys value.\n")
	}

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
	if atticCache.PublicKey != "" {
		printf("   trusted-public-keys = %s\n\n", atticCache.PublicKey)
	} else {
		printf("   ⚠️  trusted-public-keys: not yet published — check https://github.com/stevedores-org/local-ci for updates\n\n")
	}

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
