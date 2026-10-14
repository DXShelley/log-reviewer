# 版本记录

## 0.1.0 · 2026-10-14

- 单文件二进制，提供 terminal 与本机浏览器 GUI。
- 流式多文件扫描、固定长度磁盘索引、Java / Python 多行异常识别。
- 通用文本、JSON、Pino、Serilog、glog、Go panic 及自定义规则。
- 异常指纹聚合，次数、首尾时间、启发式影响、可观察来源范围。
- 时间、关键字、逐行正则、级别和文件筛选；分页详情、上下文、复制与导出。
- Agent JSON stdout、schema 报告、稳定分类 ID、样本字节位置与认证 HTTP API。
- Windows amd64 实测；另外五个平台完成编译。

已知边界与测试限制见 `VALIDATION.md`，包括 clipboard、race、平台实际运行及性能适用范围。
