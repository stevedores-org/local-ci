package main

import "strings"

// RemoteSSHDefaults holds canonical SSH users for remote CI (see docs/SSH_IDENTITY.md).
type RemoteSSHDefaults struct {
	MacOSUser      string `toml:"macos_user"`
	LinuxSparkUser string `toml:"linux_spark_user"`
	WindowsUser    string `toml:"windows_user"`
}

const (
	remotePlatformMacOS      = "macos"
	remotePlatformLinuxSpark = "linux_spark"
	remotePlatformWindows    = "windows"
)

func (d RemoteSSHDefaults) withDefaults() RemoteSSHDefaults {
	out := d
	if strings.TrimSpace(out.MacOSUser) == "" {
		out.MacOSUser = "aivcs"
	}
	if strings.TrimSpace(out.LinuxSparkUser) == "" {
		out.LinuxSparkUser = "aivcs2"
	}
	if strings.TrimSpace(out.WindowsUser) == "" {
		out.WindowsUser = "aivcs"
	}
	return out
}

// NormalizeSSHHost applies canonical users to bare Tailscale host names.
// Explicit user@host values are returned unchanged.
func NormalizeSSHHost(host, platform string, defaults RemoteSSHDefaults) string {
	host = strings.TrimSpace(host)
	if host == "" || strings.Contains(host, "@") {
		return host
	}
	d := defaults.withDefaults()
	user := d.MacOSUser
	switch platform {
	case remotePlatformLinuxSpark:
		user = d.LinuxSparkUser
	case remotePlatformWindows:
		user = d.WindowsUser
	}
	return user + "@" + host
}

func (h RemoteHost) effectivePlatform(presetName string) string {
	if p := strings.TrimSpace(h.Platform); p != "" {
		return p
	}
	// Back-compat: sparky / aivcs2 presets are always the Linux Spark node.
	switch presetName {
	case "sparky", "aivcs2":
		return remotePlatformLinuxSpark
	case "msi":
		return remotePlatformWindows
	default:
		return remotePlatformMacOS
	}
}

func (c *Config) normalizeRemoteHost(presetName string, h RemoteHost) RemoteHost {
	h.Host = NormalizeSSHHost(h.Host, h.effectivePlatform(presetName), c.SSHDefaults)
	return h
}
