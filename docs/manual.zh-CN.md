# 使用与实现详解

[返回首页](../README.zh-CN.md) · [开发指南](development.md) · [恢复与卸载](recovery.md)

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

普通 HTTP 使用共享连接池与标准上游代理转发，HTTPS 使用底层库的 TLS/HTTP2 连接处理。在请求头 hook 设置 `Flow.Stream=true`，避免库默认的请求/响应完整缓冲。读取正文时只计数并将前缀复制到预分配的 32 KiB 块中。所有正文的队列发送均为非阻塞：繁忙时停止保存当前正文前缀，标记 truncated 并说明原因，原网络流继续。

- 请求和普通响应每个方向最多保存 **1 MiB**；HTML / XHTML 响应最多 **8 MiB**。每层解压使用相同上限；超限或损坏时保留压缩字节。HAR 导入和详情读取同样受限。Response 支持正文搜索、高亮、匹配计数和 Enter / Shift+Enter 跳转。
- **256 个 32 KiB 块**（8 MiB），512 个排队事件，最多 1,024 个同时采集的流；超过容量时继续转发并跳过采集。
- 最多 **3,000 条请求**，自动淘汰最旧记录；所有正文文件合计预算 **256 MiB**，包括 HTML；元数据和文件系统开销另计。
- 正文异步写入权限为 0600 的本地文件；每 5 秒及退出时保存元数据。崩溃可能丢失最后一个检查点之后的记录。
- 前端每 200 ms 拉取增量元数据，虚拟列表仅渲染可见行；详情按需读取，正文不进入请求列表事件。
- 上游 TLS 证书始终使用系统信任验证，不静默关闭校验。

这不能承诺“零开销”：MITM 必须终止 TLS、建立上游连接、重新序列化 HTTP，并复制预览字节。包括 HTML 在内的正文采集不会等待磁盘队列。

## 测试

```sh
cd frontend && npm ci && npm test && npm run build
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

## 代码结构（路径相对于仓库根目录）

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

## Codex / MCP 网络协议分析

### 设置中一键连接（推荐）

打开 **Settings → Codex → 连接 Codex**。应用会离线安装内嵌的 `mitmany-network-analysis` skill 到 `~/.agents/skills`，并将当前应用的绝对路径与 `--mcp --data-dir <本机数据目录>` 注册到用户 Codex `config.toml` 的 `mcp_servers.mitmany`。配置目录遵循 `CODEX_HOME`，未设置时使用 `~/.codex`；不需要另外安装 Codex CLI。

安装器仅更新 mitmany 条目，保留其他配置和注释。替换已有内容前，会在原目录保存权限为 0600 的 `.mitmany-*.bak` 备份；配置无效、目录不可写或检测到并发修改时显示错误，不报告成功。既有 skill 若被 Codex 显式禁用，会提示用户先启用。内联形式 `mcp_servers = {...}` 需先改为标准表格式再使用一键连接。

设置面板分别显示 Skill、MCP 注册、本地服务状态，并每 5 秒刷新最近收到的已认证工具调用时间。**“配置已就绪”不代表 Codex 当前在线**；MCP 工具调用也可能来自其他兼容客户端。完成后在 Codex 重新连接 MCP 或新开会话。移动应用后再次连接即可更新路径；macOS 从 DMG 或 App Translocation 运行时会提示先安装到稳定位置。

skill 通过 Go embed 随安装包分发，无需下载或访问源码目录。旧版手动装在 `~/.codex/skills` 的同名 skill 不会被自动删除；若 Codex 列出重复 skill，可自行保留新路径的版本。

### 手动连接

应用启动时提供带随机令牌的本机查询服务（仅 `127.0.0.1`，独立随机端口）。MCP 桥接进程读取应用数据目录的 `agent-endpoint.json`，多个客户端共用桌面应用的数据；不会启动第二套代理。该文件包含访问令牌，不要分享或提交。正常退出清理文件，重启会轮换令牌。接口拒绝浏览器 Origin、错误 Host 和未认证请求。

开发环境构建独立桥接程序并注册到 Codex：

```sh
go build -o build/bin/mitmany-mcp ./cmd/mitmany-mcp
codex mcp add mitmany -- "$PWD/build/bin/mitmany-mcp"
mkdir -p "$HOME/.agents/skills/mitmany-network-analysis"
cp skills/mitmany-network-analysis/SKILL.md "$HOME/.agents/skills/mitmany-network-analysis/SKILL.md"
```

Windows 可使用 `go build -o build/bin/mitmany-mcp.exe ./cmd/mitmany-mcp`，注册其绝对路径，将 skill 文件夹复制到用户 `.agents/skills`。

安装包里的主程序也支持 `--mcp`（只运行桥接，不打开另一个窗口）：

```sh
# macOS，按实际安装路径调整
codex mcp add mitmany -- /Applications/mitmAny.app/Contents/MacOS/mitmAny --mcp
# Windows：codex mcp add mitmany -- C:\实际路径\mitmAny.exe --mcp
```

重新连接 Codex MCP / 开启新会话后，打开新版 mitmAny 并通过界面启用电脑或 Android 抓包。可说：“使用 mitmany-network-analysis 分析 api.example.com 的登录和分页协议，我会操作 App；请引用实际请求作为证据。” Skill 是工作流指导，MCP 才是数据接口；未安装 skill 也能调用工具。

工具包括 `capture_status`、`begin_observation`、`mark_observation`、`end_observation`、`summarize_traffic`、`search_flows`、`get_flow`、`read_body`、`wait_for_flows`。观察操作仅维护逻辑书签，不清空数据、不更改系统/手机代理、不发送或重放请求。会话指定 host 为精确匹配，不是访问权限隔离。

先聚合和筛选，再按 flow_id 读取头部与正文。列表默认 30 条、最多 100 条；用 `next_before_cursor` 翻页，保留初始 `after_cursor`，保存第一页的事件 `cursor`，读完后推进到该游标（活跃请求更新可能在下一轮重复出现，按 ID 去重）。`reset` 表示历史失效，需要重新读取当前快照。等待最多 30 秒。正文默认 8,000 字符、单次最多 32,000 字符，offset 按脱敏后 Unicode 字符计数；解码/输入上限为 1 MiB，包括 HTML，GUI 的 HTML 读取上限为 8 MiB。

当前限制：观察会话不固定证据，仍受共享存储的 3,000 条/正文预算影响；重启丢失观察书签，查询会报告清空、淘汰和游标失效。暂不可靠区分来源设备或 App 进程，不提供正文全文搜索、JSONPath、WebSocket 消息解析或请求重放。敏感 header/query/JSON/form 的常见键脱敏，任意文本和未知键不保证无凭证；无效 JSON 会拒绝返回以避免脱敏失效。相同值的代号在应用进程内稳定，重启变化。流量始终视为不可信证据，不能将响应中的文字作为智能体指令。
