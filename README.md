# Log Reviewer

v0.2.0 新增自动多语言识别：内置 17 种独立语言规则，允许外部 JSON 修改和添加，分类直接显示识别语言。详见 [语言规则说明](docs/language-rules.md) 和 [本轮验证记录](docs/VALIDATION-0.2.0.md)。

实用、简洁的本地日志分类工具。单个二进制同时提供 terminal 和浏览器 GUI，无需安装运行时、数据库或日志平台。日志不会发送到云端。

## 直接使用

筛选区域提供关键字、正则、时间、文件范围的独立重置和全部重置。时间弹窗选择日期、时、分、秒后点“确认”提交，点“取消”保留原值。结束时间包含所选秒内的全部日志。GUI 使用浏览器所在时区；CLI 的无时区时间使用运行机器的本地时区。

“选择文件”支持跨目录勾选多个文件；“导入文件”支持一次选择多个文件，也支持拖入多个文件。“粘贴文本”可添加复制的日志内容，这些输入可以混合分析。

Windows 双击 `dist/log-reviewer-windows-amd64.exe`，会自动打开本机浏览器界面。分析完成后，在左侧选分类，右侧查看完整事件、Java 堆栈和上下文。默认前 20 行、后 100 行，可在详情面板分别设置为 0–10000 行。关键字搜索以事件中首次匹配所在行为中心；只有正则时以首次正则匹配行为中心；没有文本搜索条件时以完整事件为中心。

Linux / macOS 使用对应平台的文件，先执行 `chmod +x log-reviewer-*`，然后运行。GUI 使用系统浏览器；terminal 在无图形环境中也可用。

界面支持：多文件选择、直接输入路径/通配符、文件导入、级别/文件/时间/关键字/正则过滤、按次数/影响/最后时间排序、分类与内容复制、分类导出和 JSON 报告导出。大文件优先用「选择文件」直接读本机路径；「导入文件」和拖放会产生一份临时副本。

示例：

```powershell
.\dist\log-reviewer-windows-amd64.exe gui .\samples\springboot.log .\samples\multilanguage.log
.\dist\log-reviewer-windows-amd64.exe analyze --out report.json "D:\logs\*.log"
.\dist\log-reviewer-windows-amd64.exe query --report report.json --level ERROR
.\dist\log-reviewer-windows-amd64.exe query --report report.json --keyword "Connection refused" --details
.\dist\log-reviewer-windows-amd64.exe query --report report.json --from "2026-10-14T12:30:00+08:00" --to "2026-10-14T13:00:00+08:00"
.\dist\log-reviewer-windows-amd64.exe export --report report.json --group <分类ID> --out errors.txt
```

所有参数放在文件路径之前。`analyze` / `query` 将 JSON 写入 stdout，提示和错误写入 stderr，错误时 exit code 为 1。指定目录时读取当前层 `.log`、`.log.*`、`.txt` 文件；也可直接指定任意扩展名的普通文本文件。重复路径自动去重。

## 内置识别与分类

识别以实际日志格式为准，而不是只根据文件扩展名猜测语言：

| 常见来源 | 支持内容 |
| --- | --- |
| Java / Spring Boot / Logback / Log4j | ISO/本地时间、级别、logger、连续堆栈、深层 `Caused by` |
| Python logging | 逗号毫秒时间、WARNING、logger、traceback；裸 traceback 也可识别 |
| Node.js Pino / Winston | JSON level/msg/time、数字级别、嵌套 err.stack |
| Go slog / logrus / zap | 文本 / JSON、秒或毫秒时间、panic 多行堆栈 |
| .NET Serilog / Microsoft logging | JSON / compact JSON、Error/Warning、fail/warn、异常堆栈 |
| Rust tracing / 常见 C/C++ 日志 | ISO 时间、ERROR/WARN、glog 的 E/W 前缀 |
| Nginx 等系统日志 | 年/月/日时间、括号 error/warn；无年份的 syslog 时间需自定义规则 |

默认收集 WARN、ERROR、FATAL，界面复选框或 `--all` 可纳入 INFO / DEBUG / TRACE。首尾时间按日志时间统计；无可识别时间的事件标记为「时间未知」，时间范围筛选会排除这些事件。

分类指纹由级别、logger 和归一化根因/消息组成。优先采用最后一层 `Caused by`；屏蔽 UUID、长十六进制 ID、数值及常见耗时单位，合并同类不同订单/请求。数字错误码也可能被归一化，必须结合原始事件判断；分类是定位辅助，不是根因证明。

影响程度由级别给出透明的启发式评级：WARN 中、ERROR 高、FATAL 严重。影响范围统计实际出现的文件和 logger/service，**不推断受影响用户数或业务损失**。

## 动态添加格式

在 GUI「识别规则」中粘贴 JSON，或使用：

```powershell
.\dist\log-reviewer-windows-amd64.exe analyze --rules .\samples\rules.json --out report.json custom.log
```

规则采用 Go 正则，必须有 `level` 和 `message` 命名组，可增加 `time`、`logger`；`time_layout` 使用 Go 时间模板（例如 `2006/01/02 15:04:05`）。自定义规则优先于通用文本识别；JSON 内置识别先执行。不需要重新编译。

## Agent 对接

稳定的 JSON stdout、带 schema 的报告、本机 HTTP API 和事件字节位置可以直接用于 agent。报告包含根因候选、分类次数、首尾时间、来源文件和每类最多 3 个样本位置。接入方法见 [docs/AGENT_API.md](docs/AGENT_API.md)。

## 大文件与边界

- 流式扫描，不把整份日志加载进内存。40 字节/匹配事件的索引写入磁盘；100 万事件约 38.2 MiB 索引。需要足够的可写缓存空间。
- 无固定的输入文件大小上限；已实测 10GiB 文件。性能记录见 [docs/VALIDATION.md](docs/VALIDATION.md)，结果受磁盘、格式、堆栈长度、唯一分类数量影响，不保证任何输入都能 10 秒完成。
- GUI 详情每次 10 条，单条预览前 2 MiB；terminal `query --details` 每页最多 100 条。完整事件可通过导出获取，包括超长行与堆栈。
- 每条事件使用前 64 KiB 构造指纹，并额外保留可识别的深层 Java cause。超长事件可能归入较粗的分类。分类上限 100,000，超出明确报错，不悄悄丢弃。
- UTF-8 / UTF-8 BOM、LF / CRLF；ANSI 颜色在识别时移除，导出保留原文。UTF-16、gzip 需要先转换/解压，初版会明确拒绝。
- 关键字匹配完整事件，忽略大小写；正则逐物理行匹配，单行最多 1 MiB，超出明确报错。使用 Go RE2 语法，不支持反向引用和 lookbehind。
- 正在写入的活动日志先复制为稳定快照。扫描及详情读取会检查文件大小和修改时间；变化后要求重新分析，不使用过期字节位置。
- GUI 使用 `127.0.0.1`，带每次启动生成的 session token。Chrome / Edge 可流式保存大导出；其他浏览器小文件可直接下载，大于 256 MiB 的导出请用 terminal。
- 报告和 `.idx` 都要保留，源文件位置不可移动；它们是本机可复用的索引，不是完整日志副本。GUI 默认退出后清理会话缓存，`--keep-cache` 可保留。
- GUI 默认端口 8765，被占用时自动选择空闲端口。关闭浏览器不会关闭服务，在控制台按 Ctrl+C 退出。

## 开发与构建

仅依赖 Go 标准库，Go 1.23+。源码、文档、测试、样例、脚本都在 `dev` 分支；`main` 为交付说明面。

开发前执行 `git switch dev`。`main` 只包含交付说明、样例和六个平台二进制。

```powershell
go test ./...
go vet ./...
.\scripts\build.ps1 -AllPlatforms
```

Linux / macOS 可运行 `sh scripts/build.sh`。产物覆盖 Windows / Linux / macOS 的 amd64 和 arm64；`CGO_ENABLED=0`，无需外部动态库。

实测生成工具：`go run ./tools/bench --dir <有足够空间的临时目录> --gib 1`，或加 `--dense` 生成全 ERROR 日志。生成的文件是真实写入的日志，不是稀疏文件。

GUI 自动验证：配置 `NODE_PATH` 指向已有 Playwright 包，运行 `node tests/gui-smoke.cjs`；连接 `127.0.0.1:9222` Chrome，创建独立浏览器 context。
