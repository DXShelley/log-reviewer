# Agent 接入

## 首选：terminal JSON

```sh
log-reviewer analyze --out report.json app.log worker.log
log-reviewer query --report report.json --level ERROR
log-reviewer query --report report.json --group a888c34a40347801 --details --limit 3
log-reviewer export --report report.json --group a888c34a40347801 --out evidence.txt
```

分类 ID 以实际报告为准。分类摘要本身就是候选原因和证据入口；agent 应读取实际堆栈、时间窗口和其他分类，确认同一因果链，避免把所有 WARN 都当根因。

报告 `schema = log-reviewer/v1`，主要字段：

| 字段 | 用途 |
| --- | --- |
| `sources[]` | 源文件绝对路径、字节大小、修改时间、行数、事件数 |
| `groups[].id` | 根据分类指纹生成的稳定 ID，不依赖扫描顺序 |
| `groups[].title` / `root_cause` | 归一化标题、原始根因候选 |
| `groups[].count` / `first` / `last` / `unknown_time` | 频次、时间、无时间事件数 |
| `groups[].level` / `impact` / `impact_basis` | 原始级别与透明启发式评级 |
| `groups[].sources` / `loggers` | 在当前输入中观察到的范围 |
| `groups[].samples[]` | 前 3 个样本的 `source`、`offset`、`length`、`line`、`time` |
| `index_path` | 供后续 query/export 复用的本机索引路径 |

`samples.source` 是 `sources` 的零基索引；offset/length 的单位是字节，line 是从 1 开始的物理行号。无需理解索引格式也能按样本位置直接读取源文件。时间为 RFC3339；未识别到时间时字符串缺省/空，数字时间 0 代表未知。

`query --details` 返回 `{details: [...], more: bool}`；详情包含完整位置、事件预览、before/after 上下文和 truncated 标志。单事件预览最多 2 MiB，导出不截断。支持 `--offset` 和 `--limit` 分页。

## 本机 HTTP API

```sh
log-reviewer serve --no-browser --port 8765 --keep-cache
```

启动输出 URL、API token 和缓存目录。请求只走本机直连，附带 `Authorization: Bearer <token>`。服务仅监听 127.0.0.1；跨站 Origin、非 127.0.0.1 Host 和未认证 API 会被拒绝。

| 方法与路由 | 参数 / 内容 |
| --- | --- |
| `POST /api/analyze` | JSON `{paths: ["/path/app.log"], all: false, rules: []}`；返回 accepted，异步分析 |
| `GET /api/status` | busy、bytes、total、events、lines、error、ready；完成时附 report |
| `POST /api/cancel` | 取消正在进行的分析 |
| `GET /api/groups` | group、level、keyword、regex、from、to、source，返回筛选后的分类与次数 |
| `GET /api/details` | 同上，另有 offset（默认 0）、limit（默认 20，上限 100） |
| `GET /api/export` | 同上，返回符合条件的完整原始事件文本 |
| `GET /api/report` | 完整 JSON 报告 |
| `GET /api/browse?path=...` | 本机文件选择；path=roots 列磁盘 / 根目录 |
| `POST /api/upload?name=...` | body 为原始文件，返回 upload id；analyze 的 uploads 可引用该 ID |

`from/to` 包含端点；无时区时间按扫描主机本地时区理解，跨机器集成建议传带时区 RFC3339。`source` 为零基文件索引，缺省或 -1 代表全部。keyword 忽略大小写并匹配完整事件；regex 匹配逐物理行，使用 Go RE2 语法。

HTTP 非成功响应为 `{error: "..."}`。分析失败保留上一次成功报告，调用方必须检查 status.error 并确认本次任务完成，不能把旧报告误认成新分析结果。单实例同一时间只接受一个分析任务。

## 推荐分析顺序

1. analyze 输入的稳定日志快照，检查 exit code。
2. 读取严重级别、高频类别、首次时间、root_cause 与来源分布。
3. 对候选类别 query 3–10 条样本，检查堆栈和上下文。
4. 用明确时间窗口跨文件搜索相关异常，不把高频的连锁报错误判为最早原因。
5. export 确认的类别，形成带文件、行号、时间的证据；未知业务影响需要明确注明。

初版提供 CLI + HTTP 接入，没有内置 MCP server，也不调用模型 API。
