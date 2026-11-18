<h1 align="center">mitmAny</h1>

<p align="center"><a href="README.md">English</a> · <a href="README.zh-CN.md">简体中文</a> · <a href="README.zh-TW.md">繁體中文</a> · <a href="README.ja.md">日本語</a> · <a href="README.ko.md">한국어</a></p>

<p align="center">
  <img src="docs/assets/ai-network-detective.png" alt="AI 网络侦探概念插画" width="820" />
</p>
<p align="center">
  <img src="docs/assets/project-badges.svg" alt="100% free · MCP ready · MIT · macOS + Windows" width="680" />
</p>

**让 AI 看见真实网络请求，把抓包变成接口逆向的起点。**

mitmAny 是一款**为 AI 辅助逆向打造的 HTTP / HTTPS 抓包工具**，支持 macOS、Windows 和 Android USB 抓包。把网页或 App 的真实请求交给 Codex 等兼容 MCP 的 AI 客户端，让 AI 基于证据协助梳理接口、认证流程、分页规则、响应结构与业务协议。

**软件 100% 免费、开源，所有功能全部开放。没有付费版、订阅、试用期限或付费解锁功能。** 自愿支持只用于项目维护，不解锁任何特权。你选择的 AI 客户端或模型服务可能单独收费，这不是 mitmAny 的软件收费。


## 下载安装

**v1.2.1** · [发布说明](https://github.com/wonderfulsuccess/mitmAny/releases/tag/v1.2.1)

| 系统 | 直接下载 |
| --- | --- |
| macOS — Apple Silicon / Intel 通用版 | [下载 macOS ZIP](https://github.com/wonderfulsuccess/mitmAny/releases/download/v1.2.1/mitmAny-1.2.1-macos-universal.zip) |
| Windows — x64 | [下载 Windows ZIP](https://github.com/wonderfulsuccess/mitmAny/releases/download/v1.2.1/mitmAny-1.2.1-windows-amd64.zip) |

[SHA-256 校验文件](https://github.com/wonderfulsuccess/mitmAny/releases/download/v1.2.1/SHA256SUMS.txt)。请先解压；macOS 应用移至 Applications，Windows 需要 WebView2。本次安装包未签名，macOS 未公证，可能出现系统安全提示。[安装与校验说明](docs/installation.md)


## 软件界面

![mitmAny 请求列表与 JSON 详情，展示合成示例数据](docs/assets/workspace.png)

*截图使用合成流量，不包含真实账户或设备信息。[演示流程](docs/demo.md)*

## 从抓包到 AI 辅助逆向

```mermaid
flowchart LR
  A["操作网页或 App"] --> B["捕获真实流量"]
  B --> C["通过 MCP 交给 AI"]
  C --> D["整理接口证据"]
```


你操作网页或 App，mitmAny 记录证据，AI 帮你理解接口背后的逻辑。

| 想搞清楚什么 | 可以让 AI 协助完成什么 |
| --- | --- |
| 没有文档的接口 | 按实际请求整理端点、方法、参数和响应结构，并引用请求 ID。 |
| 登录与会话 | 梳理请求顺序、Header 和 Cookie 的变化，区分已观察事实与推测。 |
| 分页与搜索 | 对比滚动、筛选、搜索操作产生的请求，识别游标与页码参数。 |
| 业务流程 | 标记操作时间，分析对应窗口内的请求链路。 |
| 接口异常 | 对比成功和失败请求、状态码、耗时与捕获正文，寻找差异。 |
| 集成与文档 | 基于流量生成接口说明、字段笔记或客户端代码草稿，供你审核。 |

**MCP 把抓包现场直接接入 AI 工作流。** 不必反复手动复制截图与 JSON：AI 客户端可以查询采集状态、限定目标域名、汇总和搜索请求、读取 Header 与受限正文，并等待新流量。观察书签帮助组织分析，但不会开启抓包或保留已被淘汰的记录。

随包提供 `mitmany-network-analysis` skill，引导 AI 基于实际证据分析。在 **Settings → Codex** 中安装 skill 并注册 MCP，重新连接 MCP 或新开 Codex 会话，然后手动开始抓包，试试：

> 分析 `api.example.test` 的登录和分页协议，我会操作 App。请只观察这个域名，引用请求 ID 支持结论，明确指出缺失或截断的证据，最后整理接口文档和待确认事项。

AI 分析由连接的客户端完成，mitmAny 本身不内置大模型。MCP 读取已捕获流量、管理观察书签，不会重放请求、更改代理或绕过证书固定。用于你拥有或获授权分析的系统。[连接方法与工具说明](docs/manual.zh-CN.md#codex--mcp-网络协议分析)。


## 界面语言

软件首次启动默认使用英文。在 **Settings → Language** 中选择 **简体中文** 即可切换，语言偏好会保留到下次启动。README 提供英文、简体中文、繁体中文、日文、韩文；软件界面只支持英文和简体中文。

## 三步开始

1. **配置 HTTPS**：打开“管理证书”，安装并信任本机生成的 CA。未信任时 HTTPS 仅透传。
2. **选择来源**：点击播放按钮抓取电脑系统代理流量；或连接已开启 USB 调试的 Android，在 Android 面板开启手机代理。
3. **发起请求**：操作网页或应用，选择请求查看 Headers、JSON / 文本、图片与 Timing。结束后停止抓包；断开手机前关闭手机代理。

遇到断网可使用工具栏“恢复电脑网络”；手机在 Android 面板恢复。应用无法启动时参阅[恢复指南](docs/recovery.md)。仅分析自己有权访问和调试的流量。

## 能做什么

- HTTP / HTTPS、HTTP/2、流式响应与 SSE 正文前缀。
- 域名分组、筛选、搜索、收藏、虚拟列表、JSON 折叠与正文搜索。
- Android USB 抓包，无需手机与电脑在同一 Wi-Fi。
- HAR 导入 / 导出、复制 cURL、保存捕获正文。
- 系统代理备份与恢复、本机 CA 管理、深浅主题。
- 只读 MCP 查询与随包附带的协议分析 skill；在 Settings → Codex 中连接。

**AI 分析示例**：先开启抓包，再让 AI“分析 `api.example.test` 的登录和分页协议，我会操作 App；请引用实际请求作为证据”。[连接说明与工具边界](docs/manual.zh-CN.md#codex--mcp-网络协议分析)

## 能力边界

- 不覆盖忽略系统代理的应用、UDP / QUIC / HTTP3；没有 TUN 模式。
- 不绕过证书固定；Android 7+ 应用通常需要自行配置信任用户 CA。
- WebSocket 透传并记录握手，暂不提供逐条消息面板或请求重放。
- 请求与普通响应每方向最多 1 MiB，HTML 响应最多 8 MiB；解压同样受限。最多 3,000 条请求、256 MiB 正文预算。
- 队列繁忙时可能截断或跳过副本，继续转发。记录会淘汰，观察书签不固定证据。

## 隐私与安全

流量和 CA 私钥保存在本机；mitmAny 不主动上传抓包数据。**连接 AI 客户端后，工具返回的流量可能由客户端发送到模型服务。** 常见字段脱敏不能保证任意正文无敏感信息。HAR、cURL 和导出的正文可能包含凭证，分享前请检查。

[隐私与数据生命周期](docs/privacy.md) · [私下报告安全问题](SECURITY.md)

## 开发与贡献

依赖 Go 1.26+、Node.js（见 `.nvmrc`）、Wails CLI 2.15.0 和平台编译环境。

```sh
nvm use
npm --prefix frontend ci
npm --prefix frontend test
npm --prefix frontend run build
go install github.com/wailsapp/wails/v2/cmd/wails@v2.15.0
wails dev
```

[开发与测试](docs/development.md) · [架构](docs/architecture.md) · [路线图](docs/roadmap.md) · [更新记录](CHANGELOG.md) · [发布流程](docs/releasing.md)

## 支持项目

如果 mitmAny 对你有帮助，可以通过一次性、自愿的小费支持项目持续维护：

[US $1](https://buy.stripe.com/aFa7sMaQP5TIgZr7Xt97G02) · [US $10](https://buy.stripe.com/28E28s3oneqe6kNcdJ97G03) · [US $100](https://buy.stripe.com/fZu00k3on0zo8sV2D997G04)

以上均为一次性支付，不会自动续费，也不提供额外付费功能或优先支持。Stripe 支付页的收款商户名称为 **SimiSF**。

价格以美元设置，包含适用税费。Stripe 可能提供本地币种报价，请在付款前核对币种和总额。可用支付方式取决于地区和结账币种；验证时人民币结账提供支付宝和微信支付，美元结账未提供微信支付，最终以支付页显示为准。

提交问题、完善文档和贡献代码，同样是对项目的支持。

## 许可

[MIT](LICENSE)。本地维护的 `go-mitmproxy` 分支保留其原许可证与补丁说明。[第三方声明](THIRD_PARTY_NOTICES.md)
