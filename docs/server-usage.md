# 服务器与远程访问

当前 GUI 的访问控制入口位于顶部“设置”中的“访问控制”页签。“上传文件”始终存在；默认另显示“选择路径”，通过 `--show-file-picker` 或 `--show-path-input=false` 切换为部署机器“选择文件”入口。两个部署文件入口互斥，详见 [GUI 设置](gui-settings.md)。

## 无图形环境

服务器无需安装浏览器或桌面环境。浏览器在访问者的机器上运行。

```bash
chmod +x ./log-reviewer-linux-amd64
./log-reviewer-linux-amd64 gui --no-browser /var/log/myapp/app.log
# 默认 --host 0.0.0.0 --https-port 8766 --port 8765
```

Windows：

```powershell
.\dist\log-reviewer-windows-amd64.exe gui --host 0.0.0.0 --no-browser
```

默认优先使用 HTTPS（`--https-port 8766`），同时保留 HTTP（`--port 8765`），两种协议均监听全部 IPv4 网卡并尝试 IPv6。指定端口不可用时自动申请可用端口；`--port 0` / `--https-port 0` 动态分配。启动输出 `GUI` 为 HTTPS 地址，`HTTP` 为兼容地址，并输出各监听地址与 token。浏览器默认打开 HTTPS；以启动输出的最终端口为准。

### HTTPS 证书与 HTTP 兼容

首次启动自动创建二进制同目录的 `log-reviewer-tls.pem`，同时包含自签名证书和私钥；`--tls-file <路径>` 可指定可写位置。证书包含 localhost、回环地址、当前网卡 IP、`--host` 和 `--allowed-hosts` 中的地址。重启复用证书；新增地址或有效期不足一天时在启动时更新，证书有效期一年。公网 NAT 地址无法自动探测，需要使用受信任证书或显式配置对应地址。

自签名证书提供传输加密，但浏览器首次访问会提示证书不受信任；工具不会自动安装根证书或更改浏览器安全设置。需要无警告访问时，使用组织或公共 CA 签发且包含实际访问地址的证书：

```powershell
.\log-reviewer-windows-amd64.exe gui --tls-cert fullchain.pem --tls-key private-key.pem
# 仅 HTTPS，关闭 HTTP 兼容入口
.\log-reviewer-windows-amd64.exe gui --http=false --https-port 8766
```

TLS 最低版本为 1.2。指定证书和密钥必须成对提供，加载失败或证书过期会终止启动，不降级到 HTTP。自签名文件损坏也会报错；备份后移除该文件可重新生成。证书更新需重启服务。Unix 新建私钥文件权限为 0600；Windows 请限制目录 ACL。不要将私钥上传到仓库或发送给客户端。

HTTP 兼容入口不强制跳转，旧客户端可继续直接使用；它与 HTTPS 共用会话 token、白名单和 Host/Origin 校验。HTTP 传输没有加密，生产部署建议使用 `--http=false`。不同协议/端口的浏览器存储相互隔离，切换入口时使用包含 token 的完整启动 URL，或重新输入 token。未启用 HSTS，以保留 HTTP 兼容能力。

External 地址是本机网卡地址，可能包含私网、VPN、虚拟网卡；无法自动判断公网 NAT 地址或防火墙是否允许访问。IPv6 URL 用方括号，链路本地地址包含作用域；跨机器时作用域应改成客户端对应网卡，浏览器对这类地址支持有限，优先使用全局 IPv6。IPv6 不可用或相同端口已占用时输出原因，IPv4 仍正常服务。

## 路径、权限和空间

- “输入路径”使用工具所在服务器的路径。“导入文件”支持选择客户端多个文件或整个目录，拖放支持多个文件；导入会上传到服务器并产生临时副本。
- 服务器已有大日志优先直接选择路径，避免网络传输和额外磁盘占用。上传单文件上限 10 GiB。
- 进程只具备运行账号的文件权限。使用对日志目录有读取权限的专用账号；远程 token 持有者能够浏览该账号可访问的服务器目录和分析文件，当前没有目录隔离或多用户权限。
- 缓存保存索引、上传副本及导出临时文件；可指定已有且可写、空间充足的目录：`--cache-dir /data/log-reviewer-cache`。每次运行在其下新建会话子目录。
- 持久规则可指定 `--rule-dir /data/log-reviewer-rules`，适用于服务账号无 HOME、配置目录不可写等场景；目录需要读写权限。
- 默认退出删除会话缓存；`--keep-cache` 保留缓存，但不自动恢复 GUI 会话。重启生成新 token，需要新访问 URL。Linux SIGTERM 和 Ctrl+C 支持退出清理；强制终止可能留下缓存。

## 网络和反向代理

### 客户端 IP 白名单

支持单个 IPv4/IPv6 和 CIDR，仅按 TCP 连接来源判断，不信任 `X-Forwarded-For`、`X-Real-IP`。命令行配置会保存，下次启动自动加载：

```powershell
.\log-reviewer-windows-amd64.exe gui --allowed-client-ips "127.0.0.1,::1,192.168.10.25,10.20.0.0/24"
# 关闭限制并保存
.\log-reviewer-windows-amd64.exe gui --allowed-client-ips off
```

默认状态文件为二进制同目录的 `log-reviewer-state.json`；可用 `--state-file <路径>` 指定可写位置。初次启动无文件时允许全部来源；已有文件损坏、规则非法或保存失败会报错，不会静默放开。显式空参数无效，使用 `off` 关闭。IPv4 映射 IPv6 来源按 IPv4 匹配，映射形式的 CIDR 不接受，请使用 IPv4 CIDR。

GUI 顶部“访问控制”可配置并保存，保存成功立即应用到全部 HTTP 路由，包括已有连接的后续请求。GUI 禁止保存会排除当前连接的规则；远程误锁可在服务器本机通过 CLI 覆盖规则恢复。回环地址不会自动放行，需要显式列入。反向代理场景只能看到代理 IP，最终客户端限制应在代理层设置，且后端端口应避免被绕过直连。

全部 API（含访问控制配置）始终需要 `Authorization: Bearer <token>`。token 每次启动随机生成，不写入状态文件；浏览器可通过启动 URL 的 fragment 接收，或在访问控制窗口输入验证。具有 token 的允许来源可修改白名单，当前仍是单会话权限模型。状态文件应由运行账号独占写入；Unix 新建文件权限为 0600，Windows 应通过目录 ACL 限制写权限。同一二进制建议只运行一个共享该状态文件的实例。

白名单不能替代 HTTPS、VPN 或主机防火墙；HTTP 不提供 token 和日志传输加密。默认允许全部来源以兼容既有部署，服务器部署建议显式设置白名单。

默认暴露网卡是显式产品行为，API 始终要求会话 token；token 代表整个会话的操作权限。推荐使用受信任的 HTTPS 证书、内网或 VPN；避免直接公开 HTTP 兼容端口。

仅允许本机访问：

```bash
./log-reviewer-linux-amd64 gui --host 127.0.0.1 --port 8765 --no-browser
# 在客户端运行：
ssh -N -L 8765:127.0.0.1:<服务器HTTP最终端口> user@server
# 客户端打开 http://127.0.0.1:8765/#<服务器token>
```

仅监听 IPv6：`gui --host :: --no-browser`；指定网卡：`gui --host 192.168.1.20 --no-browser`。

域名或 HTTPS 代理需要白名单：

```bash
./log-reviewer-linux-amd64 gui --host 127.0.0.1 --no-browser --allowed-hosts logs.example.com
```

代理可转发到 HTTP 兼容端口，或校验证书后转发到 HTTPS 端口；根据启动输出选择上游协议与最终端口。保留原始 Host（含非默认端口）、Authorization 和 Origin。允许同源 HTTP/HTTPS Origin，拒绝跨站 Origin；不使用转发头自动绕过校验。支持多个白名单域名，以逗号分隔，不含协议、路径或端口。代理部署于站点根路径，当前不支持路径前缀。

远程访问失败依次检查：输出的最终端口、服务器监听、主机防火墙、云安全组、容器端口映射、路由/NAT、域名白名单及 token。动态端口可能与预先配置的容器映射或代理上游不一致；这类部署应确保期望端口空闲，并核实启动输出。IPv6 需单独确认路由和防火墙。

HTTP 非本机页面可能无法使用剪贴板 API 或浏览器流式保存文件：复制会弹出文本框供手动复制；大分类优先用 CLI export，或通过 HTTPS 访问。代理上传限制及超时需覆盖日志大小；不要把 10 GiB 日志经默认小上传限制代理上传。

## 优化建议

测试矩阵、运行命令和人工验收见 [HTTPS 与访问控制测试](security-testing.md)。

当前版本适合可信人员共享一个分析会话；不同浏览器共用分析结果和规则，重新分析会替换当前结果。

后续优先改进：

1. 增加 `--log-root` 目录边界和只读模式，限制 token 能读取及修改的范围。
2. 增加独立多会话或多用户权限，防止并行分析相互覆盖。
3. 增加离线报告加载及索引恢复，减少重启后重复扫描；区分“保留缓存”和“恢复会话”。
4. 增加跟随日志、轮转识别和增量分析；目前分析是快照，日志持续追加时需重新分析。
5. 增加稳定认证及部署探针；目前 token 每次启动变化，代理需要管理最终监听端口。
6. 增加 gzip/zstd 流式输入及远程导出流式下载，降低网络、磁盘和浏览器内存压力。

不需要交互界面时，直接使用 CLI analyze/query/export 输出 JSON 给 agent，省去监听端口、上传和浏览器。
