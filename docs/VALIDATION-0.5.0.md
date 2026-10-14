# v0.5.0 验证记录

2026-10-14，Windows amd64，Go 1.25.3。

## 已通过

- 修改前先重新构建 v0.4.0 的六平台二进制，保存于项目 .work/binaries-v0.4.0，包含 SHA256SUMS.txt。
- `go test ./...`、`go vet ./...`、`go mod verify` 通过。
- 本地真实 SSH/SFTP 测试服务器验证密码、私钥、加密私钥、known_hosts、错误指纹和错误密码、目录过滤与递归、数量和大小限制。
- 文件变化检测、截断、取消后清理、符号链接拒绝、递归深度限制及报告/详情来源追溯验证通过。
- `tests/ui-sftp.cjs` 验证桌面/手机布局、获取后分析、凭据输入清空、错误恢复、私钥文件选择、取消和 CLI fetch --analyze。
- `tests/server-host.cjs`、`tests/gui-smoke.cjs`、`tests/ui-rules.cjs`、`tests/ui-filters.cjs` 回归通过。
- Browser plugin not available，使用现有 Playwright 与独立 Chrome context；未关闭用户日常浏览器。
- 最终 CGO_ENABLED=0 六平台构建成功，Windows 7.57/6.94 MiB、Linux 7.26/6.81 MiB、macOS 7.41/6.94 MiB，均低于 10 MB（十进制）。

## 未完成与边界

- Windows race 检测测试进程启动失败，退出状态 `0xc0000139`（STATUS_ENTRYPOINT_NOT_FOUND），调整 MinGW DLL 搜索路径后仍失败；现有 logs 包也出现同样启动失败。未获得 race 验证结果，不能宣称无数据竞争。普通测试及实际二进制正常运行。
- 未连接用户的真实远程服务器，未在 Linux/macOS 实机运行。FTP/FTPS、跳板机、MFA、定时/增量同步和断点续传不在本版范围。
- 新依赖锁定 pkg/sftp v1.13.11、x/crypto v0.55.0，后者是本机 Go 1.25 可构建的版本；更新至需 Go 1.26 的版本应同步更新构建工具链。
- 大文件传输耗时与日志分析耗时分别计算；未新增远程 10 GiB 传输吞吐实测，磁盘总占用仍取决于累计获取和导出。

## 网络记录

- 原地址 `https://proxy.golang.org/github.com/pkg/sftp/@v/list` 连接超时；Xget `https://fast.dxshelley.fun/golang/github.com/pkg/sftp/@v/list` TLS 连接超时。
- 原地址 `https://proxy.golang.org/golang.org/x/crypto/@v/v0.43.0.mod` 连接超时；Xget 对应元数据请求成功。Xget 的 SFTP @latest 请求也发生 TLS 超时。
- 后续公开模块使用可用的 `https://goproxy.cn` 下载，依赖记录在 go.mod/go.sum，模块校验通过；未转发用户凭据或日志。
