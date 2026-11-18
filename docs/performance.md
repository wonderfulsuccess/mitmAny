# Performance notes

Historical measurements from 2026-10-21, before the bounded HTML change. These numbers have not been re-measured for the open-source preparation. They are not release acceptance results. The original local output was not tracked; no raw-data download is claimed.

Reproduce on your machine:

```sh
go test ./internal/capture -run '^$' -bench 'Benchmark(Tap|Forwarding)' -benchmem -count=3
```

Record OS, architecture, Go version, payload, concurrency, capture loss and raw results alongside any published claim.

## Historical audit (2026-10-21)

新增 `internal/capture/audit_test.go`，覆盖 HTTP/2 并发、普通及 CONNECT 路径的 WS/WSS 握手和双向消息、客户端拒绝 CA、停滞的 TLS 握手、下载中断、关闭后的 HTTP/2 隧道、逐跳头处理、压缩编码、未解密隧道可见性、采集队列丢块后的连续前缀和会话检查点。`app_test.go` 补充 HAR 二进制正文/压缩状态/trailers 往返测试。

本轮修复：TLS/断流诊断漏记、WebSocket 握手字段丢失与状态漏记、关闭后 H2 连接残留、逐跳头外泄、队列拥塞后预览错误拼接、HAR 二进制请求损坏和压缩字节重导入的问题。详情新增 request/response trailers，正文预览支持有界的 zstd 与多层压缩，文本按声明的 charset 解码。无变化的会话不再每 5 秒重写；检查点写入失败会显示为存储错误。

测量环境：Apple M1 Pro、macOS、回环网络；每个基准重复 3 次，原始结果未纳入仓库。

- 1 MiB HTTP：直连约 0.218–0.221 ms/次，代理约 0.448–0.473 ms/次；代理每次分配约 19–21 KB，修改前约 51–53 KB。
- 64 KiB HTTPS/HTTP2，复用已建立连接：直连约 0.168–0.170 ms/次，代理约 0.291–0.310 ms/次；代理每次分配约 24–25 KB，修改前约 57–59 KB。
- 这些数据未证明稳定的延迟改善；已确认的是分配量下降。HTTPS 数值不含首次建连/证书签发开销，也不代表互联网或手机实测速度。
- 1 MiB 连续极限转发压测出现大量采集副本丢弃，网络转发仍成功。当前磁盘副本写入约 655–728 MB/s，低于该回环场景的转发速率。吞吐数值不能解释为完整抓包吞吐；后续优先优化有界写入合并和详情解压缓存。

边界：没有 TUN/UDP/QUIC/HTTP3 接管；绕过系统代理的流量无法采集。证书固定、双向 TLS 和不信任用户 CA 的应用仍可能无法解密。WebSocket 记录握手和生命周期，尚无逐条消息面板；gRPC trailers 可查看，尚无 Protobuf schema 解析。当时非 HTML 每方向保留 1 MiB 预览，HTML 完整采集（当前已改为 8 MiB 上限）；拥塞时普通正文可能跳过副本或整个请求；压缩损坏、截断、超过解码上限或不支持的编码会保留原字节。Android 代理当前默认直连，不自动继承电脑原有的上游代理链。
