import { useEffect, useRef } from 'react';
import { Check, CircleHelp, Loader2, ShieldCheck, X } from 'lucide-react';
import { tr, diagnostic } from './i18n';
import { certificateLabels, type CertificateHealth } from './setupState';

export interface SetupProps {
  health: CertificateHealth;
  running: boolean;
  recording: boolean;
  locked: boolean;
  busy: boolean;
  error: string;
  onInstall: () => void;
  onStart: () => void;
  onCheck: () => void;
  onManage: () => void;
}
export function CaptureSteps(p: SetupProps) {
  const ready = p.health === 'ready';
  return <ol className="setup-steps">
    <li className={ready ? 'complete' : ''}>
      <span className="step-marker">{ready ? <Check size={16} /> : '1'}</span>
      <div><strong>{tr('Install and trust the CA certificate')}</strong>
        <p>{tr(ready ? 'Certificate ready. HTTPS content can be inspected.' : 'Read HTTPS requests. Your system will ask you to approve installation.')}</p>
        {!ready && <span className="step-status">{tr(certificateLabels[p.health])}</span>}
        {p.health === 'invalid' && <p>{tr('Check your computer’s date and time, then open certificate details. Reinstalling the same expired certificate will not renew it.')}</p>}
        {p.locked && !ready && <p>{tr('Stop desktop and Android capture before installing a certificate.')}</p>}
      </div>
      {!ready && <button className="primary" disabled={p.busy || p.health === 'checking' || (p.locked && p.health !== 'error' && p.health !== 'invalid')}
        onClick={p.health === 'error' ? p.onCheck : p.health === 'invalid' ? p.onManage : p.onInstall}>
        {tr(p.health === 'error' ? 'Retry check' : p.health === 'invalid' ? 'Certificate details' : 'Install certificate')}</button>}
    </li>
    <li className={p.running && p.recording ? 'complete' : ''}>
      <span className="step-marker">{p.running && p.recording ? <Check size={16} /> : '2'}</span>
      <div><strong>{tr('Turn on capture')}</strong><p>{tr(p.running ? p.recording ? 'Visit a webpage or use an app. Requests will appear here.' : 'Recording is paused. Resume to save new requests.' : 'Automatically route this computer’s HTTP/HTTPS traffic through mitmAny.')}</p></div>
      {(!p.running || !p.recording) && <button className="primary" disabled={p.busy || (!p.running && !ready)} onClick={p.onStart}>{tr(p.running ? 'Resume recording' : 'Start capture')}</button>}
    </li>
  </ol>;
}
export default function CaptureSetup(p: SetupProps & {
  troubleshooting: boolean;
  address: string;
  systemProxy: boolean;
  hasRequests: boolean;
  recoveryPending: boolean;
  onClose: () => void;
  onTroubleshoot: () => void;
  onRecover: () => void;
  onAndroid: () => void;
}) {
  const dialog = useRef<HTMLDialogElement>(null);
  useEffect(() => { dialog.current?.showModal(); }, []);
  return <dialog ref={dialog} className="modal setup-modal" aria-labelledby="setup-title"
    onCancel={e => { e.preventDefault(); if (!p.busy) p.onClose(); }}>
    <div className="modal-top"><span>{tr('Capture setup')}</span><button aria-label={tr('Close setup')} disabled={p.busy} onClick={p.onClose}><X size={18} /></button></div>
    <div className="setup-heading"><ShieldCheck size={28} /><h2 id="setup-title">{tr(p.troubleshooting ? 'Find out why requests are missing' : 'Your first request, in two steps')}</h2>
      <p>{tr('Set up this computer once, then start capturing whenever you need it.')}</p></div>
    <CaptureSteps {...p} />
    {p.error && <div className="error-box" role="alert">{diagnostic(p.error)}</div>}
    {p.busy && <p className="setup-working" role="status"><Loader2 size={15} className="spin" />{tr('Working… Approve the system prompt if one appears.')}</p>}
    {p.troubleshooting ? <div className="setup-troubleshooting">
      <h3>{tr('Follow the request')}</h3>
      <p className="setup-path">{tr('Certificate → Capture → Proxy → Request → HTTPS content')}</p>
      <details open={!p.hasRequests}><summary>{tr('No requests appear')}</summary>
        <p>{tr('Open or refresh a webpage after starting capture. An idle app may not send any requests.')}</p>
        <p>{p.running ? p.systemProxy ? tr('System proxy mode is on. Existing HTTP/HTTPS proxies are chained automatically; you do not normally need to turn them off.') : tr('Manual proxy mode: configure your browser or app to use {p0} for HTTP and HTTPS. To use automatic setup, stop capture with the top toolbar, then start it from this guide.', {p0: p.address}) : tr('Start capture first. If it fails, the error above stays visible so you can resolve it and retry.')}</p>
        <p>{tr('If another proxy app or VPN overwrites system proxy settings, stop capture, check that app’s proxy mode, and start capture again. Some apps ignore the system proxy; configure their proxy explicitly.')}</p>
      </details>
      <details><summary>{tr('Requests appear, but HTTPS content is missing')}</summary><p>{tr('Recheck CA trust. Firefox and some apps use their own certificate store. Apps with certificate pinning may reject interception even when the system trusts the CA.')}</p></details>
      <details open={p.recoveryPending}><summary>{tr('Capture fails or the network stops working')}</summary><p>{tr('If the proxy port is in use, stop the other program using that port and retry. To undo mitmAny’s desktop proxy changes, restore the previous network settings below. Android proxy settings are managed separately.')}</p>
        <button className="secondary" disabled={p.busy} onClick={p.onRecover}>{tr('恢复电脑网络')}</button>
      </details>
      <div className="setup-links"><button className="secondary" disabled={p.busy} onClick={p.onCheck}>{tr('Retry check')}</button><button className="secondary" disabled={p.busy} onClick={p.onManage}>{tr('Certificate details')}</button><button className="secondary" disabled={p.busy} onClick={p.onAndroid}>{tr('Android 手机')}</button></div>
    </div> : <button className="setup-help" disabled={p.busy} onClick={p.onTroubleshoot}><CircleHelp size={15} />{tr('Not seeing requests? Troubleshoot')}</button>}
    <div className="setup-footer"><button className="secondary" disabled={p.busy} onClick={p.onClose}>{tr(p.running ? 'Back to requests' : 'Set up later')}</button></div>
  </dialog>;
}
