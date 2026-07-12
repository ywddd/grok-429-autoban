package main

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"grok-429-autoban/cpasdk/pluginapi"
)

func TestManagementRegistration(t *testing.T) {
	reg := managementRegistration()
	if len(reg.Routes) != 3 || len(reg.Resources) != 1 {
		t.Fatalf("registration = %#v", reg)
	}
	if reg.Routes[0].Path != "/bans" || reg.Routes[1].Path != "/unban" || reg.Routes[2].Path != "/unban-all" {
		t.Fatalf("routes = %#v", reg.Routes)
	}
	if reg.Resources[0].Path != "/status" {
		t.Fatalf("resources = %#v", reg.Resources)
	}
}

func TestManagementListAndUnban(t *testing.T) {
	oldStore := activeStore
	activeStore = newBanStore()
	defer func() { activeStore = oldStore }()
	activeStore.Set(testEntry("auth-1", time.Now().Add(time.Hour)))

	list := managementRequest(http.MethodGet, "/bans", nil)
	response, err := dispatchManagement(list)
	if err != nil || response.StatusCode != http.StatusOK {
		t.Fatalf("list response = %#v, err=%v", response, err)
	}
	if !strings.Contains(string(response.Body), "auth-1") {
		t.Fatalf("list body = %s", response.Body)
	}
	if strings.Contains(string(response.Body), "access_token") {
		t.Fatal("status leaked secret field")
	}

	unban := managementRequest(http.MethodPost, "/unban", []byte(`{"auth_id":"auth-1"}`))
	response, err = dispatchManagement(unban)
	if err != nil || response.StatusCode != http.StatusOK {
		t.Fatalf("unban response = %#v, err=%v", response, err)
	}
	if _, ok := activeStore.Get("auth-1"); ok {
		t.Fatal("auth-1 remains after unban")
	}
}

func TestManagementRejectsMissingAuthIDAndClearsAll(t *testing.T) {
	oldStore := activeStore
	activeStore = newBanStore()
	defer func() { activeStore = oldStore }()
	activeStore.Set(testEntry("auth-1", time.Now().Add(time.Hour)))
	activeStore.Set(testEntry("auth-2", time.Now().Add(time.Hour)))

	response, err := dispatchManagement(managementRequest(http.MethodPost, "/unban", []byte(`{}`)))
	if err != nil || response.StatusCode != http.StatusBadRequest {
		t.Fatalf("missing auth response = %#v, err=%v", response, err)
	}
	response, err = dispatchManagement(managementRequest(http.MethodPost, "/unban-all", nil))
	if err != nil || response.StatusCode != http.StatusOK {
		t.Fatalf("unban all response = %#v, err=%v", response, err)
	}
	if len(activeStore.List(time.Now())) != 0 {
		t.Fatal("unban-all did not clear state")
	}
}

func TestManagementResourcePageIsChinese(t *testing.T) {
	response, err := managementStatusPage(pluginapi.ManagementRequest{})
	if err != nil || response.StatusCode != http.StatusOK {
		t.Fatalf("page response = %#v, err=%v", response, err)
	}
	body := string(response.Body)
	if !strings.Contains(body, "Grok 429 自动禁用") || !strings.Contains(body, "/bans") {
		t.Fatalf("page body missing expected text: %s", body)
	}
}

func managementRequest(method, path string, body []byte) pluginapi.ManagementRequest {
	return pluginapi.ManagementRequest{
		Method: method,
		Path:   path,
		Body:   body,
	}
}
