# 开发说明

## 分支与目录

`dev` 是日常开发分支，也是贡献 PR 的目标；GitHub 默认分支 `main` 保留完整 Go 项目，包括 `go.mod`、`go.sum`、源码、测试、构建脚本和文档，验证后从 `dev` 合并。两分支均可直接构建。所有分支均禁止提交 `dist/`、可执行文件、依赖安装目录、缓存和测试报告，二进制仅作为 GitHub Release assets 发布。

| 目录 | 用途 |
| --- | --- |
| `cmd/log-reviewer/` | CLI 入口和命令 |
| `internal/logs/` | 解析、分类、索引、上下文及时间导出 |
| `internal/logs/rules/` | 按语言独立保存的 JSON 规则 |
| `internal/server/` | GUI 服务和 HTTP API |
| `internal/server/web/` | 内嵌 GUI 与离线组件 |
| `internal/remote/` | SFTP 传输 |
| `internal/testsupport/` | Go 集成测试辅助设施 |
| `tests/` | 浏览器回归与 SFTP 测试入口 |
| `scripts/` | 构建和依赖组件打包 |
| `tools/bench/` | 大文件基准工具 |
| `samples/` | 脱敏样例和规则 |
| `.github/` | CI、依赖更新、Issue 和 PR 模板 |

## 构建与检查

需要 Go 1.25+，先执行：

```sh
go mod download
go test ./...
go vet ./...
go mod verify
```

Windows：`powershell -File scripts/build.ps1 -AllPlatforms`。
Linux/macOS：`sh scripts/build.sh`。构建产物写入 `dist/`，生成 `SHA256SUMS.txt`，每个二进制不得超过 10 MiB。覆盖运行中的 Windows 程序可能产生 `~` 备份文件，该文件不会进入校验列表或发布文件。

GUI 通过 Go embed 打包，修改前端后必须重新构建并重启程序。无需安装前端框架或 Node.js 即可构建应用；只有修改离线组件时才需要 Node.js，参阅 [第三方依赖说明](../THIRD_PARTY_NOTICES.md)。

## 浏览器回归

HTTPS、HTTP 兼容、证书信任、TLS 版本、token 和 IP 白名单的用例矩阵及临时构建命令见 [访问控制测试](../security-testing.md)。执行 `node tests/ui-https.cjs`、`node tests/ui-access.cjs` 和 `node tests/server-host.cjs`。`GUI:` 行现在是 HTTPS；HTTP 兼容测试应读取 `HTTP:` 行。自签名证书的测试例外只作用于独立测试 context，不关闭全局 TLS 校验。

只读源文件、路径格式、本机导入、远程直读和网络异常缓存的用例矩阵、运行命令及人工验收见 [只读读取与路径兼容测试](../readonly-testing.md)。浏览器测试可用 `LR_TEST_BINARY` 指定临时构建的程序，避免依赖过期的 `dist/` 产物。

已有测试通过 Playwright 连接本机 Chrome `127.0.0.1:9222`，使用独立浏览器 Profile。设置 `NODE_PATH` 指向安装的 Playwright 后执行 `node tests/ui-design.cjs`、`node tests/ui-workspace.cjs`、`node tests/gui-smoke.cjs`。更多入口见 [项目状态](../PROJECT_STATUS.md)。浏览器测试目前属于本地验证，不宣称已在 GitHub CI 中运行。

## 自动化与发布

- Go CI 在 `dev` 和 `main` 的 push/PR 上运行 Linux、Windows、macOS 的 Go 测试、vet 和格式检查，并检查模块完整性。
- 交付 CI 在 `main` 上检查未跟踪构建产物，并验证文档链接。
- 发布通过 GitHub Release 附加六个平台二进制、SHA256 和第三方许可；已有标签不覆盖。
- Dependabot 针对 `dev` 更新 Go 模块、针对相关分支更新 GitHub Actions。

工作流使用最小权限和固定 action commit。标准 Go 目录已经符合项目边界，暂不移动核心模块或增加无必要的层级。
