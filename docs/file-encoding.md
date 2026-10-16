# 文件编码处理与验证

## 支持范围

文件编码在 Go 后端统一处理。本机路径、目录展开、浏览器文件上传、SFTP 下载及远程直读共用日志引擎；上传和下载保留原始字节，解析和查询时解码。浏览器粘贴文本已经是 Unicode，无法恢复粘贴前被其他程序错误解码的内容。

| 输入 | 当前行为 | 主要场景 |
| --- | --- | --- |
| UTF-8、UTF-8 BOM | 优先识别，按 UTF-8 读取 | Linux、容器、现代应用日志 |
| GBK、GB18030 | 非 UTF-8 日志按 GB18030 解码，兼容 GBK | Windows 中文工具、数据库导出及历史日志 |
| LF、CRLF、末尾无换行 | 均支持，不因解码改变源文件 | 跨平台日志 |
| 带 BOM 的 UTF-16 LE/BE | 明确拒绝，需先转 UTF-8 | Windows Unicode 文本 |
| gzip | 根据文件头明确拒绝，需先解压 | 归档日志 |
| Big5、Shift_JIS、其他编码、同文件混合编码 | 没有可靠的自动识别支持，导入前统一转为 UTF-8 | 多地区或多来源拼接文件 |
| 外部 JSON 规则、语言规则覆盖、JSON 报告 | 必须是合法 UTF-8，允许 BOM；非法字节明确报错 | CLI 规则导入、规则目录、报告重新加载 |

UTF-16 的明确拒绝针对 BOM 签名；无 BOM 的 UTF-16、UTF-32 或二进制文件不保证能被准确识别并拒绝。自动识别不是文件格式校验器。

## 编码识别的边界

当前先检查文件起始最多 64 KiB 的样本，再在扫描各行解析前检查 UTF-8 有效性；发现不符合 UTF-8 的内容后，文件编码记录为 `GB18030`。纯 ASCII 内容按 UTF-8 处理，中文出现在样本之后的常规日志行也有回归覆盖。超长物理行仅解析有界前缀，不代表检查过该行的所有字节。

这是一种面向中文日志的回退策略，不是通用编码检测。合法 UTF-8 字节不证明文件原始编码一定是 UTF-8；Big5、损坏的 UTF-8 或混合编码也可能触发 GB18030 回退。当前没有统计检测置信度、手动编码选择或解码异常计数功能。

如果导入后仍有乱码，先在文本编辑器中确认实际编码，并另存为 UTF-8 副本再导入。不要覆盖唯一的原始日志。已被替换成 `�` 的字符无法仅靠切换编码还原，应重新取得原始文件。更新前的报告需要重新分析以刷新分类和编码元数据。

## 搜索、偏移与导出

- 解析、分类、关键词、正则和上下文使用解码后的文本；中文跨 64 KiB 读取边界也应可搜索。
- 每个文件的编码保存在报告 `sources[].encoding`，重新加载报告后按相同编码读取。
- 索引 `offset`、`length` 和事件 ID 始终对应源文件原始字节。截取原始范围后再解码，不能把 UTF-8 字符串长度当作源文件字节偏移。
- 事件文本导出和时间片段导出输出 UTF-8；GBK 文件的导出字节与源文件不同。原有换行保留，时间合并在文件边界缺少换行时补换行；UTF-8 BOM 可能随时间片段保留。
- 文件上传、SFTP 下载、源文件读取和内容摘要校验不改写源字节。JSON 报告输出为 UTF-8。

## 组件选型

| 组件 | 状态 | 用途与适用场景 |
| --- | --- | --- |
| Go 标准库 `unicode/utf8` 与 BOM 检查 | 已使用 | UTF-8 快速验证、JSON 严格编码检查、明确签名识别，无额外运行依赖 |
| `golang.org/x/text v0.41.0` | 已使用 | `encoding/simplifiedchinese` 负责 GBK/GB18030 到 UTF-8 转换，`transform` 支持流式解码，适合大文件和 SFTP；纯 Go，无需 CGO |
| `github.com/saintfish/chardet` | 仅候选，未引入 | 多地区、未知来源日志的统计编码检测；返回候选及置信度，需要真实样本评估并配合用户手动覆盖 |
| ICU / iconv 类方案 | 未引入 | 特殊编码很多且已有原生库运行环境的服务端批处理；当前单文件跨平台分发不优先选择 |

转换组件不能替代编码识别。`x/text` 解码器对部分非法字节会输出 Unicode 替换字符，而不一定返回错误。组件虽然提供 UTF-16 等解码能力，项目仍需适配换行与原始字节索引，不能因此宣称已支持这些输入。

参考：[Go 编码库](https://pkg.go.dev/golang.org/x/text/encoding)、[流式转换](https://pkg.go.dev/golang.org/x/text/transform)、[Go Charset Detector](https://github.com/saintfish/chardet)。许可见 [第三方组件](THIRD_PARTY_NOTICES.md)。

## 回归测试矩阵

| 测试 | 覆盖内容 |
| --- | --- |
| `TestLegacyChineseLog` | 固定 GBK 字节、本机与 reader 入口、分类、中文搜索、上下文、两类导出、原始偏移和源文件不变 |
| `TestEncodingBoundariesAndReports` | UTF-8 / GB18030 四字节字符、64 KiB 边界关键词与正则、上下文锚点、报告保存/加载 |
| `TestUTF8BOMAndLateGBK` | UTF-8 BOM、ASCII 前缀超过初始采样后出现 GBK 中文 |
| `TestEncodingLineEndingsAndMixedSources` | UTF-8 / BOM / GBK 同批分析，LF / CRLF / 无结尾换行，两类 UTF-8 导出与源字节不变 |
| `TestUnsupportedEncodingSignatures` | UTF-16 LE/BE BOM、gzip 签名的明确拒绝提示 |
| `TestJSONFileEncoding` / `TestReportRejectsLegacyJSONEncoding` | UTF-8 JSON、BOM、非法编码及报告读取拒绝 |
| `TestChineseImportAPIs` | HTTP 路径、目录、上传、分类、详情和中文筛选导出 |
| `TestSFTPChineseEncoding` | 本地真实 SSH/SFTP fixture，下载和直读 GBK 日志、中文搜索与正则、UTF-8 导出、原始字节长度及源文件保护 |

运行方式（Go 1.25 或以上）：

```sh
go test ./internal/logs ./internal/server ./internal/remote -run 'TestLegacyChineseLog|TestEncodingBoundariesAndReports|TestUTF8BOMAndLateGBK|TestEncodingLineEndingsAndMixedSources|TestUnsupportedEncodingSignatures|TestJSONFileEncoding|TestReportRejectsLegacyJSONEncoding|TestChineseImportAPIs|TestSFTPChineseEncoding' -count=1
go test ./... -count=1
go vet ./...
python scripts/check-doc-links.py
```

测试使用临时文件与本地 SFTP 服务，不需要生产服务器或外部网络。此矩阵不代表已完成真实生产文件、多地区编码识别或跨平台运行验证；历史性能数据见 [验证记录](VALIDATION.md)，本次编码用例不代替大文件性能基准。
