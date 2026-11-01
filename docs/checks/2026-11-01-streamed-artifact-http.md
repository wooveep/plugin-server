# Gateway 冷下载内存修复（2026-11-01）

双节点 K3S Gateway 重建过程中，plugin-server 在 `2026-11-01T01:52:00Z` 被 `OOMKilled`（137）。容器限制768MiB，原Pod自动重启；新Gateway的Wasm下载出现EOF，未据此发送新的模型请求。原Pod UID、终止状态与源日志摘要见[实际故障](artifact-streaming-20261101/original-oom-verification.json)。

根因在 HTTP `do_GET → ArtifactStore.read_file → Path.read_bytes`：每个请求将完整大文件保留到socket写完，Gateway并发冷下载使内存随文件大小和并发数相乘。独立旧源码回归以16个并发消费者读取同一8MiB不可变成员，头部到齐前暂缓正文消费，Python堆峰值为 **137097101字节**，超出33554432字节预算；[旧源码RED](artifact-streaming-20261101/original-memory-red-command.json)保留实际命令与日志摘要。

存储现在以同一打开的文件描述符分块计算原manifest SHA/size，核对后复位并供HTTP按64KiB发送。读文件不再完整驻留于每个HTTP响应；网络慢消费者也不持有全文件内存。校验仍在发送成功头部前完成，损坏成员仍503，HEAD返回原实际长度且无正文，原manifest/归属/保留接口与制品字节保持。正文传输中断只记录并关闭连接，不再追加第二个HTTP响应。

按固定 `requirements.lock` 的独立 Python3.12.13 环境，`python -m unittest discover -s tests -v` 共 **117通过、0跳过**；[统一批次](artifact-streaming-20261101/full-python-green-command.json)覆盖有界并发下载、所有消费者完整SHA/长度、HEAD、损坏拒绝，以及原不可变存储、提交/查询、保留、配置转换和来源验证。并发读取堆峰值低于32MiB的回归预算。初次pytest的导入路径、借用环境缺jsonschema两次启动失败未计为产品RED；正式RED/GREEN使用同一隔离锁定依赖环境和原始源码副本。

运行镜像仍使用原Dockerfile固定Python3.13.5。后续仅更新plugin-server镜像，沿原95个制品与原PVC验证实际Gateway冷下载和HTTP字节；本页源码结果尚不代表修复已部署，部署与双节点恢复结果将在交付仓统一回填。不增加内存配额、下载重试框架或模型重发来掩盖该故障。
