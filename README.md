# Grok 429 Auto Ban

CLIProxyAPI（CPA）插件：检测 Grok/xAI 账号的免费额度耗尽（429）和权限拒绝（403）响应，并自动将对应账号移出调度池。

## 检测条件

只有以下条件同时满足才会禁用账号：

1. Provider 是 `xai` 或 `grok`
2. 命中下面任一错误：

| HTTP | JSON `code` | 恢复方式 |
| --- | --- | --- |
| `429` | `subscription:free-usage-exhausted` | 默认 24 小时后自动恢复 |
| `403` | `permission-denied` | 手动解禁 |

普通 429/403、其他错误码、Cloudflare 信息和 `X-Should-Retry` 都不会触发禁用。

## 恢复规则

### 429 免费额度耗尽

你提供的响应没有精确的恢复时间，所以插件使用响应头 `Date` 加 24 小时：

```text
2026-07-12 19:33:34 +08:00 触发
2026-07-13 19:33:34 +08:00 恢复
```

如果以后 Grok 提供 `Retry-After` 或 reset 时间，插件优先使用它。账号到期后会在下一次调度时自动恢复，不需要后台定时器。

### 403 权限拒绝

`permission-denied` 表示凭证权限有问题，不是临时额度窗口。插件会长期禁用该账号，直到你调用管理接口解禁，或在状态页手动解禁。

## CPA 配置

```yaml
plugins:
  enabled: true
  configs:
    grok-429-autoban:
      enabled: true
      priority: 100
      fallback_hours: 24
      persist_state: true
      state_file: plugins/data/grok-429-autoban/bans.json
      log_matches: true
```

`state_file` 留空时只保存在内存。建议设置一个可写路径，这样 CPA 重启后未到期的禁用仍然保留。

## 安装

Windows amd64：

```text
plugins/windows/amd64/grok-429-autoban.dll
```

也可以直接放在 CPA 的 `plugins/` 目录。文件名去掉扩展名就是插件 ID。

安装后重启 CPA，或在 CPAMP 插件管理页面刷新。插件商店来源使用本项目的 `registry.json`。

## 管理接口

以下接口需要 CPA Management Key：

```text
GET  /v0/management/plugins/grok-429-autoban/bans
POST /v0/management/plugins/grok-429-autoban/unban
POST /v0/management/plugins/grok-429-autoban/unban-all
GET  /v0/resource/plugins/grok-429-autoban/status
```

单个解禁请求：

```json
{"auth_id":"你的 CPA auth_id"}
```

接口只返回账号 ID、禁用时间和恢复时间，不返回 token、Cookie 或完整认证文件。

## 本地构建

要求：

- Go 1.21 或更高版本
- CGO
- Windows 使用 MinGW-w64/LLVM-MinGW，Linux 使用 gcc，macOS 使用 clang

Windows：

```powershell
.\build.ps1
```

Linux/macOS：

```bash
./build.sh
```

测试：

```text
go test ./...
go test -race ./...
go vet ./...
```
