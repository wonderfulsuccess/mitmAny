# mitmAny

Go + Wails + React 桌面 HTTP / HTTPS 流量分析工具，支持 macOS 与 Windows。

首版采用系统 HTTP/HTTPS 代理。优先转发网络数据，异步保存有上限的流量副本；不提供请求改写规则。

## 启动

依赖：Go 1.26+、Node.js 22.12+、Wails CLI 2.15，以及 Wails 对应平台编译环境（macOS Xcode Command Line Tools；Windows WebView2）。

```sh
go install github.com/wailsapp/wails/v2/cmd/wails@v2.15.0
cd frontend
npm ci
cd ..
wails dev
```

构建本机安装包：`wails build -clean`。macOS 通用包：`wails build -clean -platform darwin/universal`。输出在 `build/bin/`。
也可交叉构建 Windows：`wails build -platform windows/amd64`。仓库的 GitHub Actions 配置会分别在 Windows 和 macOS 上生成 Windows exe 和 macOS 通用 zip。发布前仍需开发者签名、macOS 公证与 Windows 真机验证。

### 一键生成三个 ZIP

在 macOS 上执行（需要上述编译环境，Node.js 22.12+）：

```sh
./scripts/build-release.sh
```

脚本在临时副本中构建，前端仅构建一次，不替换工作目录中的应用。版本号读取 `wails.json`，三个包全部成功并通过 ZIP 完整性检查后，输出至 `build/releases/`：

- `mitmAny-<版本>-windows-amd64.zip`：Windows x64，内含 `mitmAny.exe`。
- `mitmAny-<版本>-macos-amd64.zip`：macOS Intel，内含 `mitmAny.app`。
- `mitmAny-<版本>-macos-arm64.zip`：macOS Apple Silicon，内含 `mitmAny.app`。
- `SHA256SUMS-<版本>.txt`：三个 ZIP 的 SHA-256 校验值。

macOS 压缩使用 `ditto` 保留应用包权限和资源。脚本不执行开发者签名或公证；Windows 目标机器需要 WebView2。若使用 nvm，执行前先运行 `nvm use 22`。

## 首次抓包

1. 点击右上角齿轮进入 **Settings → CA Certificate → 管理证书**，点击安装并信任 CA，完成系统授权。Windows 安装至当前用户根证书存储；macOS 由应用直接调用 Security.framework 安装至当前用户默认钥匙串并请求用户信任授权（不再通过 AppleScript 设置根证书信任）。
2. 点击标题栏的播放按钮（开始抓包），应用会先备份原有 HTTP/HTTPS 系统代理，再启用 `127.0.0.1:9080`。
3. 在浏览器或遵循系统代理的应用中访问网络。选择请求查看 Overview、Headers、Request、Response 和 Timing。
4. 标题栏的停止按钮恢复原代理配置并停止采集新请求。已有连接继续转发；本地代理监听会保持到应用退出，让缓存代理设置的客户端仍可通信。
5. 暂停按钮仅暂停新请求的记录，保持系统代理；已在采集的请求会完成正文预览。正常退出先恢复代理。强制结束进程后，下次启动自动尝试恢复；设置中也有“恢复网络”。进程退出与重启之间无法自动提供转发，首版没有后台守护服务。

不接管系统时，在 Settings 关闭 System proxy 后开始抓包，可手动配置代理。CA 未信任时 HTTPS 使用 CONNECT 透传，无法查看其解密正文。

## 已实现

- 自定义无边框窗口、拖动标题栏、macOS/Windows 窗口控制、深浅主题、可调整详情面板。
- 实时虚拟列表、域名分组、搜索、资源类型筛选、错误筛选、收藏、请求简要瀑布图。
- 请求与响应头、查询参数、JSON / 文本、图片预览、PDF 预览（取决于系统 WebView 支持）、二进制保存。
- HTTP/HTTPS、HTTP/2、流式响应、SSE 正文前缀。WebSocket 在首版仅流式透传，尚无消息面板。
- CA 检查、安装/重新安装同一证书、导出公钥、卸载。重新安装会补齐缺失的公钥文件，不会换根证书。
- 本地当前会话恢复、清空、HAR 导入/导出、复制 URL / cURL、保存已捕获正文。
- `⌘/Ctrl F` 搜索、方向键浏览、Esc 关闭详情、`⌘/Ctrl ,` 设置、`⌘/Ctrl B` 隐藏或展开左侧导航栏（自动记忆状态）。

- JSON 正文使用 11px 等宽字体与紧凑语法高亮，支持对象/数组折叠、HTTP(S) 链接通过系统浏览器打开、大数据虚拟滚动，并保留长整数和原始 Raw 正文。

## 转发与容量策略

普通 HTTP 使用共享连接池与标准上游代理转发，HTTPS 使用底层库的 TLS/HTTP2 连接处理。在请求头 hook 设置 `Flow.Stream=true`，避免库默认的请求/响应完整缓冲。读取正文时只计数并将前缀复制到预分配的 32 KiB 块中。普通正文的队列发送为非阻塞：繁忙时跳过副本，标记 truncated / skipped，原网络流继续。HTML 响应会完整流式写盘，队列繁忙时等待可用块，避免丢失中间内容。

- 非 HTML 正文每个方向最多保存 **1 MiB**。HTML / XHTML 响应无采集、解压大小或显示行数上限，HAR 导入、导出及正文保存保留完整 HTML。Response 支持不区分大小写的正文搜索、高亮、匹配计数和 Enter / Shift+Enter 跳转；搜索词在切换请求、标签及关闭再打开详情后保留，自动应用到当前响应。
- **256 个 32 KiB 块**（8 MiB），512 个排队事件，最多 1,024 个同时采集的流；超过容量时继续转发并跳过采集。
- 最多 **3,000 条请求**，自动淘汰最旧记录；非 HTML 正文文件预算 **256 MiB**，HTML 响应不计入该字节预算，因此实际磁盘占用可超过 256 MiB。
- 正文异步写入权限为 0600 的本地文件；每 5 秒及退出时保存元数据。崩溃可能丢失最后一个检查点之后的记录。
- 前端每 200 ms 拉取增量元数据，虚拟列表仅渲染可见行；详情按需读取，正文不进入请求列表事件。
- 上游 TLS 证书始终使用系统信任验证，不静默关闭校验。

这不能承诺“零开销”：MITM 必须终止 TLS、建立上游连接、重新序列化 HTTP，并复制预览字节。普通采集不会等待磁盘；完整 HTML 采集在磁盘繁忙时可能给转发引入背压。

## 测试

```sh
cd frontend && npm run build
cd ..
go test -race ./...
go vet ./...
go test ./internal/capture -run '^$' -bench 'Benchmark(Tap|Forwarding)' -benchmem
```

集成测试覆盖：首段流式响应、上传与 gzip 字节一致性、HTTPS 与 HTTP/2、上游证书拒绝、请求/响应 trailers 保留、停止抓包不断开在途下载、并发请求、暂停、SSE 实时预览、队列饱和、清空后的在途请求、HAR 导入和会话恢复。

一次本地 Apple M1 Pro 的 1 MiB HTTP 回环基准（并发磁盘采集，非真实互联网与非 HTTPS）：直连约 **0.212 ms/次，4.94 GB/s**，代理约 **0.452 ms/次，2.32 GB/s**。这是测试条件下的样本，不是带宽或时延保证；高负载下可能跳过采集副本。32 KiB 块复制的饱和队列微基准也不能代表完整代理性能。

## 平台与协议边界

系统代理不覆盖忽略代理的应用、原始 TCP/UDP 或 QUIC。首版无 TUN 接管；证书固定、双向 TLS、自定义证书库应用可能失败，需要在应用侧配置。Firefox / Java 等可能需要单独导入 CA。系统代理的绕过列表继续生效。

macOS 保存各已启用网络服务的 HTTP/HTTPS 代理地址、端口和启用状态；不覆盖需要账号认证的现有代理。Windows 保存并恢复当前用户相应注册表值，通知 WinINet 刷新；不修改 WinHTTP 独立代理。已禁用的 macOS 代理恢复其禁用状态，空地址配置可能保留本地端点但不生效。网络服务增删/PAC/VPN 共存仍需要真机验证。

系统模式自动串联原有显式 HTTP/HTTPS 代理；macOS 各服务存在冲突代理时拒绝自动接管，避免改变原路由。手动模式默认直连。首版不自动解析 PAC，也不修改 VPN 或绕过列表；依赖 PAC 的环境需要另外验收。CA、代理切换、睡眠唤醒与安装器需要在目标 Windows/macOS 环境继续验收。

## 代码结构

- `internal/capture`：流式副本采集、有界队列、磁盘缓存、增量索引和 HAR。
- `internal/platform`：macOS / Windows 系统代理备份与恢复。
- `internal/certificates`：本机 CA 与系统证书信任。
- `frontend/src`：React 工作台、虚拟列表与详情。
- `third_party/go-mitmproxy`：MIT 许可的 v1.9.4 本地分支；补丁说明见其 `PATCHES.md`，保留原始许可证。

数据默认位于 macOS `~/Library/Application Support/mitmAny` 或 Windows `%AppData%/mitmAny`。CA 私钥与流量数据不上传。


### macOS 证书授权失败时

如果旧版报告 `The authorization was denied since no user interaction was possible`，请使用 0.1.1 或更新版本。新版由 GUI 应用直接请求 macOS 证书信任授权，旧版的 AppleScript 信任安装路径已移除。如系统仍拒绝交互，可在证书管理中点击“打开钥匙串”，导入本机 CA，双击 `mitmAny Local CA`，展开“信任”，选择“始终信任”，完成系统授权后回到应用点击“重新检查”。该入口只打开 CA 公钥文件。

## Android 手机抓包（ADB）

1. 安装 Android SDK Platform-Tools，连接手机，开启 USB 调试并在手机允许此电脑。应用会查找 PATH、`ANDROID_HOME` / `ANDROID_SDK_ROOT` 和常见 SDK 安装路径中的 adb。
2. 点击标题栏 **Android**，选择手机，打开 **手机代理** 开关。应用自动启动监听、建立 ADB USB 隧道并设置手机代理，直接开始抓包，无需另外开启电脑系统代理。设备列表自动更新。
3. 首次使用 HTTPS 时，点击 **安装 CA 证书**。应用复制公钥证书到手机 `Download/mitmAny-ca.crt` 并打开安全设置。在手机选择「安装证书 → CA 证书」和该文件并确认安装，不会传输私钥。
4. 关闭 **手机代理** 开关会自动恢复手机原有代理、移除隧道并关闭监听。正常退出会先自动恢复手机代理并移除 ADB 隧道，再关闭监听；拔掉 USB 前请先关闭。

代理使用 `127.0.0.1:9081`，手机与电脑无需同一 Wi-Fi，也不向局域网开放端口。开启失败时自动回滚；恢复失败时保留备份和监听，重连手机后关闭开关重试；正常退出时若恢复失败会提示并阻止退出。多台待恢复设备会逐台尝试，一台断线不会跳过其他设备。应用仅在手机代理和隧道均配置成功后显示已开启。

原代理备份保存在应用数据目录的 `android-proxies.json`。设备断线或应用异常退出后，重新连接手机并进入 Android 面板关闭代理开关；USB 重连丢失隧道时也可恢复。断线期间手机可能仍保留回环代理，需要重连恢复。已有 9081 隧道会被拒绝覆盖。ADB 权限不足时，部分厂商需要额外开启「USB 调试（安全设置）」。

Android 系统代理仅覆盖遵循该设置的 HTTP/HTTPS 客户端。Android 7+ 应用通常不会默认信任用户安装的 CA；自己的调试应用应通过 [Network Security Configuration](https://developer.android.com/privacy-and-security/security-config) 配置信任用户 CA。证书固定、自定义信任库、忽略代理的应用和 QUIC/UDP 不会因此自动获得解密支持。

可选实机验证（会短暂设置并恢复指定设备代理）：

```sh
MITMANY_TEST_ANDROID_SERIAL=<adb设备序列号> go test -race -run TestAndroidUSBProxy .
```

## 网络自检（2026-10-21）

新增 `internal/capture/audit_test.go`，覆盖 HTTP/2 并发、普通及 CONNECT 路径的 WS/WSS 握手和双向消息、客户端拒绝 CA、停滞的 TLS 握手、下载中断、关闭后的 HTTP/2 隧道、逐跳头处理、压缩编码、未解密隧道可见性、采集队列丢块后的连续前缀和会话检查点。`app_test.go` 补充 HAR 二进制正文/压缩状态/trailers 往返测试。

本轮修复：TLS/断流诊断漏记、WebSocket 握手字段丢失与状态漏记、关闭后 H2 连接残留、逐跳头外泄、队列拥塞后预览错误拼接、HAR 二进制请求损坏和压缩字节重导入的问题。详情新增 request/response trailers，正文预览支持有界的 zstd 与多层压缩，文本按声明的 charset 解码。无变化的会话不再每 5 秒重写；检查点写入失败会显示为存储错误。

测量环境：Apple M1 Pro、macOS、回环网络；每个基准重复 3 次，原始结果见 `output/audit/network-bench-2026-10-21.txt`。

- 1 MiB HTTP：直连约 0.218–0.221 ms/次，代理约 0.448–0.473 ms/次；代理每次分配约 19–21 KB，修改前约 51–53 KB。
- 64 KiB HTTPS/HTTP2，复用已建立连接：直连约 0.168–0.170 ms/次，代理约 0.291–0.310 ms/次；代理每次分配约 24–25 KB，修改前约 57–59 KB。
- 这些数据未证明稳定的延迟改善；已确认的是分配量下降。HTTPS 数值不含首次建连/证书签发开销，也不代表互联网或手机实测速度。
- 1 MiB 连续极限转发压测出现大量采集副本丢弃，网络转发仍成功。当前磁盘副本写入约 655–728 MB/s，低于该回环场景的转发速率。吞吐数值不能解释为完整抓包吞吐；后续优先优化有界写入合并和详情解压缓存。

边界：没有 TUN/UDP/QUIC/HTTP3 接管；绕过系统代理的流量无法采集。证书固定、双向 TLS 和不信任用户 CA 的应用仍可能无法解密。WebSocket 记录握手和生命周期，尚无逐条消息面板；gRPC trailers 可查看，尚无 Protobuf schema 解析。非 HTML 每方向保留 1 MiB 预览，HTML 完整采集；拥塞时普通正文可能跳过副本或整个请求；压缩损坏、截断、超过解码上限或不支持的编码会保留原字节。Android 代理当前默认直连，不自动继承电脑原有的上游代理链。
