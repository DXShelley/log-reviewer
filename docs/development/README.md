# 开发说明

## 分支与目录

`dev` 是完整源码分支，也是贡献 PR 的目标；GitHub 默认分支 `main` 提供交付文件。查看代码请切换至 [dev](https://github.com/DXShelley/log-reviewer/tree/dev)。保留此约定，避免把开发缓存和测试资源带入交付面。

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

已有测试通过 Playwright 连接本机 Chrome `127.0.0.1:9222`，使用独立浏览器 Profile。设置 `NODE_PATH` 指向安装的 Playwright 后执行 `node tests/ui-design.cjs`、`node tests/ui-workspace.cjs`、`node tests/gui-smoke.cjs`。更多入口见 [项目状态](../PROJECT_STATUS.md)。浏览器测试目前属于本地验证，不宣称已在 GitHub CI 中运行。

## 自动化与发布

- 开发 CI 在 `dev` 的 push/PR 上运行 Linux、Windows、macOS 的 Go 测试、vet 和格式检查，并检查模块完整性。
- 交付 CI 在 `main` 上验证六个平台文件、SHA256、体积和文档链接。
- 发布通过 GitHub Release 附加六个平台二进制、SHA256 和第三方许可；已有标签不覆盖。
- Dependabot 针对 `dev` 更新 Go 模块、针对相关分支更新 GitHub Actions。

工作流使用最小权限和固定 action commit。标准 Go 目录已经符合项目边界，暂不移动核心模块或增加无必要的层级。
