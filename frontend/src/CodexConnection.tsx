import { tr, diagnostic, getLocale } from "./i18n";
import { useCallback, useEffect, useRef, useState } from "react";
import { CheckCircle2, LoaderCircle, Plug, RefreshCw } from "lucide-react";
import { api, errorText, isDesktop } from "./bridge";
import type { CodexStatus } from "./types";

export default function CodexConnection() {
  const [status, setStatus] = useState<CodexStatus | null>(null);
  const [busy, setBusy] = useState(false);
  const [checking, setChecking] = useState(true);
  const [error, setError] = useState("");
  const mounted = useRef(false);
  const inFlight = useRef(false);

  const refresh = useCallback(async (visible = false) => {
    if (inFlight.current) return;
    inFlight.current = true;
    if (visible) setChecking(true);
    try {
      const next = await api.GetCodexStatus();
      if (mounted.current) { setStatus(next); if (visible) setError(""); }
    } catch (e) {
      if (mounted.current) setError(errorText(e));
    } finally {
      inFlight.current = false;
      if (mounted.current) setChecking(false);
    }
  }, []);

  useEffect(() => {
    mounted.current = true;
    void refresh(true);
    const timer = window.setInterval(() => { void refresh(); }, 5000);
    return () => { mounted.current = false; window.clearInterval(timer); };
  }, [refresh]);

  async function connect() {
    if (inFlight.current) return;
    inFlight.current = true;
    setBusy(true);
    setError("");
    try {
      const next = await api.ConnectCodex();
      if (mounted.current) { setStatus(next); setError(next.error); }
    } catch (e) {
      if (mounted.current) setError(errorText(e));
    } finally {
      inFlight.current = false;
      if (mounted.current) setBusy(false);
    }
  }

  const title = checking ? tr("正在检查") : status?.ready ? tr("配置已就绪") : tr("尚未就绪");
  const issue = error || status?.error;
  return (
    <section className="codex-connection" aria-label={tr("Codex 连接")}>
      <div className="setting-row codex-heading">
        <div>
          <strong>Codex</strong>
          <span>{tr("一键安装分析 skill，让 Codex 读取流量并分析网络协议。")}</span>
        </div>
        <button className="secondary" disabled={busy || checking || !isDesktop()} onClick={() => void connect()}>
          {busy ? <LoaderCircle size={15} className="codex-spinner" /> : <Plug size={15} />}
          {busy ? tr("正在连接…") : status?.ready ? tr("重新连接") : tr("连接 Codex")}
        </button>
      </div>
      <p className="setting-note">{tr("连接后 AI 客户端可读取抓包数据，并可能将其发送至模型服务。仅分析已获授权的流量；常见字段脱敏不能覆盖所有敏感正文。")}</p>
      <div className="codex-state" aria-live="polite" aria-busy={busy || checking}>
        <div className="codex-status-line">
          <span className={status?.ready ? "codex-badge ready" : "codex-badge"}>
            {status?.ready && <CheckCircle2 size={12} />} {title}
          </span>
          <button className="codex-refresh" disabled={busy || checking} onClick={() => void refresh(true)} aria-label={tr("检查 Codex 连接状态")} title={tr("检查状态")}>
            <RefreshCw size={13} />
          </button>
        </div>
        <div className="codex-checks">
          <span>{tr("Skill：")}{status?.skillInstalled ? (status.skillDisabled ? tr("已安装 · 已禁用") : tr("已安装")) : tr("待安装")}</span>
          <span>{tr("MCP：")}{status?.mcpRegistered ? tr("已注册") : tr("待注册 / 更新")}</span>
          <span>{tr("本地服务：")}{status?.serviceRunning ? tr("运行中") : tr("不可用")}</span>
        </div>
        {status?.lastActivity ? <p>{tr("最近工具调用：")}{new Date(status.lastActivity).toLocaleString(getLocale())}</p> : <p>{tr("尚未收到工具调用。")}</p>}
        {issue && <p className="codex-error" role="alert">{diagnostic(issue)}</p>}
        <p>{status?.ready ? tr("在 Codex 中重新连接 MCP 或新开会话，然后说：“使用 mitmany-network-analysis 分析这个 App 的请求”。") : tr("连接会安装随应用附带的 skill，并注册当前 mitmAny；无需单独安装 Codex CLI。")}</p>
        {status?.ready && <p>{tr("配置就绪不代表 Codex 当前在线。开始分析前，请先开启电脑或 Android 抓包。")}</p>}
        {status?.configPath && <details>
          <summary>{tr("安装位置")}</summary>
          <p>Skill <code>{status.skillPath}</code></p>
          <p>Codex <code>{status.configPath}</code></p>
          <p>{tr("移动应用后请重新连接。替换已有文件前会在原目录保存备份。")}</p>
        </details>}
      </div>
    </section>
  );
}
