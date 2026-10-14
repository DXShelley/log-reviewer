# v0.6.0 验证记录

2026-10-14，Windows amd64，Go 1.25.3。

## 自动验证

- `go test ./...`：时间导出包含未收集的 INFO、纳秒精度、续行堆栈、乱序、超长行、取消、源文件变化、摘要分页、合并/ZIP API、CLI 分文件和防覆盖。
- `go vet ./...`、`go mod verify`。
- SFTP 本地真实测试验证指定 `.ssh/known_hosts`、已知主机匹配与错误拒绝；仍覆盖密码、私钥及取消。
- `tests/ui-workspace.cjs`：多文件拖放、默认折叠与按需读取、请求 ID、总体/单条上下文、六种布局、原生全屏与弹框回退、合并文本与 ZIP 下载、390px 手机无横向溢出。
- `tests/gui-smoke.cjs`、`tests/ui-filters.cjs`、`tests/ui-rules.cjs`、`tests/server-host.cjs`、`tests/ui-sftp.cjs` 回归。
- 六个平台构建，所有二进制低于 10 MiB；Windows amd64 约 7.72 MiB。

截图由浏览器测试保存在本地 `reports/`。GUI 测试使用独立浏览器 context 和本地测试 SFTP 服务器。Browser plugin not available，沿用 Playwright/CDP。

## 限制

- Linux/macOS 与 ARM64 已交叉构建，未在对应实际机器上运行。
- 本轮未重复 1 GiB / 10 GiB 分析性能基准；分析核心未改变，历史基准见 VALIDATION。
- 本机 Go race runtime 启动报 `0xc0000139`，没有声称 race 检查通过，详见 v0.5.0 记录。
- 无可识别时间的独立行会继承此前时间；特殊自定义日志头时间需先转换。分文件顺序由报告 sources 决定。
