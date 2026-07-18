# Grok Auto Ban

CLIProxyAPI（CPA）插件：检测 Grok/xAI 账号的免费额度耗尽（429）、权限拒绝（403）和认证失败（401），并自动将对应账号移出调度池。

插件 ID / 仓库：grok-autoban（原 grok-429-autoban，v0.1.8 起更名）。

## 生效范围

只对 Provider 为 xai、x-ai 或 grok 的账号生效。其他 Provider 的 401/403/429 不会触发禁用。

## 检测条件

只有以下条件同时满足才会禁用账号：

1. Provider 是 xai、x-ai 或 grok
2. 命中下面任一错误：

| HTTP | 匹配条件 | 恢复方式 |
| --- | --- | --- |
| 429 | JSON code = subscription:free-usage-exhausted | 默认 24 小时后自动恢复 |
| 403 | JSON code = permission-denied | 手动解禁 |
| 401 | 任意 Grok/xAI 认证失败响应 | 手动解禁 |

普通 429/403、其他错误码不会触发禁用。401 只要求状态码。

## 恢复规则

### 429 免费额度耗尽

默认使用响应头 Date 加 24 小时；优先使用 Retry-After 或 reset 时间。可通过 fallback_hours 调整（1-168，默认 24）。到期后下一次调度自动恢复。

### 403 / 401 长期禁用

需手动解禁。

## CPA 配置

`yaml
plugins:
  enabled: true
  configs:
    grok-autoban:
      enabled: true
      priority: 100
      fallback_hours: 24
      persist_state: true
      state_file: plugins/data/grok-autoban/bans.json
      log_matches: true
`

## 安装

`	ext
plugins/windows/amd64/grok-autoban.dll
`

## 管理接口

`	ext
GET  /v0/management/plugins/grok-autoban/bans
POST /v0/management/plugins/grok-autoban/unban
POST /v0/management/plugins/grok-autoban/unban-all
GET  /v0/resource/plugins/grok-autoban/status
`

## 本地构建

`powershell
.\build.ps1
`

`	ext
go test ./...
`
