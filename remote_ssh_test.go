package main

import "testing"

func TestNormalizeSSHHost_BareMacOS(t *testing.T) {
	got := NormalizeSSHHost("uranus", remotePlatformMacOS, RemoteSSHDefaults{})
	if got != "aivcs@uranus" {
		t.Fatalf("got %q", got)
	}
}

func TestNormalizeSSHHost_BareLinuxSpark(t *testing.T) {
	got := NormalizeSSHHost("spark-bde7", remotePlatformLinuxSpark, RemoteSSHDefaults{})
	if got != "aivcs2@spark-bde7" {
		t.Fatalf("got %q", got)
	}
}

func TestNormalizeSSHHost_ExplicitUserUnchanged(t *testing.T) {
	got := NormalizeSSHHost("stevenirvin@uranus", remotePlatformMacOS, RemoteSSHDefaults{})
	if got != "stevenirvin@uranus" {
		t.Fatalf("explicit user must not be rewritten; got %q", got)
	}
}

func TestNormalizeSSHHost_CustomDefaults(t *testing.T) {
	d := RemoteSSHDefaults{MacOSUser: "bot", LinuxSparkUser: "builder"}
	if got := NormalizeSSHHost("discovery", remotePlatformMacOS, d); got != "bot@discovery" {
		t.Fatalf("got %q", got)
	}
}

func TestRemoteHostEffectivePlatform(t *testing.T) {
	h := RemoteHost{Platform: "linux_spark"}
	if h.effectivePlatform("uranus") != remotePlatformLinuxSpark {
		t.Fatal("explicit platform should win")
	}
	h = RemoteHost{}
	if h.effectivePlatform("sparky") != remotePlatformLinuxSpark {
		t.Fatal("sparky preset should imply linux_spark")
	}
	if h.effectivePlatform("uranus") != remotePlatformMacOS {
		t.Fatal("default should be macos")
	}
}

func TestGetRemoteHost_NormalizesBareHost(t *testing.T) {
	dir := writeRemoteTomlWithHosts(t, `
[ssh_defaults]
macos_user = "aivcs"
linux_spark_user = "aivcs2"

[hosts.uranus]
host = "uranus"
platform = "macos"
`)
	cfg, err := LoadConfig(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	h, err := cfg.GetRemoteHost("uranus")
	if err != nil {
		t.Fatal(err)
	}
	if h.Host != "aivcs@uranus" {
		t.Fatalf("got %q", h.Host)
	}
}
