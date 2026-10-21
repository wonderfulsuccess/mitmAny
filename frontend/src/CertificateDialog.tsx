import { useEffect, useRef, useState } from "react";
import {
  ArrowDownToLine,
  Check,
  CheckCircle2,
  ChevronRight,
  Fingerprint,
  Loader2,
  RefreshCw,
  ShieldCheck,
  ShieldOff,
  X,
} from "lucide-react";
import type { Certificate } from "./types";
import { api, errorText } from "./bridge";
export default function CertificateDialog({
  onClose,
  onUpdate,
  running,
}: {
  onClose: () => void;
  onUpdate: (c: Certificate) => void;
  running: boolean;
}) {
  const [certificate, setCertificate] = useState<Certificate | null>(null);
  const [busy, setBusy] = useState("");
  const [error, setError] = useState("");
  const dialog = useRef<HTMLDialogElement>(null);
  const isMac = navigator.platform.includes("Mac");
  useEffect(() => {
    dialog.current?.showModal();
    let active = true;
    api
      .GetCertificateStatus()
      .then((c) => {
        if (active) {
          setCertificate(c);
          onUpdate(c);
        }
      })
      .catch((e) => {
        if (active) setError(errorText(e));
      });
    return () => {
      active = false;
    };
  }, []);
  const action = async (
    name: string,
    fn: () => Promise<Certificate | void>,
  ) => {
    setBusy(name);
    setError("");
    try {
      const result = await fn();
      if (result) {
        setCertificate(result);
        onUpdate(result);
      }
    } catch (e) {
      setError(errorText(e));
    } finally {
      setBusy("");
    }
  };
  return (
    <dialog
      ref={dialog}
      className="modal certificate-modal"
      onCancel={(e) => {
        e.preventDefault();
        if (!busy) onClose();
      }}
      onClick={(e) => {
        if (e.target === e.currentTarget && !busy) onClose();
      }}
    >
      <div className="modal-top">
        <span>SECURITY & TRUST</span>
        <button aria-label="关闭证书管理" disabled={!!busy} onClick={onClose}>
          <X size={18} />
        </button>
      </div>
      <div className="certificate-hero">
        <div className={`shield-icon ${certificate?.trusted ? "trusted" : ""}`}>
          <ShieldCheck size={30} />
        </div>
        <h2>Unlock HTTPS inspection.</h2>
        <p>
          安装本机专属的 CA 证书，以查看加密请求。
          <br />
          私钥始终保存在你的设备上。
        </p>
      </div>
      <div className={`trust-status ${certificate?.trusted ? "trusted" : ""}`}>
        {certificate?.trusted ? (
          <CheckCircle2 size={18} />
        ) : (
          <ShieldOff size={18} />
        )}
        <div>
          <strong>
            {certificate?.trusted
              ? "CA certificate is trusted"
              : "CA certificate is not trusted"}
          </strong>
          <span>
            {certificate?.trusted
              ? "已通过系统证书信任检查"
              : "系统会显示证书授权提示"}
          </span>
        </div>
        <button
          title="重新检查"
          aria-label="重新检查证书"
          disabled={!!busy}
          onClick={() => action("check", api.GetCertificateStatus)}
        >
          <RefreshCw size={15} className={busy === "check" ? "spin" : ""} />
        </button>
      </div>
      <div className="certificate-info">
        <div>
          <span>Issued to</span>
          <strong>{certificate?.subject || "mitmAny Local CA"}</strong>
        </div>
        <div>
          <span>Expires</span>
          <strong>
            {certificate?.expires
              ? new Date(certificate.expires).toLocaleDateString()
              : "—"}
          </strong>
        </div>
        <div className="fingerprint">
          <span>
            <Fingerprint size={13} /> SHA-256 fingerprint
          </span>
          <code>
            {certificate?.fingerprint || "Available in the desktop application"}
          </code>
        </div>
      </div>
      {running ? (
        <div className="notice">请先停止抓包，再安装或卸载证书。</div>
      ) : null}
      {error ? (
        <div role="alert" className="error-box">
          {error}
        </div>
      ) : null}
      <button
        className="primary install-button"
        disabled={!!busy || running}
        onClick={() => action("install", api.InstallCertificate)}
      >
        {busy === "install" ? (
          <Loader2 size={16} className="spin" />
        ) : certificate?.trusted ? (
          <RefreshCw size={16} />
        ) : (
          <ShieldCheck size={16} />
        )}{" "}
        {busy === "install"
          ? "等待系统授权…"
          : certificate?.trusted
            ? "重新安装 CA 证书"
            : "安装并信任 CA 证书"}
        <ChevronRight size={16} />
      </button>
      {isMac ? (
        <div className="keychain-help">
          <button
            className="secondary"
            disabled={!!busy || running}
            onClick={() => action("keychain", api.OpenCertificateInKeychain)}
          >
            打开钥匙串
          </button>
          <p>
            如系统授权仍不可用：导入本机 CA 后，在钥匙串中双击“mitmAny Local
            CA”，展开“信任”，将“使用此证书时”设为“始终信任”，完成系统授权后点击“重新检查”。
          </p>
        </div>
      ) : null}
      <div className="cert-actions">
        <button
          disabled={!!busy}
          onClick={() => action("export", api.ExportCertificate)}
        >
          <ArrowDownToLine size={14} />
          导出公钥证书
        </button>
        <button
          disabled={!!busy || running || !certificate?.trusted}
          onClick={() => action("uninstall", api.UninstallCertificate)}
        >
          卸载证书
        </button>
      </div>
      <div className="modal-note">
        <Check size={12} /> 仅用于本机调试 · Firefox 等应用可能需要独立导入证书
      </div>
    </dialog>
  );
}
