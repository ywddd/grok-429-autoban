package main

import (
	"encoding/json"
	"testing"
	"time"

	"grok-429-autoban/cpasdk/pluginapi"
)

func TestSchedulerFiltersActiveGrokBans(t *testing.T) {
	store := newBanStore()
	store.Set(testEntry("banned", time.Now().Add(time.Hour)))
	req := pluginapi.SchedulerPickRequest{
		Provider: "xai",
		Candidates: []pluginapi.SchedulerAuthCandidate{
			{ID: "banned", Provider: "xai", Priority: 100},
			{ID: "available", Provider: "xai", Priority: 10},
		},
	}
	got := pickCandidate(req, store, time.Now())
	if !got.Handled || got.AuthID != "available" {
		t.Fatalf("response = %#v", got)
	}
}

func TestSchedulerDelegatesWhenNothingIsBanned(t *testing.T) {
	req := pluginapi.SchedulerPickRequest{
		Provider: "xai",
		Candidates: []pluginapi.SchedulerAuthCandidate{
			{ID: "one", Provider: "xai"},
		},
	}
	got := pickCandidate(req, newBanStore(), time.Now())
	if !got.Handled || got.DelegateBuiltin != pluginapi.SchedulerBuiltinRoundRobin || got.AuthID != "" {
		t.Fatalf("response = %#v", got)
	}
}

func TestSchedulerRestoresExpiredGrokBan(t *testing.T) {
	store := newBanStore()
	store.Set(testEntry("expired", time.Unix(100, 0)))
	req := pluginapi.SchedulerPickRequest{
		Provider: "xai",
		Candidates: []pluginapi.SchedulerAuthCandidate{
			{ID: "expired", Provider: "xai"},
		},
	}
	got := pickCandidate(req, store, time.Unix(101, 0))
	if !got.Handled || got.DelegateBuiltin != pluginapi.SchedulerBuiltinRoundRobin {
		t.Fatalf("response = %#v", got)
	}
	if _, ok := store.Get("expired"); ok {
		t.Fatal("expired ban was not removed")
	}
}

func TestHandleUsageRecordsExactGrokBans(t *testing.T) {
	oldStore := activeStore
	activeStore = newBanStore()
	defer func() { activeStore = oldStore }()

	record := pluginapi.UsageRecord{
		Provider: "xai",
		AuthID:   "auth-1",
		Failed:   true,
		Failure:  pluginapi.UsageFailure{StatusCode: 429, Body: realGrok429Body},
	}
	if _, err := handleUsageRecord(record, defaultPluginConfig(), time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, ok := activeStore.Get("auth-1"); !ok {
		t.Fatal("exact Grok 429 was not stored")
	}

	record.AuthID = "auth-403"
	record.Failure = pluginapi.UsageFailure{StatusCode: 403, Body: realGrok403Body}
	if _, err := handleUsageRecord(record, defaultPluginConfig(), time.Now()); err != nil {
		t.Fatal(err)
	}
	if entry, ok := activeStore.Get("auth-403"); !ok {
		t.Fatal("exact Grok 403 was not stored")
	} else if entry.ErrorCode != permissionDeniedErrorCode || entry.ResetSource != "manual_unban" {
		t.Fatalf("403 entry = %#v", entry)
	}

	record.Failure.Body = `{"code":"rate_limit"}`
	record.AuthID = "auth-2"
	record.Failure.StatusCode = 429
	if _, err := handleUsageRecord(record, defaultPluginConfig(), time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, ok := activeStore.Get("auth-2"); ok {
		t.Fatal("generic 429 was stored")
	}
}

func TestHandleSchedulerPickEnvelope(t *testing.T) {
	oldStore := activeStore
	activeStore = newBanStore()
	defer func() { activeStore = oldStore }()
	activeStore.Set(testEntry("banned", time.Now().Add(time.Hour)))

	raw, err := handleSchedulerPick(mustJSON(pluginapi.SchedulerPickRequest{
		Provider: "xai",
		Candidates: []pluginapi.SchedulerAuthCandidate{
			{ID: "banned", Provider: "xai", Priority: 100},
			{ID: "available", Provider: "xai", Priority: 1},
		},
	}))
	if err != nil {
		t.Fatal(err)
	}
	var env struct {
		Result pluginapi.SchedulerPickResponse `json:"result"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatal(err)
	}
	if env.Result.AuthID != "available" {
		t.Fatalf("result = %#v", env.Result)
	}
}

func mustJSON(value any) []byte {
	raw, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return raw
}
