package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

var (
	cpaManagementBaseURL = "http://127.0.0.1:8317"
	cpaManagementDo      = http.DefaultClient.Do
)

func cpaManagementPassword() string {
	if value := strings.TrimSpace(os.Getenv("MANAGEMENT_PASSWORD")); value != "" {
		return value
	}
	return strings.TrimSpace(os.Getenv("CPA_MANAGEMENT_KEY"))
}

func extractBearerToken(headers http.Header) string {
	if headers == nil {
		return ""
	}
	auth := strings.TrimSpace(headers.Get("Authorization"))
	if auth == "" {
		for key, values := range headers {
			if strings.EqualFold(strings.TrimSpace(key), "Authorization") && len(values) > 0 {
				auth = strings.TrimSpace(values[0])
				break
			}
		}
	}
	if auth == "" {
		return ""
	}
	const prefix = "bearer "
	if len(auth) > len(prefix) && strings.EqualFold(auth[:len(prefix)], prefix) {
		return strings.TrimSpace(auth[len(prefix):])
	}
	return auth
}

func resolveManagementPassword(headers http.Header) string {
	if headers != nil {
		if token := extractBearerToken(headers); token != "" {
			return token
		}
		if token := strings.TrimSpace(headers.Get("X-Management-Key")); token != "" {
			return token
		}
		for key, values := range headers {
			if strings.EqualFold(strings.TrimSpace(key), "X-Management-Key") && len(values) > 0 {
				if token := strings.TrimSpace(values[0]); token != "" {
					return token
				}
			}
		}
	}
	return cpaManagementPassword()
}

func disableAuthInCPA(authID string) error {
	return setAuthDisabledInCPA(authID, true, cpaManagementPassword())
}

func enableAuthInCPA(authID string, password string) error {
	return setAuthDisabledInCPA(authID, false, password)
}

func setAuthDisabledInCPA(authID string, disabled bool, password string) error {
	authID = strings.TrimSpace(authID)
	if authID == "" {
		return fmt.Errorf("auth_id is required")
	}
	password = strings.TrimSpace(password)
	if password == "" {
		password = cpaManagementPassword()
	}
	if password == "" {
		return fmt.Errorf("CPA management password is unavailable")
	}

	body, errMarshal := json.Marshal(map[string]any{
		"name":     authID,
		"disabled": disabled,
	})
	if errMarshal != nil {
		return errMarshal
	}
	req, errRequest := http.NewRequest(
		http.MethodPatch,
		strings.TrimRight(cpaManagementBaseURL, "/")+"/v0/management/auth-files/status",
		bytes.NewReader(body),
	)
	if errRequest != nil {
		return errRequest
	}
	req.Header.Set("Authorization", "Bearer "+password)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, errDo := cpaManagementDo(req)
	if errDo != nil {
		return errDo
	}
	defer resp.Body.Close()
	raw, errRead := io.ReadAll(resp.Body)
	if errRead != nil {
		return errRead
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("CPA management API returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	return nil
}
