import { tr, diagnostic } from "./i18n.ts";
import type { AndroidDevice, AndroidState, AppState, Batch, Certificate, CodexStatus, Detail } from "./types";
type Native = {
  SetLanguage(language: string): Promise<void>;
  GetCodexStatus(): Promise<CodexStatus>;
  ConnectCodex(): Promise<CodexStatus>;
  GetAndroidState(): Promise<AndroidState>;
  AndroidDevices(): Promise<AndroidDevice[]>;
  SetAndroidEnabled(serial: string, enabled: boolean): Promise<AndroidState>;
  SetAndroidListening(enabled: boolean): Promise<AndroidState>;
  SetAndroidProxy(serial: string): Promise<AndroidState>;
  RestoreAndroidProxy(serial: string): Promise<AndroidState>;
  InstallAndroidCertificate(serial: string): Promise<string>;
  GetState(): Promise<AppState>;
  StartCapture(system: boolean): Promise<AppState>;
  StopCapture(): Promise<AppState>;
  RecoverNetwork(): Promise<AppState>;
  SetRecording(v: boolean): Promise<AppState>;
  GetFlows(cursor: number): Promise<Batch>;
  GetFlow(id: string): Promise<Detail>;
  ClearFlows(): Promise<void>;
  GetCertificateStatus(): Promise<Certificate>;
  InstallCertificate(): Promise<Certificate>;
  UninstallCertificate(): Promise<Certificate>;
  ExportCertificate(): Promise<void>;
  OpenCertificateInKeychain(): Promise<void>;
  ExportHAR(): Promise<void>;
  ImportHAR(): Promise<number>;
  SaveBody(id: string, request: boolean): Promise<void>;
  WindowAction(action: string): Promise<void>;
};
declare global {
  interface Window {
    go?: { main?: { App?: Native } };
  }
}
export const isDesktop = () => !!window.go?.main?.App;
const unavailable = () =>
  Promise.reject(
    new Error(tr("请通过 Wails 桌面应用使用网络与证书功能。当前为界面预览。")),
  );
const previewState: AppState = {
  running: false,
  systemProxy: false,
  recording: true,
  address: "127.0.0.1:9080",
  platform: navigator.platform.includes("Mac") ? "darwin" : "windows",
  recoveryPending: false,
  error: "",
  storageError: "",
  dataDirectory: "Desktop application only",
};
const preview: Native = {
  SetLanguage: async () => {},
  GetCodexStatus: async () => ({ skillInstalled: false, mcpRegistered: false, skillDisabled: false, ready: false, serviceRunning: false, lastActivity: 0, skillPath: "", configPath: "", error: tr("请在 mitmAny 桌面应用中连接 Codex。") }),
  ConnectCodex: unavailable,
  GetAndroidState: async () => ({ enabled: false, serial: "", listening: false, address: "127.0.0.1:9081", pending: [] }),
  AndroidDevices: async () => [],
  SetAndroidEnabled: unavailable,
  SetAndroidListening: unavailable,
  SetAndroidProxy: unavailable,
  RestoreAndroidProxy: unavailable,
  InstallAndroidCertificate: unavailable,
  GetState: async () => previewState,
  GetFlows: async () => ({
    flows: [],
    cursor: 0,
    reset: true,
    total: 0,
    bytes: 0,
    dropped: 0,
  }),
  GetCertificateStatus: async () => ({
    exists: false,
    trusted: false,
    valid: false,
    subject: "mitmAny Local CA",
    fingerprint: "",
    expires: "",
    path: "",
  }),
  StartCapture: unavailable,
  StopCapture: unavailable,
  RecoverNetwork: unavailable,
  SetRecording: unavailable,
  GetFlow: unavailable,
  ClearFlows: async () => {},
  InstallCertificate: unavailable,
  UninstallCertificate: unavailable,
  ExportCertificate: unavailable,
  OpenCertificateInKeychain: unavailable,
  ExportHAR: unavailable,
  ImportHAR: unavailable,
  SaveBody: unavailable,
  WindowAction: async () => {},
};
export const api = new Proxy({} as Native, {
  get:
    (_, key: keyof Native) =>
    (...args: never[]) => {
      const target = window.go?.main?.App ?? preview;
      if (key === "SetLanguage" && typeof target[key] !== "function") return preview.SetLanguage(args[0]);
      return (target[key] as (...args: never[]) => unknown)(...args);
    },
});
export function openExternalURL(value: string) {
  const url = new URL(value);
  if (url.protocol !== "http:" && url.protocol !== "https:") throw new Error(tr("仅支持 HTTP/HTTPS 链接"));
  const runtime = (window as Window & { runtime?: { BrowserOpenURL(url: string): void } }).runtime;
  if (runtime?.BrowserOpenURL) runtime.BrowserOpenURL(url.href);
  else window.open(url.href, "_blank", "noopener,noreferrer");
}
export function size(n: number) {
  if (n < 1024) return `${n} B`;
  if (n < 1048576) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / 1048576).toFixed(1)} MB`;
}
export function resourceType(content: string, url = "") {
  const c = content.toLowerCase();
  if (c.includes("json") || c.includes("graphql")) return "JSON";
  if (c.includes("event-stream")) return "SSE";
  if (c.startsWith("image/")) return "Image";
  if (c.includes("html")) return "HTML";
  if (c.includes("javascript")) return "JS";
  if (c.includes("css")) return "CSS";
  if (c.includes("pdf")) return "PDF";
  if (c.includes("font") || /\.(woff2?|ttf)(\?|$)/.test(url)) return "Font";
  if (c.startsWith("video/") || c.startsWith("audio/")) return "Media";
  return c ? "Other" : "Pending";
}
export const errorText = (e: unknown) =>
  diagnostic(e instanceof Error ? e.message : String(e));
