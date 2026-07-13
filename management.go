package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"grok-429-autoban/cpasdk/pluginapi"
)

type managementHandler struct{}

const managementRoutePrefix = "/plugins/" + pluginID

func managementRegistration() pluginapi.ManagementRegistrationResponse {
	return pluginapi.ManagementRegistrationResponse{
		Routes: []pluginapi.ManagementRoute{
			{Method: http.MethodGet, Path: managementRoutePrefix + "/bans", Description: "查看 Grok 自动禁用账号"},
			{Method: http.MethodPost, Path: managementRoutePrefix + "/unban", Description: "解除单个 Grok 账号禁用"},
			{Method: http.MethodPost, Path: managementRoutePrefix + "/unban-all", Description: "解除全部 Grok 账号禁用"},
		},
		Resources: []pluginapi.ResourceRoute{
			{Path: "/status", Menu: "Grok 自动禁用", Description: "查看 Grok 自动禁用状态"},
		},
	}
}

func (managementHandler) HandleManagement(_ context.Context, req pluginapi.ManagementRequest) (pluginapi.ManagementResponse, error) {
	return dispatchManagement(req)
}

func dispatchManagement(req pluginapi.ManagementRequest) (pluginapi.ManagementResponse, error) {
	switch {
	case req.Method == http.MethodGet && strings.HasSuffix(req.Path, "/bans"):
		return jsonManagementResponse(http.StatusOK, banStatus()), nil
	case req.Method == http.MethodPost && strings.HasSuffix(req.Path, "/unban"):
		var body struct {
			AuthID string `json:"auth_id"`
		}
		if err := json.Unmarshal(req.Body, &body); err != nil || strings.TrimSpace(body.AuthID) == "" {
			return jsonManagementResponse(http.StatusBadRequest, map[string]string{"error": "missing_auth_id"}), nil
		}
		authID := strings.TrimSpace(body.AuthID)
		password := resolveManagementPassword(req.Headers)
		if errEnable := enableAuthInCPA(authID, password); errEnable != nil {
			return jsonManagementResponse(http.StatusBadRequest, map[string]string{"error": errEnable.Error()}), nil
		}
		removed := activeStore.Delete(authID)
		saveActiveStore()
		return jsonManagementResponse(http.StatusOK, map[string]any{"ok": true, "removed": removed, "enabled": true}), nil
	case req.Method == http.MethodPost && strings.HasSuffix(req.Path, "/unban-all"):
		password := resolveManagementPassword(req.Headers)
		// time.Time{} lists all entries with ResetAt after zero (all future resets).
		items := activeStore.List(time.Time{})
		failures := make([]string, 0)
		enabled := 0
		for _, entry := range items {
			if errEnable := enableAuthInCPA(entry.AuthID, password); errEnable != nil {
				failures = append(failures, entry.AuthID+": "+errEnable.Error())
				continue
			}
			enabled++
			_ = activeStore.Delete(entry.AuthID)
		}
		if len(failures) == 0 {
			activeStore.Clear()
		}
		saveActiveStore()
		status := http.StatusOK
		if len(failures) > 0 && enabled == 0 {
			status = http.StatusBadRequest
		}
		return jsonManagementResponse(status, map[string]any{
			"ok":       len(failures) == 0,
			"enabled":  enabled,
			"failed":   len(failures),
			"failures": failures,
		}), nil
	case req.Method == http.MethodGet && strings.HasSuffix(req.Path, "/status"):
		return managementStatusPage(req)
	default:
		return jsonManagementResponse(http.StatusNotFound, map[string]string{"error": "not_found"}), nil
	}
}

func saveActiveStore() {
	cfg := loadedConfig()
	if cfg.PersistState && cfg.StateFile != "" {
		_ = activeStore.Save(cfg.StateFile)
	}
}

func banStatus() map[string]any {
	now := time.Now()
	items := activeStore.List(now)
	out := make([]map[string]any, 0, len(items))
	for _, entry := range items {
		out = append(out, map[string]any{
			"auth_id":           entry.AuthID,
			"provider":          entry.Provider,
			"error_code":        entry.ErrorCode,
			"banned_at":         entry.BannedAt.Format(time.RFC3339),
			"reset_at":          entry.ResetAt.Format(time.RFC3339),
			"reset_source":      entry.ResetSource,
			"trace_id":          entry.TraceID,
			"remaining_seconds": int64(entry.ResetAt.Sub(now).Seconds()),
		})
	}
	return map[string]any{
		"plugin":         pluginID,
		"fallback_hours": loadedConfig().FallbackHours,
		"bans":           out,
	}
}

func jsonManagementResponse(status int, value any) pluginapi.ManagementResponse {
	raw, _ := json.Marshal(value)
	return pluginapi.ManagementResponse{
		StatusCode: status,
		Headers:    http.Header{"Content-Type": []string{"application/json; charset=utf-8"}},
		Body:       raw,
	}
}

func managementStatusPage(_ pluginapi.ManagementRequest) (pluginapi.ManagementResponse, error) {
	body := `<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8"><title>Grok 自动禁用</title>
<style>body{font-family:system-ui,sans-serif;max-width:960px;margin:32px auto;padding:0 20px;color:#1f2937}button{padding:8px 14px;cursor:pointer}table{width:100%;border-collapse:collapse;margin-top:18px}td,th{padding:8px;border-bottom:1px solid #ddd;text-align:left}</style></head>
<body><h1>Grok 自动禁用</h1><p>处理 free-usage-exhausted（429，默认 24 小时恢复）、permission-denied（403）和 401 认证失败（手动解禁）。</p>
<p><input id="key" type="password" placeholder="CPA Management Key"><button onclick="saveKey()">保存密钥</button></p>
<button onclick="loadBans()">刷新状态</button><button onclick="unbanAll()">全部解禁</button>
<table><thead><tr><th>账号</th><th>恢复时间</th><th>来源</th><th>剩余秒数</th><th>操作</th></tr></thead><tbody id="rows"></tbody></table>
<script>
const keyInput = document.getElementById("key"); keyInput.value = localStorage.getItem("grok429ManagementKey") || "";
function saveKey() { localStorage.setItem("grok429ManagementKey", keyInput.value); }
async function call(path, options={}) { options.headers = Object.assign({}, options.headers||{}, {"Authorization":"Bearer "+keyInput.value,"Content-Type":"application/json"}); const r = await fetch("/v0/management/plugins/grok-429-autoban"+path, options); const data = await r.json(); if (!r.ok) throw new Error((data && (data.error||data.message)) || ("HTTP "+r.status)); return data; }
async function loadBans() { const data = await call("/bans"); document.getElementById("rows").innerHTML = (data.bans||[]).map(b => "<tr><td><code>"+esc(b.auth_id)+"</code></td><td>"+esc(b.reset_at)+"</td><td>"+esc(b.reset_source)+"</td><td>"+esc(String(b.remaining_seconds))+"</td><td><button onclick='unban("+JSON.stringify(b.auth_id)+")'>解禁</button></td></tr>").join(""); }
async function unban(id) { try { await call("/unban",{method:"POST",body:JSON.stringify({auth_id:id})}); } catch (e) { alert(String(e.message||e)); } loadBans(); }
async function unbanAll() { try { const data = await call("/unban-all",{method:"POST",body:"{}"}); if (data && data.failed) alert("部分解禁失败: "+(data.failures||[]).slice(0,3).join("; ")); } catch (e) { alert(String(e.message||e)); } loadBans(); }
function esc(v) { return String(v).replace(/[&<>"']/g, c => ({"&":"&amp;","<":"&lt;",">":"&gt;","\"":"&quot;","'":"&#39;"}[c])); }
loadBans();
</script></body></html>`
	return pluginapi.ManagementResponse{
		StatusCode: http.StatusOK,
		Headers:    http.Header{"Content-Type": []string{"text/html; charset=utf-8"}},
		Body:       []byte(body),
	}, nil
}

func handleManagement(raw []byte) ([]byte, error) {
	var req pluginapi.ManagementRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}
	response, err := dispatchManagement(req)
	if err != nil {
		return nil, err
	}
	return okEnvelope(response)
}
