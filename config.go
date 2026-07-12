package main

import (
	"encoding/json"
	"log/slog"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

type pluginConfig struct {
	FallbackHours int
	PersistState  bool
	StateFile     string
	LogMatches    bool
}

type configYAML struct {
	FallbackHours int    `yaml:"fallback_hours"`
	PersistState  *bool  `yaml:"persist_state"`
	StateFile     string `yaml:"state_file"`
	LogMatches    *bool  `yaml:"log_matches"`
}

type lifecycleRequest struct {
	SchemaVersion uint32 `json:"schema_version"`
	ConfigYAML    []byte `json:"config_yaml"`
}

var currentConfig atomic.Value

func init() {
	currentConfig.Store(defaultPluginConfig())
}

func defaultPluginConfig() pluginConfig {
	return pluginConfig{
		FallbackHours: 24,
		PersistState:  true,
		LogMatches:    true,
	}
}

func decodeConfig(raw []byte) (pluginConfig, error) {
	cfg := defaultPluginConfig()
	if len(raw) == 0 {
		return cfg, nil
	}

	decoded := configYAML{}
	for _, line := range strings.Split(string(raw), "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
		switch key {
		case "fallback_hours":
			decoded.FallbackHours, _ = strconv.Atoi(value)
		case "persist_state":
			if parsed, err := strconv.ParseBool(value); err == nil {
				decoded.PersistState = &parsed
			}
		case "state_file":
			decoded.StateFile = value
		case "log_matches":
			if parsed, err := strconv.ParseBool(value); err == nil {
				decoded.LogMatches = &parsed
			}
		}
	}
	if decoded.FallbackHours >= 1 && decoded.FallbackHours <= 168 {
		cfg.FallbackHours = decoded.FallbackHours
	}
	if decoded.PersistState != nil {
		cfg.PersistState = *decoded.PersistState
	}
	cfg.StateFile = strings.TrimSpace(decoded.StateFile)
	if decoded.LogMatches != nil {
		cfg.LogMatches = *decoded.LogMatches
	}
	return cfg, nil
}

func configure(raw []byte) error {
	var req lifecycleRequest
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &req); err != nil {
			return err
		}
	}
	cfg, err := decodeConfig(req.ConfigYAML)
	if err != nil {
		return err
	}
	currentConfig.Store(cfg)
	if cfg.PersistState && cfg.StateFile != "" {
		if err := activeStore.Load(cfg.StateFile, time.Now()); err != nil {
			slog.Warn("grok-429-autoban: failed to load state", "error", err)
		}
	}
	return nil
}

func loadedConfig() pluginConfig {
	if cfg, ok := currentConfig.Load().(pluginConfig); ok {
		return cfg
	}
	return defaultPluginConfig()
}
