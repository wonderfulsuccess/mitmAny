# 开源准备验证记录

日期：2026-11-18。本记录对应本次本地工作区修改，不等于已发布版本的认证。

## 已完成

- MIT 主项目许可证；第三方声明、依赖许可证收集与随安装包分发。
- 维护者确认现有应用图标拥有公开再分发权限。
- 中英文 README、合成流量截图与演示、开发/架构/隐私/安全/恢复卸载文档。
- 首次抓包指引、工具栏电脑网络恢复入口、AI 数据共享提示。
- HTML 采集与逐层解压 8 MiB 上限；普通正文 1 MiB；全部正文计入 256 MiB 会话预算。
- HTML 队列繁忙时继续转发，停止保存副本；HAR 导入及旧正文文件读取受限；详情显示截断原因。
- MCP / HAR / UI 版本一致性检查、前端测试接入 CI、Issue/PR 模板、Dependabot 配置。
- tag 触发双平台构建与 Release 草稿、可选签名/公证、SHA-256 校验与完整文档打包。

## 本机验证结果

| 检查 | 结果 |
| --- | --- |
| `go test -race ./...` | 通过，包括新增的 HTML 超限、拥塞、解压与 HAR/旧文件读取回归测试 |
| `go vet ./...` | 通过 |
| 前端 `npm test` | 22 项通过（Node 22.20.0），包含语言默认值、切换与翻译占位符检查 |
| 前端 `npm run build` | TypeScript / Vite 构建通过 |
| `node scripts/check-release.mjs` | 通过；发布版本为 1.2.0 |
| `actionlint` | GitHub Actions 工作流静态检查通过 |
| `bash -n` | 本地打包与 macOS 签名脚本语法检查通过 |
| macOS 无凭据签名路径 | 正确生成未进行 Developer ID 签名/公证的状态说明 |
| Gitleaks 可达 Git 历史 | 3 个提交扫描通过；唯一原始命中为公开 WebSocket 示例 nonce，添加精确误报规则后通过 |
| Gitleaks 当前源码 | 在仅含已跟踪及拟加入文件的临时副本中扫描通过，不扫描用户运行数据 |
| UI 浏览器预览 | 桌面、480 × 560 窄窗口无横向溢出；合成开始/恢复、Android 和 Codex 状态切换通过 |
| 本地打包脚本 | macOS Intel、Apple Silicon、Windows x64 三个 ZIP 构建和完整性检查通过 |
| 文档相对链接 / Git whitespace | 通过 |

本地包位于 `build/releases/`，属于未正式发布的验证产物。Windows 包为 macOS 交叉编译；macOS 包没有 Developer ID 签名或公证。浏览器预览使用 mock，不能替代真实网络、证书、手机和原生窗口验收。

## 发布与后续验收

1. 仓库已按维护者要求公开。v1.2.0 使用 GitHub Actions 原生 Windows/macOS 构建，仅在双平台验证通过后公开 Release。分支保护等维护策略可后续配置。
2. 若发布签名包，配置自己的签名凭据，运行并验证 macOS 公证/Windows 签名路径；无凭据时明确发布未签名构建。
3. 按 [平台验收清单](release-checklist.md)完成真实 Windows、macOS 与 Android 设备测试，特别是异常退出、断线与已有代理恢复。
4. 本次发布版本为 v1.2.0，中英文 README 提供固定版本下载链接。历史 1.1.0 本地验证包不会作为本次发布附件。

macOS 的 `SecKeychainCopyDefault` 弃用警告仍存在，本轮未替换证书信任实现。密钥扫描为启发式检查，并非完整安全审计。源码和文档已提交并推送，仓库已公开；正式构建结果以 GitHub Actions 和 Release 页面为准。
