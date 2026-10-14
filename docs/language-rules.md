# 自动语言识别

无需预先选择语言。每个事件的报错正文与堆栈会按独立语言规则识别，文件可以包含多种语言。报告的 `sources[].languages` 与 `groups[].languages` 可供 agent 使用。空数组表示证据不足；普通 ERROR、TypeError 或通用 JSON 本身不能可靠证明语言。

内置规则覆盖 Java、Kotlin、Scala、Python、JavaScript、TypeScript、Go、C#、Rust、PHP、Ruby、C、C++、Swift、Dart、Elixir、Lua。每种语言的规则位于 `internal/logs/rules/<语言>.json`。

导出内置规则：

```powershell
./log-reviewer.exe languages > languages.json
./log-reviewer.exe analyze --language-rules languages.json --out report.json app.log
```

规则文件为 JSON 数组。`id` 是语言标识，`evidence` 是针对完整事件的 Go 正则表达式，`start` 是可选的裸异常起始行表达式，`level` 是该起始行的级别，`warning` 是警告起始行表达式，`root` 是根因提取表达式（取最后一个匹配，可使用 `(?m)` 多行锚点）。相同 ID 覆盖内置规则，新 ID 添加语言。只需提供要修改的语言。修改后重新分析生效。

GUI 的“识别规则”以语言列表展示，选择某一种语言后，分别编辑语言识别、异常起始、级别、警告起始和根因提取规则。可单独保存、恢复内置规则，也可新增或删除自定义语言。切换语言保留未保存草稿。无效正则不会覆盖已保存内容。修改后重新分析生效。

保存配置按语言分别存放为 `<id>.json`，Windows 默认 `%APPDATA%/log-reviewer/rules`，Linux 默认 `$XDG_CONFIG_HOME/log-reviewer/rules`（未设置时为 `~/.config/log-reviewer/rules`），macOS 默认 `~/Library/Application Support/log-reviewer/rules`。GUI 和 CLI analyze 自动加载。GUI 可用 `--rule-dir` 指定目录；CLI `--language-rules` 优先于持久配置。

HTTP `/api/language-rules` 支持 GET 列表、PUT 单语言规则、DELETE `?id=java` 恢复或删除规则，使用与其他接口相同的鉴权。HTTP `/api/analyze` 仍支持 `languages` 数组，显式请求规则优先。通用日志头规则保留在界面的独立折叠区。

识别依赖实际日志特征，并非源码分析。共享 JVM/JS 特征可能含糊；Kotlin/Scala/TypeScript 堆栈优先于共享特征。每个事件用于分类和识别的正文上限为 64KiB，完整日志仍可查看和导出。当前仅统计已收集事件的语言，不用普通 INFO 推断整个文件。Sanitizer 规则归入 C，但 Sanitizer 也能来自 C++，需结合 `.cpp` 堆栈判断。
