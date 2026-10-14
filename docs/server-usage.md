# 服务器与远程访问

## 无图形环境

服务器无需安装浏览器或桌面环境。浏览器在访问者的机器上运行。

```bash
chmod +x ./log-reviewer-linux-amd64
./log-reviewer-linux-amd64 gui --no-browser /var/log/myapp/app.log
# 等价于 --host 0.0.0.0 --port 8765
```

Windows：

```powershell
.\dist\log-reviewer-windows-amd64.exe gui --host 0.0.0.0 --no-browser
```

默认监听全部 IPv4 网卡，并尝试在同一端口监听 IPv6。默认端口 8765，指定端口不可用时自动申请可用端口；`--port 0` 直接动态分配。启动输出最终 Listen、Internal IPv4、Internal IPv6、各网卡 External IPv4/IPv6 地址与 token。直接复制完整 URL 到其他机器浏览器。

External 地址是本机网卡地址，可能包含私网、VPN、虚拟网卡；无法自动判断公网 NAT 地址或防火墙是否允许访问。IPv6 URL 用方括号，链路本地地址包含作用域；跨机器时作用域应改成客户端对应网卡，浏览器对这类地址支持有限，优先使用全局 IPv6。IPv6 不可用或相同端口已占用时输出原因，IPv4 仍正常服务。

## 路径、权限和空间

- “选择文件”浏览服务器文件系统；“输入路径”也使用服务器路径。“导入文件”和拖放选择客户端文件并上传到服务器，产生临时副本。
- 服务器已有大日志优先直接选择路径，避免网络传输和额外磁盘占用。上传单文件上限 10 GiB。
- 进程只具备运行账号的文件权限。使用对日志目录有读取权限的专用账号；远程 token 持有者能够浏览该账号可访问的服务器目录和分析文件，当前没有目录隔离或多用户权限。
- 缓存保存索引、上传副本及导出临时文件；可指定已有且可写、空间充足的目录：`--cache-dir /data/log-reviewer-cache`。每次运行在其下新建会话子目录。
- 持久规则可指定 `--rule-dir /data/log-reviewer-rules`，适用于服务账号无 HOME、配置目录不可写等场景；目录需要读写权限。
- 默认退出删除会话缓存；`--keep-cache` 保留缓存，但不自动恢复 GUI 会话。重启生成新 token，需要新访问 URL。Linux SIGTERM 和 Ctrl+C 支持退出清理；强制终止可能留下缓存。

## 网络和反向代理

默认暴露网卡是显式产品行为，API 始终要求会话 token。仅 HTTP，不内置 TLS；token 代表整个会话的操作权限。推荐内网、VPN、SSH 隧道或 HTTPS 反向代理，避免直接公开 HTTP。

仅允许本机访问：

```bash
./log-reviewer-linux-amd64 gui --host 127.0.0.1 --port 8765 --no-browser
# 在客户端运行：
ssh -N -L 8765:127.0.0.1:<服务器最终端口> user@server
# 客户端打开 http://127.0.0.1:8765/#<服务器token>
```

仅监听 IPv6：`gui --host :: --no-browser`；指定网卡：`gui --host 192.168.1.20 --no-browser`。

域名或 HTTPS 代理需要白名单：

```bash
./log-reviewer-linux-amd64 gui --host 127.0.0.1 --no-browser --allowed-hosts logs.example.com
```

代理转发到最终输出的监听端口，保留原始 Host（含非默认端口）、Authorization 和 Origin。允许同源 HTTP/HTTPS Origin，拒绝跨站 Origin；不使用转发头自动绕过校验。支持多个白名单域名，以逗号分隔，不含协议、路径或端口。代理部署于站点根路径，当前不支持路径前缀。

远程访问失败依次检查：输出的最终端口、服务器监听、主机防火墙、云安全组、容器端口映射、路由/NAT、域名白名单及 token。动态端口可能与预先配置的容器映射或代理上游不一致；这类部署应确保期望端口空闲，并核实启动输出。IPv6 需单独确认路由和防火墙。

HTTP 非本机页面可能无法使用剪贴板 API 或浏览器流式保存文件：复制会弹出文本框供手动复制；大分类优先用 CLI export，或通过 HTTPS 访问。代理上传限制及超时需覆盖日志大小；不要把 10 GiB 日志经默认小上传限制代理上传。

## 优化建议

当前版本适合可信人员共享一个分析会话；不同浏览器共用分析结果和规则，重新分析会替换当前结果。

后续优先改进：

1. 增加 `--log-root` 目录边界和只读模式，限制 token 能读取及修改的范围。
2. 增加独立多会话或多用户权限，防止并行分析相互覆盖。
3. 增加离线报告加载及索引恢复，减少重启后重复扫描；区分“保留缓存”和“恢复会话”。
4. 增加跟随日志、轮转识别和增量分析；目前分析是快照，日志持续追加时需重新分析。
5. 增加可选 TLS、稳定认证及部署探针；目前 token 每次启动变化，代理需要管理最终监听端口。
6. 增加 gzip/zstd 流式输入及远程导出流式下载，降低网络、磁盘和浏览器内存压力。

不需要交互界面时，直接使用 CLI analyze/query/export 输出 JSON 给 agent，省去监听端口、上传和浏览器。
