# 安全问题

当前维护版本为 0.7.x，历史版本不承诺回补安全修复。

请通过 [GitHub 私密漏洞报告](https://github.com/DXShelley/log-reviewer/security/advisories/new) 提交安全问题，不要在公开 Issue 中附带凭据、生产日志或可直接攻击线上环境的细节。如果私密报告入口不可用，请先提交不含敏感细节的 Issue，请维护者开启报告入口。

报告应包含版本、平台、最小复现和可能影响。项目尚未提供固定响应时限。

GUI 默认监听 `0.0.0.0`，优先 HTTPS（8766），保留 HTTP（8765）；访问链接包含会话令牌。正式部署建议使用 `--http=false`，并通过 `--tls-cert` / `--tls-key` 提供受信任证书。默认自签名证书需客户端核实后信任，工具不会自动安装系统根证书。HTTP 入口仍是明文，IP 白名单不能提供传输加密。

`--allowed-client-ips` 或 GUI 可设置单 IP/CIDR 白名单，仅按实际连接来源判断，不信任转发头。两种协议的 API 均要求本次启动的 `Bearer` token；持有 token 的允许来源具有整个会话权限，包括修改白名单和访问运行账号有权限的文件。状态文件和 TLS 私钥应由运行账号独占写入，不提交到仓库。

服务器部署请参阅 [服务器使用说明](docs/server-usage.md)，测试范围见 [HTTPS 与访问控制测试](docs/security-testing.md)。SFTP 主机指纹校验不可跳过；不要将真实 token、主机凭据或私钥写入问题报告。
