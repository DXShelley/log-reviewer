# Agent 接入

时间筛选的开始值包含该时刻；无小数的结束值包含所选秒或分钟内的全部事件，纯日期结束值包含当日。显式提供小数的 RFC3339 时间作为精确边界，例如 `to=2026-10-14T04:30:00.500Z`。时间筛选排除时间未知的事件；GUI 会将浏览器本地时间转换为带时区的 UTC 边界。

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

`query --details` 返回 `{details: [...], more: bool}`；详情包含完整位置、事件预览、before/after 上下文和 truncated 标志。单事件预览最多 2 MiB，导出不截断。支持 `--offset` 和 `--limit` 分页。上下文参数为 `--before 20 --after 100`（默认值），分别支持 0–10000 行。关键字或正则搜索以事件中首次匹配所在行为中心，返回 `context_line` 和 `context_text`；同时提供关键字和正则时优先以关键字为中心。无文本条件时仍按完整事件的前后边界读取。上下文每个方向最多显示 2 MiB，达到限制时返回 `context_truncated: true`。

## HTTPS / HTTP API

```sh
log-reviewer serve --no-browser --port 8765 --keep-cache
```

启动输出默认 HTTPS URL（`GUI:`，`--https-port 8766`）、HTTP 兼容 URL（`HTTP:`，`--port 8765`）、API token 和缓存目录；最终端口以输出为准。`--http=false` 关闭明文入口。默认自签名证书需客户端显式信任，或使用 `--tls-cert` / `--tls-key` 指定受信任证书。请求附带 `Authorization: Bearer <token>`，两个入口共用 token 和来源 IP 白名单；重启生成新 token，不写入状态文件。

默认监听全部 IPv4 并尝试 IPv6。仅本机监听用 `--host 127.0.0.1`；`--allowed-hosts logs.example.com` 限制访问域名，`--allowed-client-ips "127.0.0.1,10.0.0.0/8"` 限制实际连接来源，两者含义不同。来源限制不信任转发头。跨站 Origin、未允许的域名 Host 和未认证 API 会被拒绝。详见 [服务器说明](server-usage.md) 与 [安全测试矩阵](security-testing.md)。

| 方法与路由 | 参数 / 内容 |
| --- | --- |
| `GET /api/access-settings` | 返回 `allowed_client_ips` 和当前 TCP 连接的 `remote_addr` |
| `PUT /api/access-settings` | JSON `{ "allowed_client_ips": ["127.0.0.1", "10.0.0.0/8"] }`，保存并即时应用；空数组关闭限制 |
| `POST /api/analyze` | JSON `{paths: ["/path/app.log"], all: false, rules: []}`；返回 accepted，异步分析 |
| `GET /api/status` | busy、bytes、total、events、lines、error、ready；完成时附 report |
| `POST /api/cancel` | 取消正在进行的分析 |
| `GET /api/groups` | group、level、keyword、regex、from、to、source，返回筛选后的分类与次数 |
| `GET /api/details` | 同上，另有 offset（默认 0）、limit（默认 20，上限 100）、before（默认 20）、after（默认 100），范围 0–10000；context=auto 或 manual（默认），summary=1 仅返回摘要 |
| `GET /api/export` | 同上，返回符合条件的完整原始事件文本 |
| `GET /api/time-export` | from/to 必填，mode=merged 或 files，source 可选；返回原始时间片段文本或分文件 ZIP |
| `GET /api/report` | 完整 JSON 报告 |
| `GET /api/import-settings` | 返回 remote_directory、session_directory、系统默认保存目录、file_patterns、默认识别规则与 os |
| `PUT /api/import-settings` | JSON `{remote_directory: "绝对路径", file_patterns: ["*.log", "*.txt", "*.text"]}`；持久保存并迁移当前会话远程文件，更新分析与缓存引用；分析或获取中返回 409，迁移失败保留原文件与配置 |
| `POST /api/upload?name=...` | body 为原始文件，返回 upload id；analyze 的 uploads 可引用该 ID |
| `GET /api/paths` | 返回工具所在主机 hostname、系统 os、相对路径基准 working_directory；需要会话认证 |
| `POST /api/paths` | body 为 `{"paths":["C:\\logs\\*.log"]}`；校验并展开 1–1000 条本机路径，返回去重后的 paths 与主机上下文，不上传也不分析 |
| `POST /api/pick-paths?kind=files` | Windows 本机访问打开系统多文件选择器；`kind=directory` 选择目录，返回只读源路径，不复制文件；需要认证，远程访问拒绝 |
| `POST /api/remote-fetch` | SFTP 连接与选择配置，返回任务 id，异步获取 |
| `GET /api/remote-fetch` | 当前获取任务的 busy、stage、bytes、total、files、total_files、downloads、warnings、error |
| `DELETE /api/remote-fetch` | 取消当前获取 |
| `GET/PUT/DELETE /api/language-rules` | 语言列表、保存单语言、按 ?id 恢复/删除 |

`from/to` 包含端点；无时区时间按扫描主机本地时区理解，跨机器集成建议传带时区 RFC3339。`source` 为零基文件索引，缺省或 -1 代表全部。keyword 忽略大小写并匹配完整事件；regex 匹配逐物理行，使用 Go RE2 语法。

HTTP 非成功响应为 `{error: "..."}`。`401` 表示 token 无效；来源 IP、Host 或 Origin 被拒绝返回 `403`。白名单非法或会排除当前配置请求来源时返回 `400`，持久化失败返回 `500` 且不更改生效规则。GUI 和 API 保存至 `--state-file`，默认二进制同目录 `log-reviewer-state.json`，下次启动自动加载；同文件中的其他工具设置保留。只有持有 token 的允许来源可修改。

分析失败保留上一次成功报告，调用方必须检查 status.error 并确认本次任务完成，不能把旧报告误认成新分析结果。单实例同一时间只接受一个分析任务。

SFTP POST 示例：`{"host":"192.168.1.20","port":22,"user":"logreader","password":"...","fingerprint":"SHA256:...","path":"/var/log/myapp","pattern":"*.log\n*.txt\nregex:^app-\\d{4}\\.log$","recursive":false,"max_files":50,"max_bytes":10737418240}`。`pattern` 每行一条通配符或 `regex:` 正则，任一匹配即选中，兼容逗号分隔的通配符；`max_files` 默认 50。私钥用 private_key/passphrase；也可指定工具所在机器的 known_hosts 文件路径。不会保存凭据，不支持不验证主机密钥。

未知主机获取失败时，状态额外提供 `host_key: {host, fingerprint}` 供界面显示与核实；核实后可在下一次请求中传入该 `fingerprint`。不会自动信任或修改 known_hosts。显式填写的入口路径支持符号链接，目录扫描继续跳过子项中的符号链接。

GUI 两阶段流程：先向 `POST /api/remote-fetch` 提交连接配置和 `action:"preview"`，轮询直到 `stage:"preview"`，取得 `preview_id` 与 `preview_files:[{path,size,modified}]`。预览最多扫描 10000 个匹配文件，不下载内容；成功的空列表也可返回。再提交 `{action:"attach",preview_id,selected_paths:["远程路径"]}`，注册只读远程引用，返回的 `downloads` 字段保留兼容名称，实际不下载；将其中 `id` 传给分析接口的 `uploads`。详情、搜索、导出继续通过只读 SFTP 打开，网络错误时才缓存重试。拒绝空选择、重复路径、过期预览、超出数量/大小限制或不属于预览的路径。预览内存凭据 15 分钟有效；已加入的引用保留内存凭据至工具关闭，报告不包含凭据。`action:"download"` 和省略 `action` 的旧请求保持显式下载语义。

`GET /api/remote-hosts` 返回 `{hosts:[...]}`，包含成功连接后的非敏感配置，供下次回填；与访问控制配置一同保存至 `--state-file`，最多保留 20 条。响应和状态文件不包含密码、私钥内容或私钥口令。

`pattern` 还支持独立的 `exclude-dir:` 行，如 `*.log\nexclude-dir:backup\nexclude-dir:cache*`。`recursive=true` 时按各层级的目录名通配符跳过匹配目录及其全部内容；规则不排除显式选择的入口目录。

轮询时匹配任务 id，等待 busy=false，检查 error；仅成功后的 downloads[].id 可用于 analyze 的 uploads。失败/取消不返回不完整副本。HTTP 接口不会自动分析，GUI 的“获取后分析”由客户端发起下一次 analyze 请求。

报告 sources[].remote 与详情 remote 可追溯服务器、端口、远程路径、获取时间和原始修改时间。changed=true 表示获取期间文件变化，不能把它当成强一致快照。CLI 可直接 fetch --analyze 输出同样的报告给 agent。详见 [SFTP 说明](sftp-usage.md)。

## 摘要与时间片段接口

v0.7.0 详情查询增加 `context=auto|manual`，省略时保持手动行为；CLI 使用 `query --details --auto-context`。`before/after` 在自动模式下是扫描上限。详情增加 `context_mode`、`context_reason`、`context_before` 和 `context_after`，后两者为实际返回行数。GUI 默认请求自动模式，单条覆盖请求手动模式。算法边界和局限见 [工作区说明](workspace-usage.md)，不能把自动范围当作因果结论。原始时间片段导出不使用该裁剪。

`GET /api/details?...&summary=1` 返回同样的分页结构，但 `text/before/after` 为空，仅读取索引和每条最多 4 KiB，用 `key` 返回可识别的请求 ID。通过同样的筛选条件、`offset` 和 `limit=1` 再获取正文；客户端应核对 `source/offset` 防止异步查询变化。

`GET /api/time-export?from=RFC3339&to=RFC3339&mode=merged|files&source=-1` 导出原始时间片段，`merged` 返回文本，`files` 返回 ZIP。时间范围必填，`source` 可省略表示全部；忽略其他分类或关键字条件。包含 INFO 与无时间续行，按报告来源顺序合并。导出先写临时文件，读取失败返回错误而非部分成功下载。

SFTP 请求可增加 `ssh_dir`，未传指纹或 `known_hosts` 时自动读取该目录下的 `known_hosts`；省略目录使用工具运行账户的 `.ssh`。路径属于服务器本机，不属于 HTTP 客户端机器。

CLI `export-time --report report.json --from TIME --to TIME --out FILE` 或 `--out-dir NEW_DIR`，便于 agent 提取多节点证据。边界与无时间日志限制见 [工作区说明](workspace-usage.md)。

摘要中的 `key` 是可识别请求 ID 的原文，可能重复或缺省；不要把它当作唯一业务标识。可用 `source` 与字节 `offset` 联合定位事件，并保留报告、索引和稳定的源文件快照。

## 推荐分析顺序

1. analyze 输入的稳定日志快照，检查 exit code。
2. 读取严重级别、高频类别、首次时间、root_cause 与来源分布。
3. 对候选类别 query 3–10 条样本，检查堆栈和上下文。
4. 用明确时间窗口跨文件搜索相关异常，不把高频的连锁报错误判为最早原因。
5. export 确认的类别，形成带文件、行号、时间的证据；未知业务影响需要明确注明。

初版提供 CLI + HTTP 接入，没有内置 MCP server，也不调用模型 API。
