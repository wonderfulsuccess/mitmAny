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

  const title = checking ? "正在检查" : status?.ready ? "配置已就绪" : "尚未就绪";
  const issue = error || status?.error;
  return (
    <section className="codex-connection" aria-label="Codex 连接">
      <div className="setting-row codex-heading">
        <div>
          <strong>Codex</strong>
          <span>一键安装分析 skill，让 Codex 读取流量并分析网络协议。</span>
        </div>
        <button className="secondary" disabled={busy || checking || !isDesktop()} onClick={() => void connect()}>
          {busy ? <LoaderCircle size={15} className="codex-spinner" /> : <Plug size={15} />}
          {busy ? "正在连接…" : status?.ready ? "重新连接" : "连接 Codex"}
        </button>
      </div>
      <div className="codex-state" aria-live="polite" aria-busy={busy || checking}>
        <div className="codex-status-line">
          <span className={status?.ready ? "codex-badge ready" : "codex-badge"}>
            {status?.ready && <CheckCircle2 size={12} />} {title}
          </span>
          <button className="codex-refresh" disabled={busy || checking} onClick={() => void refresh(true)} aria-label="检查 Codex 连接状态" title="检查状态">
            <RefreshCw size={13} />
          </button>
        </div>
        <div className="codex-checks">
          <span>Skill：{status?.skillInstalled ? (status.skillDisabled ? "已安装 · 已禁用" : "已安装") : "待安装"}</span>
          <span>MCP：{status?.mcpRegistered ? "已注册" : "待注册 / 更新"}</span>
          <span>本地服务：{status?.serviceRunning ? "运行中" : "不可用"}</span>
        </div>
        {status?.lastActivity ? <p>最近工具调用：{new Date(status.lastActivity).toLocaleString()}</p> : <p>尚未收到工具调用。</p>}
        {issue && <p className="codex-error" role="alert">{issue}</p>}
        <p>{status?.ready ? "在 Codex 中重新连接 MCP 或新开会话，然后说：“使用 mitmany-network-analysis 分析这个 App 的请求”。" : "连接会安装随应用附带的 skill，并注册当前 mitmAny；无需单独安装 Codex CLI。"}</p>
        {status?.ready && <p>配置就绪不代表 Codex 当前在线。开始分析前，请先开启电脑或 Android 抓包。</p>}
        {status?.configPath && <details>
          <summary>安装位置</summary>
          <p>Skill <code>{status.skillPath}</code></p>
          <p>Codex <code>{status.configPath}</code></p>
          <p>移动应用后请重新连接。替换已有文件前会在原目录保存备份。</p>
        </details>}
      </div>
    </section>
  );
}
