# 项目状态与维护入口

最后更新：2026-10-14；当前版本：v0.6.0。

## 当前交付

单个 Go 二进制提供 CLI 和浏览器 GUI，支持 Windows、Linux、macOS 的 amd64/arm64。六个平台构建产物位于 `dist/`，均低于 10 MiB，Windows amd64 约 7.72 MiB；校验值见 `dist/SHA256SUMS.txt`。版本可用 `log-reviewer version` 检查。

`dev` 保留完整源码、配置、脚本、文档、测试与开发资源；`main` 保留交付文档、样例、二进制。v0.6.0 标签对应交付版本。后续日常开发从 `dev` 开始，文档更新不移动已有版本标签。

## 已实现的关键能力

- 自动识别一种或多种语言，内置主流语言的独立 JSON 规则，GUI 按语言编辑、恢复和扩展。
- 错误/警告分类，统计次数、级别启发式影响、文件/logger 范围及首尾时间；查看完整堆栈和上下文。
- 多文件路径选择、上传、拖放和文本粘贴；关键字、正则、时间、文件与级别过滤，支持单项和全部重置。
- 六种工作区布局、默认折叠摘要、展开时按需读取、全屏与不支持原生全屏时的弹框回退。
- 总体上下文默认前 20、后 100 行，范围 0–10000；应用全部会清除单条覆盖。单条事件可独立查询或恢复总体。总体设置与布局保存在浏览器，单条覆盖仅保留在当前页面会话。
- 原始时间片段导出：GUI 合并文本或分文件 ZIP，CLI `export-time`；包含分析时未收集的 INFO 和时间续行，支持多节点同一时间窗口比较。
- 服务器无 GUI 时运行 `gui --no-browser`，默认监听 `0.0.0.0:8765` 并尝试 IPv6；端口占用时自动选空闲端口，输出最终访问地址。
- 内置 SFTP 获取文件/目录，支持密码/私钥、过滤、递归、限额、进度、取消、获取后分析和来源追踪。SSH 默认端口 22，指定 `.ssh` 目录自动读取 `known_hosts`。
- CLI JSON、分析报告和带 token 的 HTTP API 可供 agent 读取分类、样本、堆栈和时间窗口证据。

操作示例与边界见 [工作区说明](workspace-usage.md)、[服务器说明](server-usage.md)、[SFTP 说明](sftp-usage.md) 和 [Agent API](AGENT_API.md)。不将会话 token、密码、私钥或机器专用访问链接写入项目文档。

## 代码入口

| 路径 | 主要职责 |
| --- | --- |
| `cmd/log-reviewer/main.go` | CLI 路由、分析/查询、GUI 启动与访问地址 |
| `cmd/log-reviewer/fetch.go` / `time_export.go` | SFTP 命令和时间片段命令 |
| `internal/logs/engine.go` / `parser.go` | 流式分析、索引、日志头与时间识别 |
| `internal/logs/query.go` / `time_export.go` | 分类筛选、摘要、正文/上下文、原始时间扫描 |
| `internal/logs/rules/` | 每种语言的内置识别规则 |
| `internal/server/server.go` / `time_export.go` | 鉴权 HTTP API、分析任务、临时文件导出 |
| `internal/server/web/app.js` / `enhancements.js` | 基本 GUI、布局、拖放、折叠详情、上下文与全屏 |
| `internal/remote/sftp.go` / `internal/server/remote.go` | SSH 信任校验、下载及任务管理 |
| `scripts/build.ps1` / `build.sh` | 无 CGO 的发布构建 |

GUI 的静态资源通过 Go embed 打包；修改 HTML/CSS/JS 后必须重新构建并重启二进制，单纯刷新旧服务不会加载新资源。

## 验证与发布

```powershell
git switch dev
go test ./...
go vet ./...
go mod verify
.\scripts\build.ps1 -AllPlatforms
```

浏览器测试需要已有 Playwright、独立 Chrome 调试 Profile 和 `127.0.0.1:9222`，通过 `NODE_PATH` 配置包路径。依次运行：

```powershell
node tests/ui-workspace.cjs
node tests/gui-smoke.cjs
node tests/ui-filters.cjs
node tests/ui-rules.cjs
node tests/server-host.cjs
go build -o .work/sftp-fixture.exe ./tests/sftp-fixture
node tests/ui-sftp.cjs
```

测试使用独立浏览器 context；SFTP 测试使用本地 fixture。截图位于忽略版本管理的 `reports/`。v0.6.0 验证结果见 [验证记录](VALIDATION-0.6.0.md)。源码发布前先提交 `dev`，再通过临时 `main` worktree 同步交付文件，更新校验值，提交交付并创建新版本标签。

## 已知限制与后续重点

- 1 GiB 在 10 秒内分类不是任意输入下的保证；10 GiB 已有历史实测，性能受磁盘、日志格式、事件密度和唯一分类数影响。v0.6.0 没有重跑大文件基准，参见 [性能记录](VALIDATION.md)。
- Linux/macOS 与 ARM64 本轮仅交叉构建，未在相应实机运行。本机 Go race runtime 报 `0xc0000139`，不能声称 race 检查通过。
- 分类、影响和根因候选是辅助判断，不能据此推断受影响用户数或业务损失。请求 ID 也可能重复，必须结合源文件、时间和堆栈。
- 原始时间扫描使用内置日志头格式，不继承分析时临时传入的自定义头规则；无时间续行继承前一时间，开头无时间内容排除。特殊时间格式、跨节点时区和时钟偏差需预先处理。
- 时间片段按报告 `sources` 顺序合并，不进行跨文件时间排序。分文件名称带序号和起止时间，避免同名冲突；CLI 不覆盖已有输出文件或目录。
- 源文件追加、轮转或替换后需重新分析。报告、索引和源文件路径必须保留；GUI 导入与远程获取会产生缓存副本，导出也需要临时磁盘空间。
- 不支持流式保存的浏览器超过 256 MiB 时改用 CLI。UTF-16、gzip 日志需先转换或解压。
- 远程获取当前为内置 SFTP，未内置 FTP/FTPS，也不读取完整 SSH config 或自动选择私钥。未知主机仍需先核实并建立信任。
- Agent 通过 CLI/HTTP 接入，尚无内置 MCP server 或自动模型根因分析。

后续优先补充实际跨平台验证、自定义时间格式导出与大文件性能回归；实现前应保留以上明确边界。
