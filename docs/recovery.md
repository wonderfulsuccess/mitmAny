# 网络恢复与卸载 / Recovery and removal

## 应用可以启动

1. 点击工具栏“恢复电脑网络”，恢复抓包前保存的电脑 HTTP/HTTPS 代理。
2. 手机重连 USB，在 Android 面板关闭手机代理。多台设备逐台恢复。
3. 确认网络正常后退出。恢复失败时保留备份文件，不要先删除应用数据。

正常退出会尝试恢复代理；强制结束到下次启动之间没有后台服务继续转发。暂停记录不会恢复系统代理。电脑恢复按钮不代替手机代理恢复。

## 应用无法启动：macOS

打开系统设置 → 网络 → 当前网络服务 → 详细信息 → 代理。仅当 Web Proxy / Secure Web Proxy 指向 `127.0.0.1:9080` 时，关闭这两个开关以临时恢复直连。对其他受影响网络服务重复检查。

如果原本有企业代理，关闭本地代理只是临时直连，并不等于还原原配置。备份在 `~/Library/Application Support/mitmAny/proxy-backup.json`，记录每个服务的 HTTP/HTTPS 服务器、端口和启用状态。根据备份恢复，或修复应用后让应用恢复。不要修改 PAC/VPN/其他代理。

## 应用无法启动：Windows

打开设置 → 网络和 Internet → 代理，检查手动代理。若地址是 mitmAny 的 `127.0.0.1:9080`，可临时关闭“使用代理服务器”。如之前使用企业代理，按原配置恢复。

备份位于 `%AppData%\mitmAny\proxy-backup.json`，包含注册表值的原始类型和 Base64 数据，不能直接作为 `.reg` 导入。保留文件，优先使用修复后的应用恢复。不要随意清除整个 Internet Settings 注册表项。mitmAny 不修改独立 WinHTTP 代理。

## Android USB 断开后无法联网

优先重新连接相同手机并在 Android 面板关闭代理。若桌面应用无法运行，用 Android SDK 的 ADB 检查（替换 `SERIAL`）：

```sh
adb -s SERIAL shell settings get global http_proxy
adb -s SERIAL reverse --list
```

仅当手机当前全局代理是 mitmAny 的 `127.0.0.1:9081`、且原本没有全局代理时，可临时解除：

```sh
adb -s SERIAL shell settings put global http_proxy :0
adb -s SERIAL reverse --remove tcp:9081
```

若原本有代理，按 `android-proxies.json` 中对应设备的原值恢复，优先使用应用的恢复操作。不要覆盖其他工具的端口。离线/未授权设备需要先恢复 USB 调试授权。

## 完整卸载

1. 恢复电脑和每台 Android 设备的代理，确认网络正常。
2. 在 Settings → CA Certificate 中卸载本机 CA 信任。若应用不可用，在 macOS 钥匙串或 Windows 当前用户“受信任的根证书颁发机构”中核对指纹，移除对应的 **mitmAny Local CA**。不要仅凭名称删除其他证书。
3. 在 Android 的用户凭据/受信任凭据设置中移除安装的 mitmAny CA，并删除 Download 中导出的 CA 公钥文件。不同厂商菜单不同。
4. 移除 Codex 配置中 `mcp_servers.mitmany` 表及其子项，保留其他服务。配置目录遵循 `CODEX_HOME`，否则为 `~/.codex`。删除 `~/.agents/skills/mitmany-network-analysis`；如曾手动安装，也检查 `~/.codex/skills` 中的同名副本。只删除本工具的配置和 skill。
5. 退出应用，删除应用包 / exe，再删除本机 mitmAny 数据目录。**删除 CA 私钥前先移除信任**；删除数据不会自动撤销系统信任。
6. 单独清理导出的 HAR、正文、剪贴板内容和 `.mitmany-*.bak` 集成备份。清空会话不会删除这些副本。

## English quick reference

Restore the desktop proxy with the toolbar recovery action. Reconnect every Android device and disable its proxy before unplugging. If the app cannot start, inspect OS proxy settings and disable only the mitmAny loopback endpoint; this is a temporary direct connection, not restoration of a pre-existing corporate proxy. Preserve backup files for exact restoration.

Before removal: restore all proxies, remove the matching CA trust on desktop and Android, remove only the `mitmany` MCP entry and its bundled skill, quit the app, then delete the app and data. Exports and integration backups require separate cleanup.
