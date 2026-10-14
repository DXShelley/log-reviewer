# v0.7.0 验证记录

2026-10-14；Windows amd64；Go 1.25.3。

- `go test ./...`：新增自动范围、不同请求/堆栈边界、时间间隔、零范围、单条手动覆盖、关键词命中时完整堆栈与 HTTP 模式校验。
- `go vet ./...`、`go mod verify`。
- `tests/ui-design.cjs`：右上角布局与功能分组、文本和 JSON 日志着色、不可信 HTML 转义、自动范围与手动优先、复制原文、换行、超过 1 MiB 文本完整保留，以及 390/768/1280/1920 宽度检查。
- `ui-workspace`、`gui-smoke`、`ui-filters`、`ui-rules`、`server-host`、`ui-sftp` 回归；仍覆盖六种布局、全屏回退、下载、语言规则、SSH 和服务器访问。
- 六平台构建均低于 10 MiB；Windows amd64 约 7.77 MiB。GUI 组件精简产物约 30 KiB，运行时离线可用。

截图保存在本地 `reports/`。浏览器验证沿用 Playwright/CDP，使用独立 context；SFTP 使用本地 fixture。

限制：本轮未重新执行 1 GiB/10 GiB 性能基准，Linux/macOS 和 ARM64 未实机运行；Go race runtime 在本机有 `0xc0000139` 的已知环境问题，未声称通过。自动上下文为启发式，可能需要手动调整；每个预览只对前 256 KiB 着色，剩余内容保留明文。
