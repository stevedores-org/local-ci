package main

import "strings"

// RemoteSSHDefaults holds canonical SSH users for remote CI (see docs/SSH_IDENTITY.md).
type RemoteSSHDefaults struct {
	MacOSUser      string `toml:"macos_user"`
	LinuxSparkUser string `toml:"linux_spark_user"`
}

const (
	remotePlatformMacOS      = "macos"
	remotePlatformLinuxSpark = "linux_spark"
)

func (d RemoteSSHDefaults) withDefaults() RemoteSSHDefaults {
	out := d
	if strings.TrimSpace(out.MacOSUser) == "" {
		out.MacOSUser = "aivcs"
	}
	if strings.TrimSpace(out.LinuxSparkUser) == "" {
		out.LinuxSparkUser = "aivcs2"
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
	if platform == remotePlatformLinuxSpark {
		user = d.LinuxSparkUser
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
	default:
		return remotePlatformMacOS
	}
}

func (c *Config) normalizeRemoteHost(presetName string, h RemoteHost) RemoteHost {
	h.Host = NormalizeSSHHost(h.Host, h.effectivePlatform(presetName), c.SSHDefaults)
	return h
}
