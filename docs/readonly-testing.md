# 只读读取与路径兼容测试

本说明对应当前开发代码。历史版本验证记录保留在 [验证记录](VALIDATION.md)，不作为本轮测试通过的依据。

## 行为约定

| 场景 | 读取方式 | 是否生成源日志副本 |
| --- | --- | --- |
| Windows 本机直连，导入文件或目录 | 系统选择器返回源路径，后端只读打开 | 否 |
| 本机直连，系统不支持原生选择器 | 引导使用“输入路径” | 否 |
| 输入工具所在机器的路径 | 规范化、展开后只读打开 | 否 |
| 远程浏览器上传访问机器的文件 | 上传至工具缓存后读取 | 是，这是显式上传 |
| GUI 远程获取或 CLI `fetch --analyze` | 只读 SFTP 直接读取 | 正常连接时否 |
| SFTP 读取连接中断或超时 | 重新连接、核对预览元数据、缓存对应文件后重试 | 是，仅对应出错文件 |
| SFTP 认证、指纹、权限、路径或源文件变化错误 | 报错，修正后重试 | 不因此启动缓存下载 |
| CLI `fetch` 不带 `--analyze`、旧 API `download` | 用户显式下载 | 是 |

分析索引和报告仍写入工具工作目录，它们不属于源日志副本。只读保证针对源文件内容；操作系统可能更新访问时间。导入目录递归列出普通文件，最多 10000 个，不跟随目录符号链接；“输入路径”中的目录只读取当前层 `.log`、`.txt`、`.log.*`。

## 自动化用例

| 用例 | 覆盖范围 | 入口 |
| --- | --- | --- |
| 路径规范化及直接分析 | 普通路径、单双引号、JSON 转义字符串、中文、空格、方括号、`#`、文件 URI；分析前后内容、大小和修改时间一致 | `TestPathFormatsReadOnly` |
| 路径兼容边界 | `file://localhost/`、百分号编码、不透明 file URI；Windows 盘符、UNC；拒绝 URI 凭据、错误编码、NUL、非文件协议；断开的远程引用提示重新连接 | `TestNormalizePathCompatibility` |
| 路径展开与 API | 相对路径、当前层目录、通配符、方括号父目录、去重、认证与错误输入 | `TestExpandPathsUserInputs`、`TestLocalPathResolution` |
| 系统选择器访问限制 | IPv4/IPv6 loopback、外部地址、非法地址、平台能力；拒绝未认证、远程请求、错误方法，不信任 `X-Forwarded-For` 伪造本机来源 | `TestLocalPeerAndPickerAvailability`、`TestPickerAccessGuards` |
| 远程只读全链路 | 分析、详情、关键字查询和导出；正常 EOF 不缓存；注入读取中断后缓存重试；服务器断开后仍能读取缓存详情、按时间导出及再次分析；序列化报告需重新连接；源内容及修改时间不变 | `TestDirectSourceAndNetworkFallback` |
| 不应缓存的远程失败 | 预览后文件变化、文件删除、认证失败、错误指纹、取消操作均返回错误，不创建缓存 | `TestDirectSourceRejectsUnsafeFallback` |
| 网络错误识别 | 包装的网络错误、超时、SFTP 连接错误；权限、不存在、取消不视为网络错误 | `TestNetworkFailureClassification` |
| 远程引用 API | 重复预览和加入相同文件复用引用；分析、刷新后再次分析；时间导出文本/ZIP 内容及远程文件名；显示别名不改源文件；正常过程不生成日志副本；关闭时释放引用并取消连接上下文 | `TestRemoteAttachReadOnly` |
| 浏览器路径流程 | 原生选择器 API 响应模拟、无需上传、路径格式、添加失败保留输入、重复路径、桌面和手机布局 | `tests/ui-paths.cjs` |
| 浏览器及 CLI 远程流程 | 指纹确认后自动重试、预览及选择、主机记录、密码和私钥、分析、取消及 CLI | `tests/ui-sftp.cjs` |

Go 的 SFTP 测试使用本机 SSH/SFTP 测试服务和虚构凭据，不依赖真实服务器。网络缓存测试注入传输读取错误，并通过真实 SFTP 连接重试下载；不代表已经模拟了所有网络故障。权限拒绝覆盖错误分类，不声称在 Windows 上通过文件权限位模拟了远端权限拒绝。

缓存复用测试在成功缓存后关闭本地 SFTP 服务，再读取详情、导出时间片段和重新分析，确认可离线复用且缓存路径不变。报告重载测试仅序列化摘要与索引位置，不带进程中的读取回调，要求提示重新连接。正常远程时间导出通过 HTTP 同时校验文本与 ZIP，确保 ZIP 使用原始远程文件名。

## 运行方式

在仓库根目录执行，强制重跑避免缓存掩盖失败：

```sh
go test ./... -count=1 -timeout 5m
go vet ./...
python scripts/check-doc-links.py
git diff --check
```

仅运行本轮核心用例：

```sh
go test ./internal/logs ./internal/remote ./internal/server -count=1 -timeout 2m -run 'TestPathFormatsReadOnly|TestNormalizePathCompatibility|TestLocalPeerAndPickerAvailability|TestPickerAccessGuards|TestDirectSource|TestNetworkFailureClassification|TestRemoteAttachReadOnly'
```

浏览器检查需要 Node.js、Playwright 和独立 Chrome 调试实例，CDP 地址为 `http://127.0.0.1:9222`。Windows PowerShell 示例，构建至临时目录，不提交二进制或测试截图：

```powershell
$env:LR_TEST_BINARY = Join-Path $env:TEMP 'log-reviewer-readonly-tests.exe'
$env:LR_SFTP_FIXTURE_BINARY = Join-Path $env:TEMP 'log-reviewer-sftp-fixture.exe'
go build -o $env:LR_TEST_BINARY ./cmd/log-reviewer
go build -o $env:LR_SFTP_FIXTURE_BINARY ./tests/sftp-fixture
node tests/ui-paths.cjs
node tests/ui-sftp.cjs
```

Playwright 不在默认模块路径时，先将 `NODE_PATH` 指向本机安装目录。`LR_SFTP_FIXTURE_BINARY` 指定本地 SSH/SFTP 测试服务；未设置时旧脚本默认使用 `.work/sftp-fixture.exe`。浏览器脚本各自启动测试服务并清理进程，截图写入临时目录。其他验证入口参见 [开发说明](development/README.md)。

## 人工验收与限制

1. Windows 本机打开“导入文件”，分别选择多个日志和目录，确认真实系统窗口能选择、取消，选择后可分析。自动化使用 API 模拟，不操作系统选择器窗口，不能替代此项。
2. 记录源文件 SHA256、大小和修改时间，执行分析、搜索、详情及导出，再次比较；源目录不应出现工具生成的新文件。读取可能更新访问时间，因此不以访问时间不变作为验收条件。
3. 输入 `"C:\logs\app.log"`、`file:///C:/logs/app%20one.log`、目录及通配符，核对匹配列表。使用实际共享目录验证 UNC，纯路径转换测试不证明网络共享可达。
4. 在授权的远程主机上验证直读：正常分析后工作目录只有索引、报告，没有日志副本。断网后恢复连接，检查错误或缓存重试结果；源文件变化应提示重新预览。
5. 从另一台机器访问工具，确认导入文件仍走浏览器上传，系统选择器接口返回拒绝；反向代理可能使实际来源显示为本机，代理部署应使用“输入路径”。
6. 关闭并重启工具后，远程报告详情需要重新连接并分析；报告和状态文件不保存密码或私钥内容。

本机历史 `go test -race` 因运行环境问题以 `0xc0000139` 退出，未完成竞态验证。跨平台测试状态以实际执行结果为准，不把交叉编译当作 Linux/macOS 原生窗口验收。
