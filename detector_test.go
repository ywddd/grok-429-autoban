package main

import (
	"net/http"
	"testing"
	"time"

	"grok-429-autoban/cpasdk/pluginapi"
)

const realGrok429Body = `{"code":"subscription:free-usage-exhausted","error":"You've used all the included free usage for model grok-4.5-build-free for now. Usage resets over a rolling 24-hour window — tokens (actual/limit): 2050798/2000000. Upgrade to a Grok subscription for higher limits: https://grok.com/supergrok"}`

func TestDetectRealGrokFreeUsageExhausted(t *testing.T) {
	now := time.Date(2026, 7, 12, 11, 40, 0, 0, time.UTC)
	record := pluginapi.UsageRecord{
		Provider: "xai",
		AuthID:   "xai-account-1",
		Failed:   true,
		Failure: pluginapi.UsageFailure{
			StatusCode: 429,
			Body:       realGrok429Body,
		},
		ResponseHeaders: http.Header{
			"Date":         []string{"Sun, 12 Jul 2026 11:33:34 GMT"},
			"X-Request-Id": []string{"0adcec99-a0fb-9519-9498-5d73a4c58035"},
		},
	}

	entry, ok := detectBan(record, defaultPluginConfig(), now)
	if !ok {
		t.Fatal("detectBan() did not match real Grok 429")
	}
	wantReset := time.Date(2026, 7, 13, 11, 33, 34, 0, time.UTC)
	if !entry.ResetAt.Equal(wantReset) {
		t.Fatalf("reset at = %s, want %s", entry.ResetAt, wantReset)
	}
	if entry.ResetSource != "date_plus_fallback" {
		t.Fatalf("reset source = %q", entry.ResetSource)
	}
	if entry.ErrorCode != exhaustedErrorCode || entry.Provider != "xai" {
		t.Fatalf("entry = %#v", entry)
	}
	if entry.TraceID != "0adcec99-a0fb-9519-9498-5d73a4c58035" {
		t.Fatalf("trace id = %q", entry.TraceID)
	}
}

func TestDetectBanRejectsNonExactMatches(t *testing.T) {
	base := pluginapi.UsageRecord{
		Provider: "xai",
		AuthID:   "auth-1",
		Failed:   true,
		Failure: pluginapi.UsageFailure{
			StatusCode: 429,
			Body:       realGrok429Body,
		},
	}
	tests := []struct {
		name   string
		mutate func(*pluginapi.UsageRecord)
	}{
		{"wrong provider", func(r *pluginapi.UsageRecord) { r.Provider = "codex" }},
		{"not failed", func(r *pluginapi.UsageRecord) { r.Failed = false }},
		{"wrong status", func(r *pluginapi.UsageRecord) { r.Failure.StatusCode = 503 }},
		{"empty auth", func(r *pluginapi.UsageRecord) { r.AuthID = "" }},
		{"invalid json", func(r *pluginapi.UsageRecord) { r.Failure.Body = "too many requests" }},
		{"wrong code", func(r *pluginapi.UsageRecord) { r.Failure.Body = `{"code":"rate_limit"}` }},
		{"missing code", func(r *pluginapi.UsageRecord) { r.Failure.Body = `{"error":"rolling 24-hour window"}` }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record := base
			tt.mutate(&record)
			if _, ok := detectBan(record, defaultPluginConfig(), time.Now()); ok {
				t.Fatal("detectBan() matched an ineligible record")
			}
		})
	}
}

func TestDetectBanAcceptsGrokProviderAndUsesLocalFallback(t *testing.T) {
	now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	record := pluginapi.UsageRecord{
		Provider: "GROK",
		AuthID:   "auth-1",
		Failed:   true,
		Failure: pluginapi.UsageFailure{
			StatusCode: 429,
			Body:       realGrok429Body,
		},
		ResponseHeaders: http.Header{"Date": []string{"not-a-date"}},
	}
	entry, ok := detectBan(record, defaultPluginConfig(), now)
	if !ok {
		t.Fatal("detectBan() did not match provider GROK")
	}
	if entry.Provider != "xai" {
		t.Fatalf("provider = %q, want xai", entry.Provider)
	}
	if !entry.ResetAt.Equal(now.Add(24*time.Hour)) || entry.ResetSource != "local_plus_fallback" {
		t.Fatalf("entry = %#v", entry)
	}
}
