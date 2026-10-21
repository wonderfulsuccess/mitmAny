import {
  lazy,
  Suspense,
  useCallback,
  useDeferredValue,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import {
  Activity,
  ArrowDown,
  ArrowDownToLine,
  ArrowUpDown,
  ChevronDown,
  ChevronRight,
  CircleHelp,
  FolderOpen,
  Globe2,
  HardDrive,
  Layers3,
  Loader2,
  LockKeyhole,
  Minus,
  Network,
  Pause,
  PanelLeftClose,
  PanelLeftOpen,
  Play,
  Search,
  Settings2,
  ShieldCheck,
  Square,
  Smartphone,
  Star,
  Trash2,
  Wifi,
  X,
  Zap,
} from "lucide-react";
import type { AndroidState, AppState, Certificate, Detail, Flow } from "./types";
import { api, errorText, isDesktop, resourceType, size } from "./bridge";
import RequestMenu from "./RequestMenu";
import Inspector from "./Inspector";
import "./App.css";
const AndroidDialog = lazy(() => import("./AndroidDialog"));
const CertificateDialog = lazy(() => import("./CertificateDialog"));
const filters = [
  "All",
  "JSON",
  "HTML",
  "JS",
  "CSS",
  "Image",
  "PDF",
  "Font",
  "Media",
  "SSE",
  "Other",
];
const initialState: AppState = {
  running: false,
  systemProxy: false,
  recording: true,
  address: "127.0.0.1:9080",
  platform: "darwin",
  recoveryPending: false,
  error: "",
  storageError: "",
  dataDirectory: "",
};
function App() {
  const [state, setState] = useState(initialState);
  const [flows, setFlows] = useState<Flow[]>([]);
  const [stats, setStats] = useState({ total: 0, bytes: 0, dropped: 0 });
  const [certificate, setCertificate] = useState<Certificate | null>(null);
  const [selected, setSelected] = useState<string | null>(null);
  const [detail, setDetail] = useState<Detail | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [query, setQuery] = useState("");
  const deferredQuery = useDeferredValue(query);
  const [type, setType] = useState("All");
  const [host, setHost] = useState("");
  const [view, setView] = useState("all");
  const [requestMenu, setRequestMenu] = useState<{ flow: Flow; x: number; y: number; anchor: HTMLElement } | null>(null);
  const closeRequestMenu = useCallback(() => setRequestMenu(null), []);
  const [stars, setStars] = useState<string[]>(() => {
    try {
      return JSON.parse(localStorage.getItem("mitmAny:stars:v1") || "[]");
    } catch {
      return [];
    }
  });
  const [androidOpen, setAndroidOpen] = useState(false);
  const [android, setAndroid] = useState<AndroidState>({ enabled: false, serial: "", listening: false, address: "127.0.0.1:9081", pending: [] });
  const capturing = state.running || android.enabled;
  const [certOpen, setCertOpen] = useState(false);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [systemProxy, setSystemProxy] = useState(true);
  const [busy, setBusy] = useState(false);
  const [toast, setToast] = useState("");
  const [dark, setDark] = useState(
    () => localStorage.getItem("mitmAny:theme") === "dark",
  );
  const [compact, setCompact] = useState(() => window.matchMedia("(max-width: 1000px)").matches);
  const [mobileSidebarOpen, setMobileSidebarOpen] = useState(false);
  useEffect(() => {
    const media = window.matchMedia("(max-width: 1000px)");
    const change = () => { setCompact(media.matches); setMobileSidebarOpen(false); };
    media.addEventListener("change", change);
    return () => media.removeEventListener("change", change);
  }, []);
  const [sidebarHidden, setSidebarHidden] = useState(
    () => localStorage.getItem("mitmAny:sidebar-hidden") === "true",
  );
  useEffect(() => {
    localStorage.setItem("mitmAny:sidebar-hidden", String(sidebarHidden));
  }, [sidebarHidden]);
  const [inspectorWidth, setInspectorWidth] = useState(420);
  const [trafficSplit, setTrafficSplit] = useState(0.46);
  const analysis = useRef<HTMLDivElement>(null);
  const resizeEnd = useRef<(() => void) | null>(null);
  useEffect(() => () => resizeEnd.current?.(), []);
  const boundSplit = (ratio: number) => {
    const height = analysis.current?.clientHeight || 700;
    return Math.max(220 / height, Math.min(1 - 238 / height, ratio));
  };
  const [ascending, setAscending] = useState(true);
  const [autoScroll, setAutoScroll] = useState(true);
  const scroll = useRef<HTMLDivElement>(null);
  const search = useRef<HTMLInputElement>(null);
  const keyboardSelection = useRef(false);
  useEffect(() => {
    if (!keyboardSelection.current) return;
    const frame = requestAnimationFrame(() => {
      scroll.current?.querySelector<HTMLElement>('.request-row[aria-selected="true"]')?.focus({ preventScroll: true });
      keyboardSelection.current = false;
    });
    return () => cancelAnimationFrame(frame);
  }, [selected]);
  const notify = useCallback((message: string) => setToast(message), []);
  const cursor = useRef(0);
  const index = useRef(new Map<string, Flow>());
  useEffect(() => {
    if (!toast) return;
    const timer = setTimeout(() => setToast(""), 6500);
    return () => clearTimeout(timer);
  }, [toast]);
  useEffect(() => {
    document.documentElement.dataset.theme = dark ? "dark" : "light";
    localStorage.setItem("mitmAny:theme", dark ? "dark" : "light");
  }, [dark]);
  useEffect(() => {
    localStorage.setItem(
      "mitmAny:stars:v1",
      JSON.stringify(stars.slice(-3000)),
    );
  }, [stars]);
  useEffect(() => {
    let active = true;
    let timer: ReturnType<typeof setTimeout>;
    Promise.all([api.GetState(), api.GetCertificateStatus(), api.GetAndroidState()])
      .then(([s, c, mobile]) => {
        if (active) {
          setState(s);
          setCertificate(c);
          setAndroid(mobile);
        }
      })
      .catch((e) => active && notify(errorText(e)));
    let count = 0;
    const poll = async () => {
      try {
        const batch = await api.GetFlows(cursor.current);
        if (!active) return;
        if (batch.reset) index.current.clear();
        for (const id of batch.removed ?? []) index.current.delete(id);
        if (batch.flows.length || batch.reset || batch.removed?.length) {
          for (const flow of batch.flows) index.current.set(flow.id, flow);
          const list = [...index.current.values()]
            .sort((a, b) => a.started - b.started || a.id.localeCompare(b.id))
            .slice(-Math.min(3000, batch.total));
          if (batch.total === 0) {
            list.length = 0;
            index.current.clear();
          } else index.current = new Map(list.map((f) => [f.id, f]));
          setFlows(list);
        }
        cursor.current = batch.cursor;
        setStats((old) =>
          old.total === batch.total &&
          old.bytes === batch.bytes &&
          old.dropped === batch.dropped
            ? old
            : {
                total: batch.total,
                bytes: batch.bytes,
                dropped: batch.dropped,
              },
        );
        if (++count % 5 === 0) {
          const [s, mobile] = await Promise.all([api.GetState(), api.GetAndroidState()]);
          if (active) setAndroid(mobile);
          if (active)
            setState((old) =>
              JSON.stringify(old) === JSON.stringify(s) ? old : s,
            );
        }
      } catch (e) {
        if (active) notify(errorText(e));
      }
      if (active) timer = setTimeout(poll, 200);
    };
    void poll();
    return () => {
      active = false;
      clearTimeout(timer);
    };
  }, [notify]);
  const selectedFlow = flows.find((f) => f.id === selected);
  const selectedRevision = selectedFlow?.revision;
  useEffect(() => {
    if (!selected) {
      setDetail(null);
      return;
    }
    let active = true;
    setDetailLoading(true);
    api
      .GetFlow(selected)
      .then((d) => {
        if (active) {
          setDetail(d);
          setDetailLoading(false);
        }
      })
      .catch((e) => {
        if (active) {
          setDetail(null);
          setDetailLoading(false);
          notify(errorText(e));
        }
      });
    return () => {
      active = false;
    };
  }, [selected, selectedRevision, notify]);
  const domains = useMemo(() => {
    const map = new Map<string, number>();
    for (const f of flows) map.set(f.host, (map.get(f.host) || 0) + 1);
    return [...map].sort((a, b) => b[1] - a[1]);
  }, [flows]);
  const visible = useMemo(() => {
    const q = deferredQuery.trim().toLowerCase();
    const starred = new Set(stars);
    const result = flows.filter(
      (f) =>
        (!host || f.host === host) &&
        (view !== "starred" || starred.has(f.id)) &&
        (view !== "errors" || !!f.error || f.status >= 400) &&
        (type === "All" || resourceType(f.contentType, f.url) === type) &&
        (!q ||
          `${f.method} ${f.url} ${f.status} ${f.contentType}`
            .toLowerCase()
            .includes(q)),
    );
    return ascending ? result : result.reverse();
  }, [flows, deferredQuery, host, view, stars, type, ascending]);
  const virtual = useVirtualizer({
    count: visible.length,
    getScrollElement: () => scroll.current,
    estimateSize: () => 38,
    overscan: 15,
  });
  useEffect(() => {
    if (autoScroll && ascending && visible.length)
      virtual.scrollToIndex(visible.length - 1, { align: "end" });
  }, [visible.length, autoScroll, ascending]);
  useEffect(() => {
    const listener = (event: KeyboardEvent) => {
      if (document.querySelector('[role="menu"]')) return;
      if (
        (event.metaKey || event.ctrlKey) &&
        event.key.toLowerCase() === "b" &&
        !document.querySelector("dialog[open]")
      ) {
        event.preventDefault();
        if (compact) setMobileSidebarOpen(open => !open);
        else setSidebarHidden((hidden) => !hidden);
      }
      if ((event.metaKey || event.ctrlKey) && event.key === ",") {
        event.preventDefault();
        setSettingsOpen(true);
      }
      if ((event.metaKey || event.ctrlKey) && event.key === "f") {
        event.preventDefault();
        if (compact) setSelected(null);
        requestAnimationFrame(() => search.current?.focus());
      }
      if (event.key === "Escape") {
        if (mobileSidebarOpen) { setMobileSidebarOpen(false); return; }
        setSelected(null);
        search.current?.blur();
      }
      if (
        event.target instanceof HTMLInputElement ||
        event.target instanceof HTMLTextAreaElement ||
        document.querySelector("dialog[open]")
      )
        return;
      if (event.altKey || event.metaKey || event.ctrlKey || mobileSidebarOpen) return;
      if (event.target instanceof HTMLElement && (event.target.isContentEditable || event.target.closest('.inspector, .sidebar, [role="separator"]'))) return;
      if (event.key === "ArrowDown" || event.key === "ArrowUp") {
        event.preventDefault();
        const current = visible.findIndex((f) => f.id === selected);
        const next = Math.max(
          0,
          Math.min(
            visible.length - 1,
            current + (event.key === "ArrowDown" ? 1 : -1),
          ),
        );
        if (visible[next]) {
          keyboardSelection.current = true;
          setAutoScroll(false);
          setSelected(visible[next].id);
          virtual.scrollToIndex(next, { align: "auto" });
        }
      }
    };
    window.addEventListener("keydown", listener);
    return () => window.removeEventListener("keydown", listener);
  }, [visible, selected, compact, mobileSidebarOpen]);
  const toggleCapture = async () => {
    setBusy(true);
    try {
      setState(
        state.running
          ? await api.StopCapture()
          : await api.StartCapture(systemProxy),
      );
    } catch (e) {
      notify(errorText(e));
      if (!state.running && systemProxy && !certificate?.trusted)
        setCertOpen(true);
    } finally {
      setBusy(false);
    }
  };
  const clear = async () => {
    try {
      await api.ClearFlows();
      index.current.clear();
      cursor.current = 0;
      setFlows([]);
      setSelected(null);
    } catch (e) {
      notify(errorText(e));
    }
  };
  const resize = (event: React.PointerEvent<HTMLDivElement>) => {
    event.preventDefault();
    resizeEnd.current?.();
    event.currentTarget.setPointerCapture(event.pointerId);
    const start = event.clientX;
    const width = inspectorWidth;
    const handle = (e: PointerEvent) => {
      if (compact) {
        const box = analysis.current?.getBoundingClientRect();
        if (box) setTrafficSplit(boundSplit((e.clientY - box.top) / box.height));
      } else setInspectorWidth(Math.max(340, Math.min(650, width + start - e.clientX)));
    };
    const end = () => {
      window.removeEventListener("pointermove", handle);
      window.removeEventListener("pointerup", end);
      window.removeEventListener("pointercancel", end);
      resizeEnd.current = null;
    };
    resizeEnd.current = end;
    window.addEventListener("pointermove", handle);
    window.addEventListener("pointerup", end);
    window.addEventListener("pointercancel", end);
  };
  const maxDuration = Math.max(
    100,
    ...visible.slice(-500).map((f) => f.duration),
  );
  const changeView = (v: string) => {
    setView(v);
    setHost("");
  };
  return (
    <div className={`app-shell ${sidebarHidden ? "sidebar-hidden" : ""} ${mobileSidebarOpen ? "mobile-sidebar-open" : ""}`}>
      <header
        className={`titlebar ${state.platform === "windows" ? "windows-titlebar" : ""}`}
      >
        <div className="titlebar-brand">
          <div className="window-controls">
            <button
              className="close"
              aria-label="关闭窗口"
              onClick={() => api.WindowAction("close")}
            >
              <X size={8} />
            </button>
            <button
              className="minimise"
              aria-label="最小化窗口"
              onClick={() => api.WindowAction("minimise")}
            >
              <Minus size={8} />
            </button>
            <button
              className="maximise"
              aria-label="最大化窗口"
              onClick={() => api.WindowAction("maximise")}
            >
              <Square size={7} />
            </button>
          </div>
          <div className="window-title">
            <span className="title-mark">
              <Activity size={13} />
            </span>
            mitmAny <span className="version">0.1</span>
          </div>
        </div>
        <div className="titlebar-workspace">
          <button
            className="titlebar-action sidebar-toggle"
            aria-label={(compact ? !mobileSidebarOpen : sidebarHidden) ? "显示侧边栏" : "隐藏侧边栏"}
            aria-expanded={compact ? mobileSidebarOpen : !sidebarHidden}
            aria-controls="workspace-sidebar"
            title={`${(compact ? !mobileSidebarOpen : sidebarHidden) ? "显示" : "隐藏"}侧边栏 · ${state.platform === "darwin" ? "⌘" : "Ctrl"} B`}
            onClick={() => compact ? setMobileSidebarOpen(open => !open) : setSidebarHidden(hidden => !hidden)}
          >
            {(compact ? !mobileSidebarOpen : sidebarHidden) ? (
              <PanelLeftOpen size={17} />
            ) : (
              <PanelLeftClose size={17} />
            )}
          </button>
          <h1 className="titlebar-view" title={host || "All requests"}>
            {host ||
              (view === "starred"
                ? "Starred requests"
                : view === "errors"
                  ? "Failed requests"
                  : "All requests")}
          </h1>
          <span
            className={`live-badge ${capturing ? "live" : ""}`}
            role="status"
          >
            <span />
            {capturing ? (state.recording ? "LIVE" : "PAUSED") : "IDLE"}
          </span>
          <div className="titlebar-drag-space" />
          <div
            className="titlebar-actions"
            role="toolbar"
            aria-label="流量工具"
          >
            <button
              className={`titlebar-action ${state.running ? "connected" : ""}`}
              aria-label="代理设置"
              title={`${state.running ? (state.systemProxy ? "系统代理已连接" : "手动代理运行中") : "代理未启动"} · ${state.address}`}
              onClick={() => setSettingsOpen(true)}
            >
              <Wifi size={16} />
            </button>
            <button
              className={`titlebar-action ${certificate?.trusted ? "trusted" : ""}`}
              aria-label="证书管理"
              title={
                certificate?.trusted ? "HTTPS 证书已信任" : "设置 HTTPS 证书"
              }
              onClick={() => setCertOpen(true)}
            >
              <ShieldCheck size={16} />
            </button>
            <button className={`titlebar-action android-entry ${android.enabled ? "connected" : ""}`} aria-label="Android 手机代理" title={android.pending.length ? "Android 手机代理 · 有已配置或待恢复设备" : "Android 手机代理与 CA 安装"} onClick={() => setAndroidOpen(true)}><Smartphone size={16} /><span>Android{android.enabled ? " · ON" : android.pending.length ? " · 待恢复" : ""}</span></button>
            <span className="titlebar-divider" />
            <button
              className="titlebar-action"
              title="导入 HAR 会话"
              aria-label="导入 HAR"
              onClick={() =>
                api
                  .ImportHAR()
                  .then((n) => n && notify(`已导入 ${n} 条请求`))
                  .catch((e) => notify(errorText(e)))
              }
            >
              <FolderOpen size={16} />
            </button>
            <button
              className="titlebar-action"
              title="导出已捕获的数据为 HAR"
              aria-label="导出 HAR"
              onClick={() => api.ExportHAR().catch((e) => notify(errorText(e)))}
            >
              <ArrowDownToLine size={16} />
            </button>
            <span className="titlebar-divider" />
            <button
              className={`titlebar-action titlebar-capture ${state.running ? "is-running" : ""}`}
              aria-label={
                busy ? "正在连接代理" : state.running ? "停止抓包" : "开始抓包"
              }
              title={
                busy ? "正在连接代理…" : state.running ? "停止抓包" : "开始抓包"
              }
              aria-pressed={state.running}
              disabled={busy}
              onClick={toggleCapture}
            >
              {busy ? (
                <Loader2 size={15} className="spin" />
              ) : state.running ? (
                <Square size={12} fill="currentColor" />
              ) : (
                <Play size={15} fill="currentColor" />
              )}
            </button>
            <button
              className="titlebar-action"
              title="使用帮助与设置"
              aria-label="使用帮助与设置"
              onClick={() => setSettingsOpen(true)}
            >
              <CircleHelp size={16} />
            </button>
          </div>
        </div>
      </header>
      <div className="workspace">
        {compact && mobileSidebarOpen ? <button className="sidebar-scrim" aria-label="关闭侧边栏" onClick={() => setMobileSidebarOpen(false)} /> : null}
        <aside className="sidebar" id="workspace-sidebar" onClick={e => {
          if (compact && (e.target as HTMLElement).closest("button")) { setMobileSidebarOpen(false); setSelected(null); }
        }}>
          <div className="workspace-picker">
            <span className="workspace-icon">
              <Network size={18} />
            </span>
            <div>
              <strong>My workspace</strong>
              <span>Personal · On this device</span>
            </div>
            <ChevronDown size={13} />
          </div>
          <div className="nav-label">WORKSPACE</div>
          <nav>
            <button
              className={
                view === "all" && !host ? "nav-item active" : "nav-item"
              }
              onClick={() => changeView("all")}
            >
              <Layers3 size={16} />
              All requests<span className="count">{flows.length}</span>
            </button>
            <button
              className={view === "starred" ? "nav-item active" : "nav-item"}
              onClick={() => changeView("starred")}
            >
              <Star size={16} />
              Starred
              <span className="count">
                {flows.filter((f) => stars.includes(f.id)).length || ""}
              </span>
            </button>
            <button
              className={view === "errors" ? "nav-item active" : "nav-item"}
              onClick={() => changeView("errors")}
            >
              <Activity size={16} />
              Errors
              <span className="count">
                {flows.filter((f) => f.error || f.status >= 400).length || ""}
              </span>
            </button>
          </nav>
          <div className="nav-label domains-label">
            DOMAINS<span>{domains.length}</span>
          </div>
          <div className="domain-list">
            {domains.length ? (
              domains.map(([name, count]) => (
                <button
                  title={name}
                  className={host === name ? "domain active" : "domain"}
                  key={name}
                  onClick={() => {
                    setHost(host === name ? "" : name);
                    setView("all");
                  }}
                >
                  <ChevronRight size={11} />
                  <Globe2 size={14} />
                  <span>{name}</span>
                  <small>{count}</small>
                </button>
              ))
            ) : (
              <div className="domain-placeholder">
                <Globe2 size={16} />
                <p>
                  Captured hosts
                  <br />
                  will appear here.
                </p>
              </div>
            )}
          </div>
          <div className="sidebar-bottom">
            <div className="local-card">
              <span>
                <HardDrive size={15} />
                Private by default
              </span>
              <p>Your traffic stays on this device.</p>
            </div>
            <button className="nav-item" onClick={() => setCertOpen(true)}>
              <ShieldCheck size={16} />
              CA certificate
              <span
                className={`tiny-status ${certificate?.trusted ? "ok" : ""}`}
              />
            </button>
            <button className="nav-item" onClick={() => setSettingsOpen(true)}>
              <Settings2 size={16} />
              Settings<span className="shortcut">⌘ ,</span>
            </button>
            <div className="sidebar-brand">
              <Activity size={15} />
              <strong>mitmAny</strong>
              <span>BUILT FOR THE DETAILS</span>
            </div>
          </div>
        </aside>
        <main className="main-workspace">
          {state.error || state.recoveryPending || state.storageError ? (
            <div className="network-alert" role="alert">
              <span>
                {state.error ||
                  state.storageError ||
                  "发现未恢复的系统代理配置"}
              </span>
              <button
                onClick={() =>
                  api
                    .RecoverNetwork()
                    .then(setState)
                    .catch((e) => notify(errorText(e)))
                }
              >
                恢复网络
              </button>
            </div>
          ) : null}
          <div
            ref={analysis}
            className={`analysis-workspace ${selected ? "has-selection" : ""}`}
            style={
              {
                "--inspector-width": `${inspectorWidth}px`,
                "--traffic-split": `${trafficSplit * 100}%`,
              } as React.CSSProperties
            }
          >
            <section className="traffic-panel">
              <div className="filter-bar">
                <label className="search-input">
                  <Search size={15} />
                  <input
                    ref={search}
                    value={query}
                    onChange={(e) => setQuery(e.target.value)}
                    placeholder="Filter by URL, method, status…"
                    aria-label="搜索请求"
                  />
                  <kbd>{state.platform === "darwin" ? "⌘" : "Ctrl"} F</kbd>
                  {query ? (
                    <button aria-label="清空搜索" onClick={() => setQuery("")}>
                      <X size={12} />
                    </button>
                  ) : null}
                </label>
                <button
                  className="icon-button"
                  aria-label={
                    state.recording ? "暂停记录新请求" : "继续记录请求"
                  }
                  title={
                    state.recording ? "暂停记录（保持代理转发）" : "继续记录"
                  }
                  onClick={() =>
                    api
                      .SetRecording(!state.recording)
                      .then(setState)
                      .catch((e) => notify(errorText(e)))
                  }
                >
                  {state.recording ? <Pause size={15} /> : <Play size={15} />}
                </button>
                <button
                  className="icon-button"
                  aria-label="清空会话"
                  title="清空本地会话"
                  onClick={clear}
                >
                  <Trash2 size={15} />
                </button>
              </div>
              <div className="resource-filters">
                {filters.map((f) => (
                  <button
                    className={type === f ? "active" : ""}
                    key={f}
                    onClick={() => setType(f)}
                  >
                    {f}
                  </button>
                ))}
                <span />
                <button
                  className={
                    autoScroll ? "auto-scroll active-scroll" : "auto-scroll"
                  }
                  title="自动滚动到最新请求"
                  aria-label="自动滚动"
                  aria-pressed={autoScroll}
                  onClick={() => setAutoScroll(!autoScroll)}
                >
                  <ArrowDown size={12} />
                </button>
              </div>
              <div
                className="request-table"
                role="table"
                aria-label="捕获的网络请求"
                aria-rowcount={visible.length + 1}
              >
                <div className="table-head request-grid" role="row">
                  <span role="columnheader">Status</span>
                  <span role="columnheader">Method</span>
                  <span role="columnheader">Name / Host</span>
                  <span role="columnheader">Type</span>
                  <span role="columnheader">Size</span>
                  <button
                    role="columnheader"
                    onClick={() => setAscending(!ascending)}
                  >
                    Time
                    <ArrowUpDown size={10} />
                  </button>
                  <span role="columnheader">Waterfall</span>
                </div>
                <div className="table-scroll" ref={scroll}>
                  {visible.length ? (
                    <div
                      style={{
                        height: virtual.getTotalSize(),
                        position: "relative",
                      }}
                    >
                      {virtual.getVirtualItems().map((row) => {
                        const f = visible[row.index];
                        const isError = !!f.error || f.status >= 400;
                        return (
                          <div
                            role="row"
                            tabIndex={selected === f.id || (!selected && row.index === 0) ? 0 : -1}
                            aria-selected={selected === f.id}
                            className={`request-row request-grid ${selected === f.id ? "selected" : ""}`}
                            key={f.id}
                            style={{ transform: `translateY(${row.start}px)` }}
                            onClick={() => { setAutoScroll(false); setSelected(f.id); }}
                            onContextMenu={(e) => {
                              e.preventDefault();
                              e.currentTarget.focus();
                              if (!compact) setSelected(f.id);
                              setRequestMenu({ flow: f, x: e.clientX, y: e.clientY, anchor: e.currentTarget });
                            }}
                            onKeyDown={(e) => {
                              if (e.key === "Enter") setSelected(f.id);
                              if (e.key === "ContextMenu" || (e.shiftKey && e.key === "F10")) {
                                e.preventDefault();
                                const box = e.currentTarget.getBoundingClientRect();
                                if (!compact) setSelected(f.id);
                                setRequestMenu({ flow: f, x: box.left + 110, y: box.bottom, anchor: e.currentTarget });
                              }
                            }}
                          >
                            <span role="cell">
                              <span
                                className={`status-code ${isError ? "error" : f.status >= 300 ? "redirect" : ""}`}
                              >
                                {f.error ? "ERR" : f.status || "•••"}
                              </span>
                            </span>
                            <span
                              role="cell"
                              className={`method ${f.method.toLowerCase()}`}
                            >
                              {f.method}
                            </span>
                            <span
                              role="cell"
                              className="request-name"
                              title={f.url}
                            >
                              <span>{f.path.split("?")[0]}</span>
                              <small>{f.host}</small>
                            </span>
                            <span
                              role="cell"
                              className={`resource-type ${resourceType(f.contentType, f.url).toLowerCase()}`}
                            >
                              {resourceType(f.contentType, f.url)}
                            </span>
                            <span role="cell" className="number">
                              {size(f.responseSize)}
                            </span>
                            <span role="cell" className="number">
                              {f.duration < 1000
                                ? `${Math.round(f.duration)} ms`
                                : `${(f.duration / 1000).toFixed(2)} s`}
                            </span>
                            <span role="cell" className="waterfall">
                              <i
                                className={isError ? "failed" : ""}
                                style={{
                                  width: `${Math.max(5, Math.min(100, (f.duration / maxDuration) * 100))}%`,
                                }}
                              />
                              {stars.includes(f.id) ? <span className="row-star starred" title="已收藏"><Star size={11} fill="currentColor" /></span> : null}
                            </span>
                          </div>
                        );
                      })}
                    </div>
                  ) : (
                    <div className="empty-state">
                      <div className="empty-illustration">
                        <div className="orbit one" />
                        <div className="orbit two" />
                        <div className="empty-network">
                          <Network size={32} strokeWidth={1.4} />
                        </div>
                        <span className="orbit-dot a" />
                        <span className="orbit-dot b" />
                        <span className="orbit-dot c" />
                      </div>
                      <span className="eyebrow">YOUR NETWORK, IN FOCUS</span>
                      <h2>
                        {flows.length
                          ? "Nothing matches. Yet."
                          : capturing
                            ? "Listening to your network."
                            : "Every request tells a story."}
                      </h2>
                      <p>
                        {flows.length
                          ? "调整筛选条件，找到你关心的请求。"
                          : capturing
                            ? "在浏览器或应用中发起请求，流量将在这里实时出现。"
                            : "Capture, inspect, and understand your traffic.\n从一个请求开始，看清网络的每一个细节。"}
                      </p>
                      {!flows.length && !capturing ? (
                        <button
                          className="primary"
                          disabled={busy}
                          onClick={toggleCapture}
                        >
                          <Play size={13} fill="currentColor" />
                          Start your first capture
                        </button>
                      ) : null}
                      <div className="empty-capabilities">
                        <span>
                          <LockKeyhole size={12} />
                          HTTPS
                        </span>
                        <span>
                          <Zap size={12} />
                          Stream first
                        </span>
                        <span>
                          <HardDrive size={12} />
                          Local only
                        </span>
                      </div>
                      {!isDesktop() ? (
                        <small className="preview-note">
                          Browser preview · 网络功能请在桌面应用中使用
                        </small>
                      ) : null}
                    </div>
                  )}
                </div>
              </div>
              <div className="table-footer">
                <span>
                  {visible.length} of {stats.total} requests
                </span>
                <span>{size(stats.bytes)} captured</span>
                <span className="local-indicator">
                  <span className="dot" />
                  {capturing ? "Capturing locally" : "Local session"}
                </span>
              </div>
            </section>
            <div
              role="separator"
              aria-label={compact ? "调整列表和详情的高度" : "调整详情面板宽度"}
              aria-orientation={compact ? "horizontal" : "vertical"}
              aria-valuenow={compact ? Math.round(trafficSplit * 100) : inspectorWidth}
              aria-valuemin={compact ? 0 : 340}
              aria-valuemax={compact ? 100 : 650}
              tabIndex={0}
              className="resize-handle"
              onPointerDown={resize}
              onKeyDown={(e) => {
                if (compact) {
                  if (e.key === "ArrowUp" || e.key === "ArrowDown") {
                    e.preventDefault(); e.stopPropagation();
                    setTrafficSplit(value => boundSplit(value + (e.key === "ArrowDown" ? 0.03 : -0.03)));
                  }
                  return;
                }
                if (e.key === "ArrowLeft")
                  setInspectorWidth((v) => Math.min(650, v + 20));
                if (e.key === "ArrowRight")
                  setInspectorWidth((v) => Math.max(340, v - 20));
              }}
            />
            {requestMenu ? <RequestMenu
              key={`${requestMenu.flow.id}-${requestMenu.x}-${requestMenu.y}`}
              flow={requestMenu.flow} x={requestMenu.x} y={requestMenu.y} anchor={requestMenu.anchor}
              starred={stars.includes(requestMenu.flow.id)}
              onClose={closeRequestMenu}
              onStar={() => setStars(old => old.includes(requestMenu.flow.id) ? old.filter(id => id !== requestMenu.flow.id) : [...old, requestMenu.flow.id])}
              onFilter={() => { setHost(requestMenu.flow.host); setView("all"); if (compact) setSelected(null); }}
              notify={notify}
            /> : null}
            <Inspector
              compact={compact}
              detail={selectedFlow && detail?.id === selected ? detail : null}
              loading={detailLoading}
              onClose={() => setSelected(null)}
              notify={notify}
            />
          </div>
          <footer className="app-status">
            <span>
              <span className={`dot ${capturing ? "green" : ""}`} />
              {capturing ? (android.enabled ? "Android capture running" : "Capture running") : "Capture stopped"}
            </span>
            <span>
              <Zap size={11} />
              Stream-first capture
            </span>
            <span>
              {stats.dropped
                ? `${stats.dropped} capture events skipped`
                : "No capture events dropped"}
            </span>
            <span className="status-right">
              HTTP / HTTPS<span>·</span>1 MiB body preview<span>·</span>v0.1.2
            </span>
          </footer>
        </main>
      </div>
      {androidOpen ? <Suspense fallback={null}><AndroidDialog onClose={() => setAndroidOpen(false)} onUpdate={setAndroid} /></Suspense> : null}
      {certOpen ? (
        <Suspense fallback={null}>
          <CertificateDialog
            onClose={() => setCertOpen(false)}
            onUpdate={setCertificate}
            running={state.running || android.listening}
          />
        </Suspense>
      ) : null}
      {settingsOpen ? (
        <Settings
          state={state}
          dark={dark}
          setDark={setDark}
          system={systemProxy}
          setSystem={setSystemProxy}
          onClose={() => setSettingsOpen(false)}
          notify={notify}
        />
      ) : null}
      {toast ? (
        <div className="toast" role="status">
          <CircleHelp size={16} />
          <span>{toast}</span>
          <button aria-label="关闭提示" onClick={() => setToast("")}>
            <X size={14} />
          </button>
        </div>
      ) : null}
    </div>
  );
}
function Settings({
  state,
  dark,
  setDark,
  system,
  setSystem,
  onClose,
  notify,
}: {
  state: AppState;
  dark: boolean;
  setDark: (v: boolean) => void;
  system: boolean;
  setSystem: (v: boolean) => void;
  onClose: () => void;
  notify: (s: string) => void;
}) {
  const ref = useRef<HTMLDialogElement>(null);
  useEffect(() => {
    ref.current?.showModal();
  }, []);
  return (
    <dialog
      ref={ref}
      className="modal settings-modal"
      onCancel={(e) => {
        e.preventDefault();
        onClose();
      }}
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div className="modal-top">
        <span>WORKSPACE SETTINGS</span>
        <button aria-label="关闭设置" onClick={onClose}>
          <X size={18} />
        </button>
      </div>
      <h2>Make yourself at home.</h2>
      <p className="settings-description">网络接管方式、界面与本地数据。</p>
      <label className="setting-row">
        <div>
          <strong>System proxy</strong>
          <span>开启抓包时接管遵循系统代理的应用</span>
        </div>
        <input
          type="checkbox"
          role="switch"
          checked={system}
          disabled={state.running}
          onChange={(e) => setSystem(e.target.checked)}
        />
      </label>
      <div className="setting-note">
        关闭后使用手动代理：<code>{state.address}</code>。忽略系统代理的应用、非
        HTTP 协议及证书固定暂不支持。系统模式会自动串联原有 HTTP/HTTPS 代理。
      </div>
      <label className="setting-row">
        <div>
          <strong>Dark appearance</strong>
          <span>为深夜调试准备的深色界面</span>
        </div>
        <input
          type="checkbox"
          role="switch"
          checked={dark}
          onChange={(e) => setDark(e.target.checked)}
        />
      </label>
      <div className="setting-row">
        <div>
          <strong>Capture limits</strong>
          <span>1 MiB / body · 3,000 requests · 256 MiB on disk</span>
        </div>
        <HardDrive size={17} />
      </div>
      <div className="setting-note">
        优先转发网络。容量不足时跳过抓包副本；旧请求自动淘汰。HAR
        和正文导出均受预览上限约束。SSE 持续采集正文前缀，WebSocket 首版仅透传。
      </div>
      <div className="setting-row">
        <div>
          <strong>Network recovery</strong>
          <span>恢复启动抓包前保存的系统代理配置</span>
        </div>
        <button
          className="secondary"
          onClick={() =>
            api
              .RecoverNetwork()
              .then(() => notify("系统代理配置已恢复"))
              .catch((e) => notify(errorText(e)))
          }
        >
          恢复网络
        </button>
      </div>
      <div className="settings-storage">
        <span>LOCAL DATA DIRECTORY</span>
        <code>{state.dataDirectory}</code>
      </div>
    </dialog>
  );
}
export default App;
