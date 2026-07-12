package main

import "testing"

func TestDefaultConfig(t *testing.T) {
	got := defaultPluginConfig()
	if got.FallbackHours != 24 {
		t.Fatalf("fallback hours = %d, want 24", got.FallbackHours)
	}
	if !got.PersistState {
		t.Fatal("persist state = false, want true")
	}
	if got.StateFile != "" {
		t.Fatalf("state file = %q, want empty", got.StateFile)
	}
	if !got.LogMatches {
		t.Fatal("log matches = false, want true")
	}
}

func TestDecodeConfig(t *testing.T) {
	got, err := decodeConfig([]byte("fallback_hours: 48\npersist_state: false\nstate_file: data/bans.json\nlog_matches: false\n"))
	if err != nil {
		t.Fatalf("decodeConfig() error = %v", err)
	}
	if got.FallbackHours != 48 || got.PersistState || got.StateFile != "data/bans.json" || got.LogMatches {
		t.Fatalf("config = %#v", got)
	}
}

func TestDecodeConfigInvalidFallbackUsesDefault(t *testing.T) {
	for _, raw := range []string{"fallback_hours: 0\n", "fallback_hours: 169\n"} {
		got, err := decodeConfig([]byte(raw))
		if err != nil {
			t.Fatalf("decodeConfig(%q) error = %v", raw, err)
		}
		if got.FallbackHours != 24 {
			t.Fatalf("decodeConfig(%q) fallback = %d, want 24", raw, got.FallbackHours)
		}
	}
}

func TestConfigureLoadsLifecycleYAML(t *testing.T) {
	err := configure([]byte(`{"schema_version":1,"config_yaml":"ZmFsbGJhY2tfaG91cnM6IDcyCnBlcnNpc3Rfc3RhdGU6IGZhbHNlCg=="}`))
	if err != nil {
		t.Fatalf("configure() error = %v", err)
	}
	got := loadedConfig()
	if got.FallbackHours != 72 || got.PersistState {
		t.Fatalf("loaded config = %#v", got)
	}
}
