import { tr, diagnostic, getLocale, setLocale } from "./i18n";
import { useLocale } from "./useLocale";
import {
  lazy,
  Suspense,
  useCallback,
  useDeferredValue,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import {
  Activity,
  RotateCcw,
  TriangleAlert,
  ArrowDown,
  ArrowDownToLine,
  ArrowUpDown,
  ArrowUp,
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
  Pin,
  Search,
  Settings2,
  ShieldCheck,
  Square,
  Smartphone,
  Star,
  Trash2,
  X,
  Zap,
} from "lucide-react";
import CodexConnection from "./CodexConnection";
import type { AndroidState, AppState, Certificate, Detail, Flow } from "./types";
import { api, errorText, isDesktop, resourceType, size } from "./bridge";
import { domainName, groupDomains, searchDomainGroups } from "./domainGroups";
import { anchorOffset, listAnchor, REQUEST_ROW_HEIGHT } from "./listScroll";
import type { ListAnchor } from "./listScroll";
import { newestRequestIndex, sortRequests } from "./requestSort";
import type { RequestSort, RequestSortKey } from "./requestSort";
import RequestAge from "./RequestAge";
import RequestMenu from "./RequestMenu";
import Inspector from "./Inspector";
import "./App.css";
import { version } from "../package.json";
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
const requestColumns: { key: RequestSortKey; label: string; description: string }[] = [
  { key: "status", label: "Status", description: "状态码" },
  { key: "method", label: "Method", description: "请求方法" },
  { key: "name", label: "Name / Host", description: "请求路径与域名" },
  { key: "type", label: "Type", description: "资源类型" },
  { key: "size", label: "Size", description: "响应大小" },
  { key: "age", label: "距今", description: "请求发生时间，逆序为最新在前" },
  { key: "duration", label: "Time", description: "请求耗时" },
  { key: "waterfall", label: "Waterfall", description: "瀑布图总耗时" },
];
function measureDomain(button: HTMLElement) {
  const label = button.querySelector<HTMLElement>(".domain-name");
  const text = label?.querySelector<HTMLElement>(".domain-scroll");
  if (!label || !text) return;
  const overflow = Math.max(0, text.scrollWidth - label.clientWidth);
  label.dataset.overflow = String(overflow > 1);
  label.style.setProperty("--domain-offset", `${-overflow}px`);
  label.style.setProperty("--domain-duration", `${Math.max(3, overflow / 30 + 1)}s`);
}
function App() {
  const locale = useLocale();
  useEffect(() => { void api.SetLanguage(locale).catch(error => console.error("Could not sync native dialog language", error)); }, [locale]);
  const [state, setState] = useState(initialState);
  const [flows, setFlows] = useState<Flow[]>([]);
  const [stats, setStats] = useState({ total: 0, bytes: 0, dropped: 0 });
  const [certificate, setCertificate] = useState<Certificate | null>(null);
  const [selected, setSelected] = useState<string | null>(null);
  const [detail, setDetail] = useState<Detail | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [query, setQuery] = useState("");
  const deferredQuery = useDeferredValue(query);
  const [domainQuery, setDomainQuery] = useState("");
  const [domainGrouped, setDomainGrouped] = useState(() => localStorage.getItem("mitmAny:domain-grouped") !== "false");
  useEffect(() => { localStorage.setItem("mitmAny:domain-grouped", String(domainGrouped)); }, [domainGrouped]);
  const [type, setType] = useState("All");
  const [host, setHost] = useState("");
  const [hostScope, setHostScope] = useState<"host" | "root">("host");
  const [expandedDomains, setExpandedDomains] = useState<Map<string, boolean>>(() => new Map());
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
  const [sort, setSort] = useState<RequestSort>({ key: "age", direction: "desc" });
  const [autoScroll, setAutoScroll] = useState(() => localStorage.getItem("mitmAny:follow-latest") !== "false");
  useEffect(() => { localStorage.setItem("mitmAny:follow-latest", String(autoScroll)); }, [autoScroll]);
  const viewportAnchor = useRef<ListAnchor | null>(null);
  const previousScrollMode = useRef({ scope: "", follow: autoScroll });
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
  const domains = useMemo(() => groupDomains(flows, domainGrouped), [flows, domainGrouped]);
  const filteredDomains = useMemo(() => searchDomainGroups(domains, domainQuery), [domains, domainQuery]);
  const visible = useMemo(() => {
    const q = deferredQuery.trim().toLowerCase();
    const starred = new Set(stars);
    const result = flows.filter(
      (f) =>
        (!host || (hostScope === "root" ? (f.rootDomain || domainName(f)) === host : domainName(f) === host)) &&
        (view !== "starred" || starred.has(f.id)) &&
        (view !== "errors" || !!f.error || f.status >= 400) &&
        (type === "All" || resourceType(f.contentType, f.url) === type) &&
        (!q ||
          `${f.method} ${f.url} ${f.status} ${f.contentType}`
            .toLowerCase()
            .includes(q)),
    );
    return sortRequests(result, sort, resourceType);
  }, [flows, deferredQuery, host, hostScope, view, stars, type, sort]);
  const virtual = useVirtualizer({
    count: visible.length,
    getScrollElement: () => scroll.current,
    estimateSize: () => REQUEST_ROW_HEIGHT,
    getItemKey: index => visible[index].id,
    overscan: 15,
  });
  const scrollScope = JSON.stringify([host, hostScope, view, type, deferredQuery, sort.key, sort.direction]);
  useLayoutEffect(() => {
    const element = scroll.current;
    if (!element) return;
    const previous = previousScrollMode.current;
    const scopeChanged = previous.scope !== scrollScope;
    if (autoScroll && visible.length) {
      virtual.scrollToIndex(newestRequestIndex(visible), { align: sort.key === "age" ? sort.direction === "asc" ? "end" : "start" : "auto" });
    } else if (scopeChanged) {
      virtual.scrollToOffset(0);
    } else if (!previous.follow) {
      const offset = anchorOffset(visible, viewportAnchor.current);
      if (offset !== null && Math.abs(element.scrollTop - offset) > 0.5) virtual.scrollToOffset(offset);
    }
    viewportAnchor.current = listAnchor(visible, element.scrollTop);
    previousScrollMode.current = { scope: scrollScope, follow: autoScroll };
  }, [visible, autoScroll, sort, scrollScope, virtual]);
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
  const maxDuration = visible.reduce((maximum, flow) => Math.max(maximum, flow.duration), 100);
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
              aria-label={tr("关闭窗口")}
              onClick={() => api.WindowAction("close")}
            >
              <X size={8} />
            </button>
            <button
              className="minimise"
              aria-label={tr("最小化窗口")}
              onClick={() => api.WindowAction("minimise")}
            >
              <Minus size={8} />
            </button>
            <button
              className="maximise"
              aria-label={tr("最大化窗口")}
              onClick={() => api.WindowAction("maximise")}
            >
              <Square size={7} />
            </button>
          </div>
          <div className="window-title">
            <span className="title-mark">
              <Activity size={13} />
            </span>
            mitmAny <span className="version">{version}</span>
          </div>
        </div>
        <div className="titlebar-workspace">
          <button
            className="titlebar-action sidebar-toggle"
            aria-label={(compact ? !mobileSidebarOpen : sidebarHidden) ? tr("显示侧边栏") : tr("隐藏侧边栏")}
            aria-expanded={compact ? mobileSidebarOpen : !sidebarHidden}
            aria-controls="workspace-sidebar"
            title={tr("{p0}侧边栏 · {p1} B", {p0: (compact ? !mobileSidebarOpen : sidebarHidden) ? tr("显示") : tr("隐藏"), p1: state.platform === "darwin" ? "⌘" : "Ctrl"})}
            onClick={() => compact ? setMobileSidebarOpen(open => !open) : setSidebarHidden(hidden => !hidden)}
          >
            {(compact ? !mobileSidebarOpen : sidebarHidden) ? (
              <PanelLeftOpen size={17} />
            ) : (
              <PanelLeftClose size={17} />
            )}
          </button>
          <h1 className="titlebar-view" title={host || tr("All requests")}>
            {host ||
              (view === "starred"
                ? "Starred requests"
                : view === "errors"
                  ? tr("Failed requests")
                  : tr("All requests"))}
          </h1>
          <span
            className={`live-badge ${capturing ? "live" : ""} ${stats.dropped ? "capture-warning" : ""}`}
            title={tr("{p0} · 流式采集 · {p1}", {p0: capturing ? (state.recording ? android.enabled ? tr("Android 采集中") : tr("正在采集") : tr("采集记录已暂停")) : tr("采集已停止"), p1: stats.dropped ? tr("{p0} 个采集事件未能保存", {p0: stats.dropped}) : tr("无采集事件丢失")})}
            aria-label={`${capturing ? state.recording ? tr("正在采集") : tr("采集记录已暂停") : tr("采集已停止")}，${stats.dropped ? tr("{p0} 个采集事件未能保存", {p0: stats.dropped}) : tr("无采集事件丢失")}`}
            role="status"
          >
            <span />
            <b className="live-label">{capturing ? (state.recording ? tr("LIVE") : tr("PAUSED")) : tr("IDLE")}</b>
            {stats.dropped ? <TriangleAlert size={11} aria-hidden="true" /> : null}
          </span>
          <div className="titlebar-drag-space" />
          <div
            className="titlebar-actions"
            role="toolbar"
            aria-label={tr("流量工具")}
          >
            <button className={`titlebar-action android-entry ${android.enabled ? "connected" : ""}`} aria-label={tr("Android 手机代理")} title={android.enabled ? tr("Android 手机代理已开启") : android.pending.length ? tr("Android 手机代理 · 有待恢复设备") : tr("Android 手机代理与 CA 安装")} onClick={() => setAndroidOpen(true)}><Smartphone size={16} /></button>
            <button className="titlebar-action" aria-label={tr("恢复电脑网络")} title={tr("恢复抓包前的电脑系统代理")} disabled={busy}
              onClick={() => { setBusy(true); api.RecoverNetwork().then(next => { setState(next); notify(tr("电脑系统代理已恢复；手机代理请在 Android 面板恢复")); }).catch(e => notify(errorText(e))).finally(() => setBusy(false)); }}>
              <RotateCcw size={16} />
            </button>
            <span className="titlebar-divider" />
            <button
              className="titlebar-action"
              title={tr("导入 HAR 会话")}
              aria-label={tr("导入 HAR")}
              onClick={() =>
                api
                  .ImportHAR()
                  .then((n) => n && notify(tr("已导入 {p0} 条请求", {p0: n})))
                  .catch((e) => notify(errorText(e)))
              }
            >
              <FolderOpen size={16} />
            </button>
            <button
              className="titlebar-action"
              title={tr("导出已捕获的数据为 HAR")}
              aria-label={tr("导出 HAR")}
              onClick={() => api.ExportHAR().catch((e) => notify(errorText(e)))}
            >
              <ArrowDownToLine size={16} />
            </button>
            <span className="titlebar-divider" />
            <button
              className={`titlebar-action titlebar-capture ${state.running ? "is-running" : ""}`}
              aria-label={
                busy ? tr("正在连接代理") : state.running ? tr("停止抓包") : tr("开始抓包")
              }
              title={
                busy ? tr("正在连接代理…") : state.running ? tr("停止抓包") : tr("开始抓包")
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
              title={tr("Settings · 设置")}
              aria-label={tr("设置")}
              onClick={() => setSettingsOpen(true)}
            >
              <Settings2 size={16} />
            </button>
          </div>
        </div>
      </header>
      <div className="workspace">
        {compact && mobileSidebarOpen ? <button className="sidebar-scrim" aria-label={tr("关闭侧边栏")} onClick={() => setMobileSidebarOpen(false)} /> : null}
        <aside className="sidebar" id="workspace-sidebar" onClick={e => {
          if (compact && (e.target as HTMLElement).closest("button")) { setMobileSidebarOpen(false); setSelected(null); }
        }}>
          <div className="nav-label">{tr("WORKSPACE")}</div>
          <nav>
            <button
              className={
                view === "all" && !host ? "nav-item active" : "nav-item"
              }
              onClick={() => changeView("all")}
            >
              <Layers3 size={16} />
              {tr("All requests")}<span className="count">{flows.length}</span>
            </button>
            <button
              className={view === "starred" ? "nav-item active" : "nav-item"}
              onClick={() => changeView("starred")}
            >
              <Star size={16} />
              {tr("Starred")}<span className="count">
                {flows.filter((f) => stars.includes(f.id)).length || ""}
              </span>
            </button>
            <button
              className={view === "errors" ? "nav-item active" : "nav-item"}
              onClick={() => changeView("errors")}
            >
              <Activity size={16} />
              {tr("Errors")}<span className="count">
                {flows.filter((f) => f.error || f.status >= 400).length || ""}
              </span>
            </button>
          </nav>
          <div className="nav-label domains-label">
            {tr("DOMAINS")}<span aria-live="polite">{domainQuery.trim() ? `${filteredDomains.length}/${domains.length}` : domains.length}</span>
            <button className="domain-mode" aria-label={tr("域名展示方式：{p0}，点击切换", {p0: domainGrouped ? tr("主域名分组") : tr("子域名平铺")})} aria-pressed={domainGrouped} title={domainGrouped ? tr("切换为所有域名平铺") : tr("切换为主域名分组")}
              onClick={e => { e.stopPropagation(); setDomainGrouped(value => !value); setExpandedDomains(new Map()); }}>
              {domainGrouped ? tr("分组") : tr("平铺")}
            </button>
          </div>
          <div className="domain-search">
            <Search size={13} />
            <input
              type="search" aria-label={tr("搜索域名")} placeholder={tr("搜索域名…")} value={domainQuery}
              onChange={e => { setDomainQuery(e.target.value); setExpandedDomains(new Map()); }}
              onKeyDown={e => {
                if (e.key === "Escape" && domainQuery) { e.stopPropagation(); setDomainQuery(""); }
              }}
            />
            {domainQuery ? <button aria-label={tr("清空域名搜索")} title={tr("清空域名搜索")} onClick={e => { e.stopPropagation(); setDomainQuery(""); }}><X size={12} /></button> : null}
          </div>
          <div className="domain-list">
            {filteredDomains.length ? (
              filteredDomains.map(group => {
                const expandable = group.children.length > 1 || group.children[0]?.name !== group.name;
                const expanded = expandedDomains.get(group.name) ?? !!domainQuery.trim();
                return <div className="domain-group" key={group.name}>
                    <button
                      className={host === group.name && (domainGrouped ? hostScope === "root" || !expandable : hostScope === "host") ? "domain active" : "domain"}
                      title={`${group.name} · ${expandable ? tr("点击{p0}子域名", {p0: expanded ? tr("收起") : tr("展开")}) : tr("筛选此域名的请求")}`}
                      aria-expanded={expandable ? expanded : undefined}
                      onMouseEnter={e => measureDomain(e.currentTarget)} onFocus={e => measureDomain(e.currentTarget)}
                      onClick={e => {
                        if (expandable) {
                          if (compact) e.stopPropagation();
                          setExpandedDomains(old => { const next = new Map(old); next.set(group.name, !expanded); return next; });
                        }
                        const scope = domainGrouped ? "root" : "host";
                        setHost(expandable ? group.name : host === group.name && hostScope === scope ? "" : group.name);
                        setHostScope(scope); setView("all");
                      }}
                    >
                      <span className="domain-name"><span className="domain-static">{group.name}</span><span className="domain-scroll" aria-hidden="true">{group.name}</span></span>
                      <small>{group.count}</small>
                    </button>
                  {expandable && expanded ? <div className="domain-children">
                    {group.children.map(child => <button
                      key={child.name} title={child.name} className={host === child.name && hostScope === "host" ? "domain active" : "domain"}
                      onMouseEnter={e => measureDomain(e.currentTarget)} onFocus={e => measureDomain(e.currentTarget)}
                      onClick={() => { setHost(host === child.name && hostScope === "host" ? "" : child.name); setHostScope("host"); setView("all"); }}>
                      <span className="domain-name"><span className="domain-static">{child.name}</span><span className="domain-scroll" aria-hidden="true">{child.name}</span></span><small>{child.count}</small>
                    </button>)}
                  </div> : null}
                </div>;
              })
            ) : (
              <div className="domain-placeholder">
                <Globe2 size={16} />
                <p>
                  {domains.length ? tr("没有匹配的域名") : <>{tr("Captured hosts")}<br />{tr("will appear here.")}</>}
                </p>
              </div>
            )}
          </div>
        </aside>
        <main className="main-workspace">
          {state.error || state.recoveryPending || state.storageError ? (
            <div className="network-alert" role="alert">
              <span>
                {diagnostic(state.error ||
                  state.storageError ||
                  tr("发现未恢复的系统代理配置"))}
              </span>
              <button
                onClick={() =>
                  api
                    .RecoverNetwork()
                    .then(setState)
                    .catch((e) => notify(errorText(e)))
                }
              >
                {tr("恢复网络")}</button>
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
                    placeholder={tr("Filter by URL, method, status…")}
                    aria-label={tr("搜索请求")}
                  />
                  <kbd>{state.platform === "darwin" ? "⌘" : "Ctrl"} F</kbd>
                  {query ? (
                    <button aria-label={tr("清空搜索")} onClick={() => setQuery("")}>
                      <X size={12} />
                    </button>
                  ) : null}
                </label>
                <div className="scroll-mode" role="group" aria-label={tr("流量列表滚动模式")}>
                  <button className={autoScroll ? "active" : ""} aria-pressed={autoScroll} title={tr("触底：始终跟随最新请求")} onClick={() => setAutoScroll(true)}><ArrowDown size={12} />{tr("触底")}</button>
                  <button className={!autoScroll ? "active" : ""} aria-pressed={!autoScroll} title={tr("定住：保持当前阅读位置，手动滚动")} onClick={() => setAutoScroll(false)}><Pin size={12} />{tr("定住")}</button>
                </div>
                <button
                  className="icon-button"
                  aria-label={
                    state.recording ? tr("暂停记录新请求") : tr("继续记录请求")
                  }
                  title={
                    state.recording ? tr("暂停记录（保持代理转发）") : tr("继续记录")
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
                  aria-label={tr("清空会话")}
                  title={tr("清空本地会话")}
                  onClick={clear}
                >
                  <Trash2 size={15} />
                </button>
              </div>
              <div className="resource-filters">
                {filters.map((f) => (
                  <button
                    className={type === f ? "active" : ""}
                    key={tr(f)}
                    onClick={() => setType(f)}
                  >
                    {tr(f)}
                  </button>
                ))}
              </div>
              <div
                className="request-table"
                role="table"
                aria-label={tr("捕获的网络请求")}
                aria-rowcount={visible.length + 1}
              >
                <div className="table-head request-grid" role="row">
                  {requestColumns.map(column => {
                    const active = sort.key === column.key;
                    const ascending = active && sort.direction === "asc";
                    return <div key={column.key} role="columnheader" aria-sort={active ? ascending ? "ascending" : "descending" : "none"} className={active ? "sorted-column" : ""}>
                      <button title={tr("{p0} · 点击{p1}排序", {p0: tr(column.description), p1: active ? ascending ? tr("逆序") : tr("正序") : tr("正序")})}
                        onClick={() => setSort(current => ({ key: column.key, direction: current.key === column.key && current.direction === "asc" ? "desc" : "asc" }))}>
                        <span>{tr(column.label)}</span>{active ? ascending ? <ArrowUp size={9} /> : <ArrowDown size={9} /> : <ArrowUpDown size={9} />}
                      </button>
                    </div>;
                  })}
                </div>
                <div className="table-scroll" ref={scroll} onScroll={e => { viewportAnchor.current = listAnchor(visible, e.currentTarget.scrollTop); }}>
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
                              title={f.method}
                            >
                              {f.method.slice(0, 4)}
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
                              {tr(resourceType(f.contentType, f.url))}
                            </span>
                            <span role="cell" className="number">
                              {size(f.responseSize)}
                            </span>
                            <span role="cell" className="number request-age"><RequestAge started={f.started} /></span>
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
                              {stars.includes(f.id) ? <span className="row-star starred" title={tr("已收藏")}><Star size={11} fill="currentColor" /></span> : null}
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
                      <span className="eyebrow">{tr("YOUR NETWORK, IN FOCUS")}</span>
                      <h2>
                        {flows.length
                          ? tr("Nothing matches. Yet.")
                          : capturing
                            ? tr("Listening to your network.")
                            : tr("Every request tells a story.")}
                      </h2>
                      <p>
                        {flows.length
                          ? tr("调整筛选条件，找到你关心的请求。")
                          : capturing
                            ? tr("在浏览器或应用中发起请求，流量将在这里实时出现。")
                            : tr("Capture, inspect, and understand your traffic. 从一个请求开始，看清网络的每一个细节。")}
                      </p>
                      {!flows.length ? <div className="capture-guide" aria-label={tr("首次抓包指引")}>
                        <div><strong>{tr("1 · HTTPS 证书")}</strong><span>{certificate ? certificate.trusted ? tr("已信任，可以查看 HTTPS 正文") : tr("尚未信任，HTTPS 将仅透传") : tr("正在检查证书…")}</span><button className="secondary" onClick={() => setCertOpen(true)}>{tr("管理证书")}</button></div>
                        <div><strong>{tr("2 · 选择流量来源")}</strong><span>{android.enabled ? tr("Android 手机代理已开启") : state.running ? state.systemProxy ? tr("电脑系统代理已开启") : tr("手动代理模式") : systemProxy ? tr("开始后启用电脑系统代理") : tr("开始后需手动配置代理")}</span><button className="secondary" onClick={() => setAndroidOpen(true)}>{tr("Android 手机")}</button></div>
                        <div><strong>{tr("3 · 发起请求")}</strong><span>{tr("访问网页或操作应用，再选择请求查看详情。")}</span></div>
                      </div> : null}
                      {!flows.length && !capturing ? (
                        <button
                          className="primary"
                          disabled={busy}
                          onClick={toggleCapture}
                        >
                          <Play size={13} fill="currentColor" />
                          {tr("Start your first capture")}</button>
                      ) : null}
                      <div className="empty-capabilities">
                        <span>
                          <LockKeyhole size={12} />
                          HTTPS
                        </span>
                        <span>
                          <Zap size={12} />
                          {tr("Stream first")}</span>
                        <span>
                          <HardDrive size={12} />
                          {tr("Local only")}</span>
                      </div>
                      {!isDesktop() ? (
                        <small className="preview-note">
                          {tr("Browser preview · 网络功能请在桌面应用中使用")}</small>
                      ) : null}
                    </div>
                  )}
                </div>
              </div>
              <div className="table-footer">
                <span>
                  {tr("显示 {p0} / {p1} 条请求", {p0: visible.length, p1: stats.total})}
                </span>
                <span>{tr("已捕获 {p0}", {p0: size(stats.bytes)})}</span>
                <span className="local-indicator">
                  <span className="dot" />
                  {capturing ? tr("Capturing locally") : tr("Local session")}
                </span>
              </div>
            </section>
            <div
              role="separator"
              aria-label={compact ? tr("调整列表和详情的高度") : tr("调整详情面板宽度")}
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
              onFilter={() => { setHost(domainName(requestMenu.flow)); setHostScope("host"); setView("all"); if (compact) setSelected(null); }}
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
          certificate={certificate}
          onOpenCertificate={() => { setSettingsOpen(false); setCertOpen(true); }}
          notify={notify}
        />
      ) : null}
      {toast ? (
        <div className="toast" role="status">
          <CircleHelp size={16} />
          <span>{diagnostic(toast)}</span>
          <button aria-label={tr("关闭提示")} onClick={() => setToast("")}>
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
  certificate,
  onOpenCertificate,
  notify,
}: {
  state: AppState;
  dark: boolean;
  setDark: (v: boolean) => void;
  system: boolean;
  setSystem: (v: boolean) => void;
  onClose: () => void;
  certificate: Certificate | null;
  onOpenCertificate: () => void;
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
        <span>{tr("WORKSPACE SETTINGS")}</span>
        <button aria-label={tr("关闭设置")} onClick={onClose}>
          <X size={18} />
        </button>
      </div>
      <h2>{tr("Make yourself at home.")}</h2>
      <p className="settings-description">{tr("网络接管方式、Codex、CA 证书、界面与本地数据。")}</p>
      <label className="setting-row">
        <div><strong>{tr("语言")}</strong><span>{tr("界面语言会自动保存，下次启动继续使用。")}</span></div>
        <select aria-label={tr("语言")} value={getLocale()} onChange={event => setLocale(event.target.value === "zh-CN" ? "zh-CN" : "en")}>
          <option value="en">English</option><option value="zh-CN">简体中文</option>
        </select>
      </label>
      <div className="setting-row">
        <div>
          <strong>{tr("CA Certificate")}</strong>
          <span>{certificate?.trusted ? tr("当前 CA 已信任，可用于 HTTPS 抓包") : certificate?.exists ? tr("当前 CA 尚未信任") : tr("配置 HTTPS 抓包所需的 CA 证书")}</span>
        </div>
        <button className="secondary" onClick={onOpenCertificate}>
          <ShieldCheck size={15} />  {tr("管理证书")}</button>
      </div>
      <CodexConnection />
      <label className="setting-row">
        <div>
          <strong>{tr("System proxy")}</strong>
          <span>{tr("开启抓包时接管遵循系统代理的应用")}</span>
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
        {tr("关闭后使用手动代理：")}<code>{state.address}</code>{tr("。忽略系统代理的应用、非 HTTP 协议及证书固定暂不支持。系统模式会自动串联原有 HTTP/HTTPS 代理。")}</div>
      <label className="setting-row">
        <div>
          <strong>{tr("Dark appearance")}</strong>
          <span>{tr("为深夜调试准备的深色界面")}</span>
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
          <strong>{tr("Capture limits")}</strong>
          <span>{tr("请求 / 普通响应 1 MiB · HTML 响应 8 MiB · 3,000 条 · 正文 256 MiB")}</span>
        </div>
        <HardDrive size={17} />
      </div>
      <div className="setting-note">
        {tr("所有正文计入磁盘预算，解压也有上限。队列繁忙时停止保存当前正文，网络继续转发；旧请求自动淘汰。HAR 和正文导出均受预览上限约束。SSE 持续采集正文前缀，WebSocket 首版仅透传。")}</div>
      <div className="setting-row">
        <div>
          <strong>{tr("Network recovery")}</strong>
          <span>{tr("恢复启动抓包前保存的系统代理配置")}</span>
        </div>
        <button
          className="secondary"
          onClick={() =>
            api
              .RecoverNetwork()
              .then(() => notify(tr("系统代理配置已恢复")))
              .catch((e) => notify(errorText(e)))
          }
        >
          {tr("恢复网络")}</button>
      </div>
      <div className="setting-note">
        {tr("流量保存在本机。HAR、cURL 和正文可能包含 Cookie、令牌和个人信息，分享前请检查并脱敏。 连接 AI 客户端后，工具返回的内容可能发送至模型服务；常见字段脱敏不保证所有正文安全。")}</div>
      <div className="settings-storage">
        <span>{tr("LOCAL DATA DIRECTORY")}</span>
        <code>{state.dataDirectory}</code>
      </div>
    </dialog>
  );
}
export default App;
