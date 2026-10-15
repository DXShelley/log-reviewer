# HTTPS、HTTP 兼容与访问控制测试

适用于当前 GUI/serve：默认 HTTPS 端口 8766（`--https-port`），HTTP 兼容端口 8765（`--port`）。端口占用时自动分配，测试和客户端应读取启动输出：`GUI:` 是 HTTPS，`HTTP:` 是兼容入口。`--http=false` 只关闭 HTTP；不是 HTTP 自动跳转开关。

## 自动化用例矩阵

| 范围 | 关键断言 | 测试入口 |
| --- | --- | --- |
| URL 与参数 | 默认生成 HTTPS URL，IPv6 方括号与作用域转义，两个端口参数越界拒绝 | `cmd/log-reviewer/main_test.go` |
| 自签名证书 | localhost/IPv4/IPv6/域名 SAN，重启复用，新增地址更新 | `internal/server/tls_test.go` |
| 证书异常 | 文件损坏、不可写、缺少密钥拒绝；显式过期证书拒绝，自动证书到期续签 | `internal/server/tls_test.go` |
| TLS 握手 | 显式信任测试证书后成功，未信任证书与错误主机名拒绝；TLS 1.2/1.3 成功，TLS 1.1 拒绝 | `tests/ui-https.cjs` |
| 双协议安全边界 | HTTP/HTTPS 均要求 token、拒绝跨站 Origin、执行来源 IP 白名单 | `tests/ui-https.cjs`、`internal/server/tls_test.go` |
| HTTP 关闭 | HTTPS-only 启动后，指定 HTTP 端口实际连接被拒绝 | `tests/ui-https.cjs` |
| 启动失败 | 缺少成对证书/密钥、损坏自签名文件或指定证书时退出，不输出可用 GUI/HTTP 地址 | `tests/ui-https.cjs` |
| 白名单规则 | 单 IP/CIDR、网段边界、IPv4 映射 IPv6、非法地址、伪造转发头 | `internal/server/access_test.go` |
| token | 缺失、裸 token、错误 scheme、错误值、重复认证头拒绝；重启旧 token 失效 | `internal/server/access_test.go`、`tests/ui-access.cjs` |
| 配置持久化 | GUI 保存即时生效、重启恢复、排除当前来源拒绝、CLI 覆盖及 off 关闭 | `tests/ui-access.cjs` |
| 配置失败 | 状态文件损坏拒绝启动，保存失败不修改当前规则 | `internal/server/access_test.go` |
| 浏览器流程 | HTTPS 页面身份、安全上下文、上传分析、访问控制配置、桌面/移动尺寸及运行时错误检查 | `tests/ui-https.cjs` |
| 旧入口兼容 | HTTP 通配监听、动态端口、同源校验、IPv6 可用时访问及远程浏览器上传分析 | `tests/server-host.cjs` |

## 执行方法

先运行 Go 测试与静态检查：

```powershell
go test ./...
go vet ./...
python scripts/check-doc-links.py
```

浏览器验证需要现有 Node.js、Playwright 和独立 Chrome 调试 Profile，CDP 地址为 `http://127.0.0.1:9222`。`NODE_PATH` 指向已安装的 Playwright 包目录；不需要修改全局 TLS 校验或系统证书信任。Windows 示例：

```powershell
$securityRun = Join-Path $env:TEMP ('log-reviewer-security-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $securityRun | Out-Null
go build -o (Join-Path $securityRun 'log-reviewer.exe') ./cmd/log-reviewer
if ($LASTEXITCODE -ne 0) { throw 'Build failed' }
$env:LR_TEST_BINARY = Join-Path $securityRun 'log-reviewer.exe'
node tests/ui-https.cjs
node tests/ui-access.cjs
node tests/server-host.cjs
```

HTTPS 与白名单浏览器用例使用临时目录隔离状态、证书和语言规则；浏览器使用独立 context，结束后关闭自启进程。截图和临时文件路径随测试输出，不提交到仓库。`ui-https.cjs` 先用严格 TLS 客户端验证信任和主机名，再仅在该测试的浏览器 context 中接受自签名证书；此设置不是产品默认行为。`ui-access.cjs` 通过 HTTP 兼容入口验证既有操作，HTTPS 流程由独立用例覆盖。

`tests/ui-remote-retry.cjs` 需要现有服务，通过 `LR_TEST_URL` 显式指定 HTTP 兼容 URL（含本次 token），或读取 `.work/gui.out` 的 `HTTP:` 行。该文件不应提交真实 token。其他功能的用例见 [只读读取与路径兼容测试](readonly-testing.md)。

## 人工验收与边界

1. 首次启动确认默认打开 HTTPS；自签名证书出现浏览器信任提示。工具不自动安装根证书，正式部署改用 `--tls-cert` / `--tls-key`。
2. 保留证书文件重启，确认证书不变、token 改变；新增访问地址或证书续签后重新核实证书。
3. 确认 HTTP 兼容 URL 可用；用 `--http=false` 重启后确认 HTTP 端口不再由工具监听。
4. 从另一台机器验证白名单内外来源；反向代理后的来源限制在代理层配置，不能依赖可伪造的转发头。
5. 受信任 HTTPS 下检查剪贴板与大文件保存；自动化中忽略证书错误不等于实际浏览器已经信任证书。

单元测试与本机浏览器通过不代表公共 CA 签发、系统信任安装、公网 NAT、防火墙、企业代理或跨平台实机全部验证。HTTP 保留明文传输风险，白名单不会把共享 NAT 出口下的用户隔离；持有 token 的允许来源仍有会话管理权限。安全部署说明见 [服务器说明](server-usage.md) 和 [安全报告](../SECURITY.md)。
