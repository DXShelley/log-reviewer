# v0.4.0 验证记录

2026-10-14，Windows amd64 实测。

- Go 单元测试、静态检查通过；IPv4/IPv6 URL、端口边界、Host 白名单、同源 Origin、跨站拒绝、token 校验覆盖。
- 服务器集成测试通过：占用指定端口后自动分配，默认 wildcard 监听，本机 IPv6 访问，网卡 IPv4 地址浏览器上传与分析。
- GUI smoke、规则管理、筛选回归测试通过。
- 六平台 amd64/arm64 构建小于 10 MiB；Linux/macOS 未在真实系统运行测试。
- 公网 NAT、防火墙、HTTPS 代理及容器部署未实机验证；文档列出设置方法和限制。
