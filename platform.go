package main

import (
	"os"
	"strings"
)

// Platform represents the operating system environment
type Platform string

const (
	PlatformMacOS        Platform = "macos"
	PlatformLinux        Platform = "linux"
	PlatformNixOS        Platform = "nixos"
	PlatformNixOSWSL     Platform = "nixos-wsl"
	PlatformGenericLinux Platform = "generic-linux"
	PlatformUnknown      Platform = "unknown"
)

// DetectPlatform identifies the current running environment
func DetectPlatform() Platform {
	if isMacOS() {
		return PlatformMacOS
	}

	if isLinux() {
		isNixOS := isFileExists("/etc/NIXOS")
		if !isNixOS {
			// Check os-release for NixOS
			data, err := os.ReadFile("/etc/os-release")
			if err == nil && strings.Contains(string(data), "ID=nixos") {
				isNixOS = true
			}
		}

		isWSL := false
		data, err := os.ReadFile("/proc/version")
		if err == nil {
			v := strings.ToLower(string(data))
			if strings.Contains(v, "microsoft") || strings.Contains(v, "wsl") {
				isWSL = true
			}
		}

		if isNixOS && isWSL {
			return PlatformNixOSWSL
		}
		if isNixOS {
			return PlatformNixOS
		}
		return PlatformGenericLinux
	}

	return PlatformUnknown
}

func isMacOS() bool {
	// Simple check, can be improved if needed
	return strings.Contains(strings.ToLower(os.Getenv("OSTYPE")), "darwin") || 
		isFileExists("/System/Library/CoreServices/SystemVersion.plist")
}

func isLinux() bool {
	return isFileExists("/proc/version")
}

func isFileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (p Platform) IsWSL() bool {
	return p == PlatformNixOSWSL
}

func (p Platform) IsNixOS() bool {
	return p == PlatformNixOS || p == PlatformNixOSWSL
}
