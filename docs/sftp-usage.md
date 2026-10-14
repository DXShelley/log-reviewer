# SFTP 远程日志获取

支持按需获取远程服务器的单个文件或目录，并将完成的副本交给现有日志分析器。内置 `github.com/pkg/sftp` 与 `golang.org/x/crypto/ssh`，无需安装 OpenSSH 客户端或额外动态库。

## GUI

点击“远程获取”，填写服务器、SSH 端口、用户名、主机 SHA256 指纹及远程路径。选择密码或私钥认证；私钥支持选择本机文件或粘贴，支持加密私钥口令。

目录默认匹配 `*.log,*.log.*,*.txt`，多个模式用逗号分隔；可以包含子目录。明确指定单个文件时不使用文件名过滤。支持取消及进度展示；默认获取后自动分析当前已选择的所有文件。失败不会把不完整文件加入分析。

GUI 始终要求填写预先核实的指纹。请从服务器管理员或已可信的连接获取，例如在服务器检查实际启用的 host key：

```bash
ssh-keygen -lf /etc/ssh/ssh_host_ed25519_key.pub -E sha256
```

指纹应对应服务器实际协商使用的主机密钥类型。指纹不匹配时获取失败，不提供跳过验证选项。新服务器或轮换主机密钥，需要独立核实后再更新指纹。

## CLI

获取单文件，使用私钥并指定已核实的指纹：

```bash
./log-reviewer-linux-amd64 fetch \
  --host 192.168.1.20 --port 22 --user logreader \
  --path /var/log/myapp/app.log --key ~/.ssh/id_ed25519 \
  --fingerprint 'SHA256:<已核实的主机指纹>' --out-dir ./remote-logs
```

获取目录并分析，使用已有 OpenSSH known_hosts：

```bash
./log-reviewer-linux-amd64 fetch \
  --host 192.168.1.20 --user logreader --path /var/log/myapp \
  --key ~/.ssh/id_ed25519 --known-hosts ~/.ssh/known_hosts \
  --pattern '*.log,*.txt' --recursive --max-files 1000 \
  --max-bytes 10737418240 --out-dir ./remote-logs --analyze
```

不指定指纹或 known_hosts 时，CLI 尝试 `~/.ssh/known_hosts`，不存在、未知主机或密钥变化都拒绝连接。指纹与 known_hosts 同时提供时使用指纹。

Windows 密码认证：

```powershell
# 交互输入，不把密码放进命令参数或历史。
$credential = Get-Credential -UserName logreader
$env:LOG_REVIEWER_SFTP_PASSWORD = $credential.GetNetworkCredential().Password
try {
  .\dist\log-reviewer-windows-amd64.exe fetch --host 192.168.1.20 --user logreader --path /var/log/myapp --fingerprint 'SHA256:<已核实的主机指纹>' --recursive --analyze
} finally {
  Remove-Item Env:LOG_REVIEWER_SFTP_PASSWORD -ErrorAction SilentlyContinue
}
```

默认从 `LOG_REVIEWER_SFTP_PASSWORD` 和 `LOG_REVIEWER_SFTP_PASSPHRASE` 读取密码与私钥口令。可用 `--password-env`、`--passphrase-env` 指定其他环境变量名。GUI 与 CLI 不持久保存认证信息；GUI 在请求被接受后清空凭据输入。凭据使用后依赖进程内存回收，不承诺内存的即时清零。

CLI 在 out-dir 下为每次获取创建独立 `sftp-*` 目录，下载到 `.part` 临时文件，完成并校验后才改名为 `.log`。manifest.json 保存来源及副本位置；`--analyze` 同时保存 report.json 和索引。失败或取消时删除本次不完整下载目录；获取成功后分析失败仍保留下载文件，方便重试。

## 限制与来源追踪

- 默认最多 1000 个文件，可调整至 10000；每次获取总大小最多 10 GiB。缓存可随多次获取累计，需留足磁盘空间。
- 默认不递归，递归最多 32 层，总扫描目录项最多 100000；跳过目录项中的符号链接，明确选择的符号链接也拒绝。目录列表由组件先读取再检查数量，超大单目录仍有额外内存开销。
- 网络连接和 SSH 握手上限各 30 秒，传输期间无网络活动超过 30 秒会终止。持续有数据的大文件不受总时长 30 秒限制。可通过取消或 Ctrl+C/SIGTERM 主动终止。
- 下载以列目录时观察到的大小为界，不追逐持续增长的日志；文件截断则失败。下载后比较大小和修改时间，变化会记录警告及 changed=true。同大小同修改时间的改写或轮转无法可靠识别，因此不保证强一致快照。
- manifest.json、报告 `sources[].remote` 与详情 `remote` 包含协议、服务器、端口、原始路径、获取时间、原始修改时间和变化标志。报告 source.size 是实际下载字节数。
- GUI 副本及 manifest 存在会话缓存；可用 `--cache-dir` 调整位置，默认退出删除，`--keep-cache` 可保留。CLI 获取文件默认保留。
- 传输使用 SFTP 加密；浏览器向工具提交凭据的连接需另行保护。远程 GUI 优先用 HTTPS 或 SSH 隧道，普通 HTTP 会明文传输浏览器提交的认证信息。
- 无时区的日志时间仍按分析机器本地时区解析；多台服务器时区不同的日志应带明确时区，避免错误对齐事件。压缩日志不会自动解压，目录有 .gz 等文件时建议使用 `*.log,*.txt` 过滤。
- 连接由工具运行的机器发起，使用其网络；GUI 选择私钥文件来自浏览器所在机器。CLI 的 key、known-hosts 路径属于 CLI 所在机器。
- 当前同一实例只允许一个分析或远程获取任务。没有定时同步、断点续传、SSH agent、跳板机、ssh_config、任意远程命令或自动解压支持。

## 本版范围

仅支持 SSH 上的 SFTP 子系统。FTP/FTPS、vsftpd 服务端、旧 SCP 协议尚未支持；vsftpd 是 FTP/FTPS 服务端软件，并非独立协议。若服务器关闭 SFTP 子系统，联系管理员开启，或在服务器直接运行本工具，通过浏览器访问分析结果。

本版目标是可信人员的按需获取。后续按实际需要增加目录访问边界、独立会话、跳板机及轮转增量获取，不默认扩大认证与远程执行范围。
