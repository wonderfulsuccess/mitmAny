import { tr, diagnostic } from "./i18n";
import { useEffect, useRef, useState } from "react";
import { ArrowDownToLine, Loader2, Smartphone, X } from "lucide-react";
import { api, errorText } from "./bridge";
import type { AndroidDevice, AndroidState } from "./types";

export default function AndroidDialog({ onClose, onUpdate }: {
  onClose: () => void;
  onUpdate: (s: AndroidState) => void;
}) {
  const dialog = useRef<HTMLDialogElement>(null);
  const mounted = useRef(false);
  const operating = useRef(false);
  const revision = useRef(0);
  const onUpdateRef = useRef(onUpdate);
  onUpdateRef.current = onUpdate;
  const [state, setState] = useState<AndroidState>({ enabled: false, serial: "", listening: false, address: "127.0.0.1:9081", pending: [] });
  const [devices, setDevices] = useState<AndroidDevice[]>([]);
  const [serial, setSerial] = useState("");
  const [busy, setBusy] = useState<"loading" | "enable" | "disable" | "certificate" | "">("loading");
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");
  const update = (s: AndroidState) => { setState(s); onUpdateRef.current(s); };

  useEffect(() => {
    dialog.current?.showModal();
    mounted.current = true;
    let active = true;
    let initial = true;
    let timer: ReturnType<typeof setTimeout>;
    const refresh = async () => {
      if (!operating.current) {
        const current = revision.current;
        const [status, found] = await Promise.allSettled([api.GetAndroidState(), api.AndroidDevices()]);
        if (!active) return;
        if (current === revision.current) {
          const s = status.status === "fulfilled" ? status.value : null;
          if (s) update(s);
          if (found.status === "fulfilled") {
            setDevices(found.value);
            setSerial(old => s?.serial || (old && (found.value.some(d => d.serial === old) || s?.pending.includes(old)) ? old : s?.pending[0] || found.value.find(d => d.state === "device")?.serial || ""));
          }
          if (initial) {
            const failure = [status, found].find(result => result.status === "rejected");
            if (failure?.status === "rejected") setError(errorText(failure.reason));
            setBusy("");
            initial = false;
          }
        }
      }
      if (active) timer = setTimeout(refresh, 3000);
    };
    void refresh();
    return () => { active = false; mounted.current = false; clearTimeout(timer); };
  }, []);

  const action = async (name: "enable" | "disable" | "certificate", fn: () => Promise<void>) => {
    if (operating.current) return;
    operating.current = true;
    revision.current++;
    setBusy(name); setError(""); setMessage("");
    try { await fn(); } catch (e) { if (mounted.current) setError(errorText(e)); }
    finally {
      try { const s = await api.GetAndroidState(); if (mounted.current) update(s); } catch { /* Preserve the operation error. */ }
      operating.current = false;
      if (mounted.current) setBusy("");
    }
  };
  const ready = devices.some(d => d.serial === serial && d.state === "device");
  const needsRecovery = !state.enabled && state.pending.length > 0;
  const checked = state.enabled || needsRecovery;
  const toggling = busy === "enable" || busy === "disable";
  const missing = serial && !devices.some(d => d.serial === serial);
  const status = busy === "loading" ? tr("正在检查手机连接…") : busy === "enable" ? tr("正在连接手机并开启代理…") : busy === "disable" ? tr("正在恢复手机网络…") : needsRecovery ? tr("请连接原手机，关闭开关以恢复网络") : state.enabled ? tr("已开启，手机流量将显示在请求列表") : tr("开启后自动配置手机并开始抓包");

  return <dialog ref={dialog} className="modal android-modal" aria-labelledby="android-title"
    onCancel={e => { e.preventDefault(); if (!busy) onClose(); }}
    onClick={e => { if (e.target === e.currentTarget && !busy) onClose(); }}>
    <div className="modal-top"><span>ANDROID · ADB</span><button aria-label={tr("关闭手机代理面板")} disabled={!!busy} onClick={onClose}><X size={18} /></button></div>
    <div className="android-heading"><Smartphone size={28} /><div><h2 id="android-title">{tr("Android 手机代理")}</h2><p>{tr("连接手机，一键开始抓包。")}</p></div></div>
    <label className="android-label" htmlFor="android-device">{tr("手机")}</label>
    <div className="android-devices"><select id="android-device" value={serial} disabled={!!busy || checked} onChange={e => setSerial(e.target.value)}>
      <option value="">{devices.length ? tr("选择手机") : tr("连接手机并允许 USB 调试")}</option>
      {missing ? <option value={serial}>{serial}  {tr("· 未连接")}</option> : null}
      {devices.map(d => <option key={d.serial} value={d.serial}>{d.model || d.serial}{devices.length > 1 && d.model ? ` · ${d.serial}` : ""} · {d.state === "device" ? tr("已连接") : d.state === "unauthorized" ? tr("等待 USB 调试授权") : tr("未连接")}</option>)}
    </select></div>
    <div className={`android-listening ${state.enabled ? "is-enabled" : ""}`}>
      <div><strong>{tr("手机代理")}</strong><span id="android-proxy-status" role="status">{status}</span></div>
      <button type="button" role="switch" aria-checked={checked} aria-label={tr("手机代理")} aria-describedby="android-proxy-status" className={`android-switch ${checked ? "on" : ""}`} disabled={!!busy || (!checked && !ready)}
        onClick={() => action(checked ? "disable" : "enable", async () => { update(await api.SetAndroidEnabled(serial, !checked)); })}>
        <span>{toggling ? <Loader2 size={12} className="spin" /> : null}</span>
      </button>
    </div>
    <p className="android-help android-routing-note">{tr("关闭时自动恢复手机原有代理。断开 USB 前请先关闭。")}</p>
    {error ? <div className="error-box" role="alert">{diagnostic(error)}</div> : null}
    <div className="android-ca"><div><strong>{tr("HTTPS 证书")}</strong><p>{tr("首次使用时安装一次，并在手机上确认。")}</p></div>
      <button className="secondary" disabled={!!busy || !ready} onClick={() => action("certificate", async () => { const text = await api.InstallAndroidCertificate(serial); if (mounted.current) setMessage(text); })}>
        {busy === "certificate" ? <Loader2 size={14} className="spin" /> : <ArrowDownToLine size={14} />}{tr("安装 CA 证书")}</button>
    </div>
    {message ? <div className="notice" role="status">{diagnostic(message)}</div> : null}
    <details className="android-details"><summary>{tr("连接与证书帮助")}</summary>
      <p>{tr("开启手机开发者选项中的 USB 调试，并允许此电脑。设备列表会自动更新。部分手机还需开启「USB 调试（安全设置）」；电脑需安装 Android SDK Platform-Tools。")}</p>
      <p>{tr("证书会复制到 Download/mitmAny-ca.crt，并打开手机安全设置。选择「安装证书 → CA 证书」和该文件完成安装。")}</p>
      <p>{tr("仅支持遵循系统代理、信任用户 CA 的应用。部分应用需要调试配置，证书固定和 QUIC 流量可能无法解密。")}</p>
    </details>
  </dialog>;
}
