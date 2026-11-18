import { tr, diagnostic, getLocale } from "./i18n";
import { useDeferredValue, useEffect, useMemo, useRef, useState } from "react";
import {
  ArrowDown,
  ArrowLeft,
  ArrowUp,
  Check,
  ChevronDown,
  Copy,
  Download,
  FileCode2,
  Globe2,
  LockKeyhole,
  Maximize2,
  Minimize2,
  Search,
  X,
} from "lucide-react";
import { searchBody } from "./bodySearch";
import JsonCode, { PlainCode } from "./JsonCode";
import type { Detail, Headers } from "./types";
import { api, size } from "./bridge";

const tabs = ["Overview", "Headers", "Request", "Response", "Timing"] as const;
type Tab = (typeof tabs)[number];
function HeaderGroup({ title, headers }: { title: string; headers: Headers }) {
  return (
    <details open className="header-group">
      <summary>
        <ChevronDown size={13} />
        {title}
        <span>{Object.keys(headers ?? {}).length}</span>
      </summary>
      <div>
        {Object.entries(headers ?? {}).map(([name, values]) => (
          <div className="header-pair" key={name}>
            <span>{name}</span>
            <code>{values.join("\n")}</code>
          </div>
        ))}
      </div>
    </details>
  );
}
function textForBytes(bytes: Uint8Array, mime = "") {
  const charset = /charset\s*=\s*["']?([^\s;"']+)/i.exec(mime)?.[1] || "utf-8";
  try { return new TextDecoder(charset).decode(bytes); }
  catch { return new TextDecoder().decode(bytes); }
}
type BodyProps = {
  detail: Detail;
  request: boolean;
  notify: (message: string) => void;
  responseSearch: string;
  onResponseSearchChange: (query: string) => void;
};
function BodyContent({ detail, request, notify, responseSearch, onResponseSearchChange }: BodyProps) {
  const [expanded, setExpanded] = useState(false);
  const bodyView = useRef<HTMLDivElement>(null);
  const expandButton = useRef<HTMLButtonElement>(null);
  const [mode, setMode] = useState("Pretty");
  const query = request ? "" : responseSearch;
  const setQuery = onResponseSearchChange;
  const deferredQuery = useDeferredValue(query);
  const [selection, setSelection] = useState({ query: "", index: 0 });
  const searchInput = useRef<HTMLInputElement>(null);
  const [imageFailed, setImageFailed] = useState(false);
  const raw = request ? detail.requestBody : detail.responseBody;
  const encoding = request ? detail.requestEncoding : detail.responseEncoding;
  const mime =
    (request ? detail.requestHeaders : detail.responseHeaders)?.[
      "Content-Type"
    ]?.[0] ?? "";
  const headers = request ? detail.requestHeaders : detail.responseHeaders;
  const contentEncoding = Object.entries(headers ?? {}).find(([name]) => name.toLowerCase() === "content-encoding")?.[1]?.join(", ") ?? "";
  const decoded = encoding === "base64" && contentEncoding && contentEncoding.toLowerCase() !== "identity";
  const normalizedMime = mime.toLowerCase();
  const isImage = normalizedMime.startsWith("image/") && !normalizedMime.includes("svg") && encoding === "base64";
  const isText = !mime || /text|json|javascript|xml|svg|form-urlencoded|event-stream/.test(normalizedMime);
  const bytes = useMemo(() => {
    try { return Uint8Array.from(atob(raw), c => c.charCodeAt(0)); }
    catch { return new Uint8Array(); }
  }, [raw]);
  const text = useMemo(() => isText && encoding === "base64" ? textForBytes(bytes, mime) : "", [bytes, mime, isText, encoding]);
  const matches = useMemo(() => searchBody(text, deferredQuery), [text, deferredQuery]);
  const activeMatch = selection.query === deferredQuery ? selection.index % Math.max(1, matches.length) : 0;
  const moveMatch = (direction: number) => setSelection({ query: deferredQuery, index: (activeMatch + direction + matches.length) % Math.max(1, matches.length) });
  const objectURL = useMemo(() => {
    if (!raw || encoding !== "base64" || (!isImage && !mime.includes("pdf")))
      return "";
    return URL.createObjectURL(new Blob([bytes], { type: mime }));
  }, [bytes, raw, encoding, isImage, mime]);
  useEffect(
    () => () => {
      if (objectURL) URL.revokeObjectURL(objectURL);
    },
    [objectURL],
  );
  useEffect(() => {
    if (!expanded || !bodyView.current) return;
    const panel = bodyView.current;
    const previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    const siblings: { element: HTMLElement; inert: boolean }[] = [];
    // Keep the same viewer mounted, preserving folds, search and scroll position.
    for (let ancestor: HTMLElement | null = panel; ancestor?.parentElement; ancestor = ancestor.parentElement) {
      for (const sibling of ancestor.parentElement.children) {
        if (sibling instanceof HTMLElement && sibling !== ancestor) {
          siblings.push({ element: sibling, inert: sibling.inert });
          sibling.inert = true;
        }
      }
    }
    panel.focus({ preventScroll: true });
    const keyboard = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault(); event.stopPropagation(); setExpanded(false);
      } else if (event.key === "Tab") {
        const controls = [...panel.querySelectorAll<HTMLElement>('button:not(:disabled), input, [tabindex="0"], a[href]')].filter(element => element.getClientRects().length > 0);
        const first = controls[0], last = controls.at(-1);
        if (event.shiftKey && (document.activeElement === first || document.activeElement === panel)) {
          event.preventDefault(); last?.focus();
        } else if (!event.shiftKey && (document.activeElement === last || document.activeElement === panel)) {
          event.preventDefault(); first?.focus();
        }
      }
    };
    window.addEventListener("keydown", keyboard, true);
    return () => {
      window.removeEventListener("keydown", keyboard, true);
      siblings.forEach(({ element, inert }) => { element.inert = inert; });
      const target = expandButton.current ?? previousFocus;
      if (target?.isConnected) target.focus({ preventScroll: true });
    };
  }, [expanded]);
  if (!raw) {
    const bodySize = request ? detail.requestSize : detail.responseSize;
    const missing = bodySize > 0;
    const waiting = !detail.complete && (request ? !detail.status : true);
    return (
      <div className="body-empty">
        <FileCode2 size={30} />
        <h3>{missing ? tr("正文副本不可用") : waiting ? tr("等待正文…") : request ? tr("此请求没有正文") : tr("此响应没有正文")}</h3>
        <p>{missing
          ? tr("已检测到正文传输，但未保存到正文副本，可能因采集繁忙或存储失败而缺失。")
          : waiting ? tr("收到数据后会自动显示。")
          : request ? tr("URL 参数显示在上方；只有实际发送了 Body 的请求才会在这里显示正文。")
          : detail.method === "HEAD" || detail.status === 204 || detail.status === 304 ? tr("此类响应不包含正文。") : tr("服务器未返回正文。")}</p>
      </div>
    );
  }
  return (
    <div ref={bodyView} className={`body-view ${expanded ? "response-expanded" : ""}`}
      role={expanded ? "dialog" : undefined} aria-modal={expanded || undefined} aria-label={expanded ? tr("Response 大屏浏览") : undefined}
      tabIndex={expanded ? -1 : undefined} onKeyDown={event => { if (expanded) event.stopPropagation(); }}>
      <div className="body-toolbar">
        {expanded ? <strong className="response-title" title={detail.url}>Response · {detail.path.split("?")[0]}</strong> : null}
        <div className="segmented">
          {["Pretty", "Raw"].map((v) => (
            <button
              key={v}
              className={mode === v ? "active" : ""}
              onClick={() => setMode(v)}
            >
              {tr(v)}
            </button>
          ))}
        </div>
        <span title={decoded ? tr("已解压 {p0}，显示解压后的正文", {p0: contentEncoding}) : undefined}>{decoded ? tr("已解压 {p0} · ", {p0: contentEncoding}) : ""}{size(bytes.length)}</span>
        <button
          title={tr("复制正文")}
          aria-label={tr("复制正文")}
          disabled={!isText || encoding !== "base64"}
          onClick={() =>
            navigator.clipboard
              .writeText(text)
              .then(() => notify(tr("正文已复制")))
              .catch(() => notify(tr("无法访问剪贴板")))
          }
        >
          <Copy size={14} />
        </button>
        <button
          title={tr("下载正文（自动选择文件格式）")}
          aria-label={tr("下载正文（自动选择文件格式）")}
          onClick={() =>
            api.SaveBody(detail.id, request).catch((e) => notify(diagnostic(String(e))))
          }
        >
          <Download size={14} />
        </button>
        {!request ? <button ref={expandButton} title={expanded ? tr("退出大屏浏览（Esc）") : tr("大屏浏览 Response")} aria-label={expanded ? tr("退出大屏浏览") : tr("大屏浏览 Response")} aria-expanded={expanded} onClick={() => setExpanded(value => !value)}>
          {expanded ? <Minimize2 size={14} /> : <Maximize2 size={14} />}
        </button> : null}
      </div>
      {!request && isText && encoding === "base64" ? <div className="body-search">
        <Search size={13} />
        <input ref={searchInput} aria-label={tr("搜索响应正文")} placeholder={tr("搜索 Response…")} value={query} onChange={event => setQuery(event.target.value)} onKeyDown={event => {
          if (event.key === "Enter") { event.preventDefault(); moveMatch(event.shiftKey ? -1 : 1); }
          if (event.key === "Escape") { event.stopPropagation(); setQuery(""); }
        }} />
        <span role="status" aria-live="polite">{query !== deferredQuery ? tr("搜索中…") : query ? `${matches.length ? activeMatch + 1 : 0} / ${matches.length}` : ""}</span>
        <button aria-label={tr("上一个匹配")} title={tr("上一个（Shift+Enter）")} disabled={!matches.length} onClick={() => moveMatch(-1)}><ArrowUp size={13} /></button>
        <button aria-label={tr("下一个匹配")} title={tr("下一个（Enter）")} disabled={!matches.length} onClick={() => moveMatch(1)}><ArrowDown size={13} /></button>
        {query ? <button aria-label={tr("清除搜索")} onClick={() => { setQuery(""); searchInput.current?.focus(); }}><X size={13} /></button> : null}
      </div> : null}
      {encoding === "compressed" ? (
        <div className="notice">
          { /html/i.test(mime) ? tr("压缩正文不完整、解压后超过 8 MiB 或编码暂不支持，可保存已捕获的原始字节。") : tr("压缩正文不完整、解压后超过 1 MiB 或编码暂不支持，可保存已捕获的原始字节。") }
        </div>
      ) : isImage && mode === "Pretty" ? (
        <div className="image-preview">
          {imageFailed ? (
            <p>{tr("图片副本不完整或格式不受支持，可保存已捕获的字节。")}</p>
          ) : (
            <img
              src={objectURL}
              alt={tr("Captured response")}
              onError={() => setImageFailed(true)}
            />
          )}
          <span>{mime}</span>
        </div>
      ) : mime.includes("pdf") && mode === "Pretty" && objectURL ? (
        <iframe
          sandbox=""
          title={tr("PDF preview")}
          src={objectURL}
          className="pdf-preview"
        />
      ) : isText ? (
        mode === "Pretty" && !deferredQuery ? <JsonCode text={text} notify={notify} /> : <PlainCode text={text} matches={matches} active={activeMatch} />
      ) : (
        <div className="body-empty">
          <FileCode2 size={30} />
          <h3>{tr("Binary response")}</h3>
          <p>
            {mime} · {size(bytes.length)}
          </p>
          <button
            className="secondary"
            onClick={() =>
              api.SaveBody(detail.id, request).catch((e) => notify(diagnostic(String(e))))
            }
          >
            {tr("保存正文")}</button>
        </div>
      )}
    </div>
  );
}
function Body(props: BodyProps) {
  if (!props.request) return <BodyContent {...props} />;
  let params: [string, string][] = [];
  try { params = [...new URL(props.detail.url).searchParams.entries()]; } catch { /* malformed capture URL */ }
  return <div className="request-body-panel">
    <details open className="request-query-params">
      <summary>{tr("URL 参数")} <span>{params.length}</span></summary>
      {params.length ? <dl>{params.map(([name, value], index) => <div key={index}><dt>{name}</dt><dd>{value || tr("（空值）")}</dd></div>)}</dl> : <p>{tr("此 URL 没有查询参数。")}</p>}
    </details>
    <BodyContent {...props} />
  </div>;
}
export default function Inspector({
  detail,
  loading,
  compact = false,
  onClose,
  notify,
}: {
  detail: Detail | null;
  loading: boolean;
  compact?: boolean;
  onClose: () => void;
  notify: (message: string) => void;
}) {
  const [tab, setTab] = useState<Tab>("Overview");
  // The inspector survives request changes, temporary loading and closing details.
  const [responseSearch, setResponseSearch] = useState("");
  const [copied, setCopied] = useState(false);
  if (!detail)
    return (
      <aside className="inspector inspector-empty">
        <div className="inspect-symbol">
          <FileCode2 size={28} />
        </div>
        <h3>{loading ? tr("Loading request…") : tr("A closer look.")}</h3>
        <p>{tr("选择一条请求，查看每一个细节。")}</p>
        <div className="inspect-hints">
          <span>
            <LockKeyhole size={14} />  {tr("HTTPS inspection")}</span>
          <span>
            <ArrowDown size={14} />  {tr("Streaming responses")}</span>
          <span>
            <FileCode2 size={14} />  {tr("Rich body previews")}</span>
        </div>
        <kbd>↑</kbd>
        <kbd>↓</kbd>
        <small>{tr("浏览请求")}</small>
      </aside>
    );
  const copy = async (text: string) => {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      setTimeout(() => setCopied(false), 1400);
    } catch {
      notify(tr("无法访问剪贴板"));
    }
  };
  const params = (() => {
    try {
      return [...new URL(detail.url).searchParams.entries()];
    } catch {
      return [];
    }
  })();
  return (
    <aside className="inspector" onKeyDownCapture={event => {
      if (tab === "Response" && (event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "f") {
        const input = event.currentTarget.querySelector<HTMLInputElement>('.body-search input');
        if (input) { event.preventDefault(); event.stopPropagation(); input.focus(); input.select(); }
      }
    }}>
      <div className="inspector-heading">
        <span className={`method ${detail.method.toLowerCase()}`}>
          {detail.method}
        </span>
        <span className="detail-path" title={detail.path}>
          {detail.path.split("?")[0]}
        </span>
        <button aria-label={compact ? tr("返回请求列表") : tr("关闭详情")} title={compact ? tr("返回请求列表") : tr("关闭详情")} onClick={onClose}>
          {compact ? <><ArrowLeft size={14} />  {tr("返回列表")}</> : <X size={16} />}
        </button>
      </div>
      <div className="url-line">
        {detail.scheme === "https" ? (
          <LockKeyhole size={12} />
        ) : (
          <Globe2 size={12} />
        )}
        <span title={detail.url}>{detail.url}</span>
        <button
          title={tr("复制 URL")}
          aria-label={tr("复制 URL")}
          onClick={() => copy(detail.url)}
        >
          {copied ? <Check size={13} /> : <Copy size={13} />}
        </button>
      </div>
      <div className="detail-tabs" role="tablist">
        {tabs.map((t) => (
          <button
            role="tab"
            aria-selected={tab === t}
            className={tab === t ? "active" : ""}
            key={t}
            onClick={() => setTab(t)}
          >
            {tr(t)}
          </button>
        ))}
      </div>
      {detail.method === "CONNECT" && !detail.error ? <div className="notice">{tr("此连接仅通过 HTTPS 隧道转发，未解密内部请求或正文。请检查 CA 信任与 HTTPS 抓包设置。")}</div> : null}
      {detail.truncated ? (
        <div className="notice">
          {tr("正文副本不完整：")}{diagnostic(detail.captureNote || tr("采集达到上限、队列繁忙或存储失败"))}{tr("。请求与普通响应最多 1 MiB，HTML 响应最多 8 MiB；网络转发继续。")}</div>
      ) : null}
      <div className={`detail-content ${tab === "Request" || tab === "Response" ? "body-content" : ""}`}>
        {tab === "Overview" ? (
          <>
            <div className="overview-status">
              <span
                className={
                  detail.error || detail.status >= 400
                    ? "status-bubble error"
                    : "status-bubble"
                }
              >
                <span />
                {detail.error ? tr("Failed") : detail.status || tr("Pending")}
              </span>
              <span>{detail.complete ? tr("Completed") : tr("In progress")}</span>
            </div>
            <h4>{tr("GENERAL")}</h4>
            <dl className="properties">
              <dt>{tr("Request URL")}</dt>
              <dd className="break">{detail.url}</dd>
              <dt>{tr("Request method")}</dt>
              <dd>{detail.method}</dd>
              <dt>{tr("Status code")}</dt>
              <dd>{detail.status || tr("Waiting")}</dd>
              <dt>{tr("Protocol")}</dt>
              <dd>{detail.protocol}</dd>
              <dt>{tr("Content type")}</dt>
              <dd>{detail.contentType || "—"}</dd>
              <dt>{tr("Started at")}</dt>
              <dd>
                {new Date(detail.started).toLocaleTimeString(getLocale(), {
                  hour12: false,
                  hour: "2-digit",
                  minute: "2-digit",
                  second: "2-digit",
                  fractionalSecondDigits: 3,
                })}
              </dd>
            </dl>
            <h4>{tr("TRANSFER")}</h4>
            <div className="transfer-cards">
              <div>
                <ArrowUp size={15} />
                <strong>{size(detail.requestSize)}</strong>
                <span>{tr("Request body")}</span>
              </div>
              <div>
                <ArrowDown size={15} />
                <strong>{size(detail.responseSize)}</strong>
                <span>{tr("Response body")}</span>
              </div>
            </div>
            {detail.error ? (
              <div className="error-box">{diagnostic(detail.error)}</div>
            ) : null}
            {params.length ? (
              <>
                <h4>{tr("QUERY PARAMETERS")}</h4>
                {params.map(([k, v], i) => (
                  <div className="header-pair" key={i}>
                    <span>{k}</span>
                    <code>{v}</code>
                  </div>
                ))}
              </>
            ) : null}
          </>
        ) : tab === "Headers" ? (
          <>
            <HeaderGroup
              title={tr("Request headers")}
              headers={detail.requestHeaders}
            />
            <HeaderGroup
              title={tr("Response headers")}
              headers={detail.responseHeaders}
            />
            {Object.keys(detail.requestTrailers ?? {}).length ? <HeaderGroup title={tr("Request trailers")} headers={detail.requestTrailers!} /> : null}
            {Object.keys(detail.responseTrailers ?? {}).length ? <HeaderGroup title={tr("Response trailers")} headers={detail.responseTrailers!} /> : null}
          </>
        ) : tab === "Request" || tab === "Response" ? (
          <Body
            key={`${detail.id}-${tab}`}
            detail={detail}
            request={tab === "Request"}
            notify={notify}
            responseSearch={responseSearch}
            onResponseSearchChange={setResponseSearch}
          />
        ) : (
          <div className="timing-panel">
            <h4>{tr("REQUEST TIMELINE")}</h4>
            <p>{tr("从代理接收请求头开始计算。")}</p>
            <div className="timing-row">
              <span>{tr("Waiting for response")}</span>
              <strong>{detail.ttfb.toFixed(1)} ms</strong>
            </div>
            <div className="timing-track">
              <div
                style={{
                  width: `${Math.min(100, (detail.ttfb / Math.max(1, detail.duration)) * 100)}%`,
                }}
              />
            </div>
            <div className="timing-row">
              <span>{tr("Content download")}</span>
              <strong>
                {Math.max(0, detail.duration - detail.ttfb).toFixed(1)} ms
              </strong>
            </div>
            <div className="timing-track download">
              <div
                style={{
                  width: `${Math.min(100, (Math.max(0, detail.duration - detail.ttfb) / Math.max(1, detail.duration)) * 100)}%`,
                }}
              />
            </div>
            <div className="timing-total">
              <span>{tr("Total duration")}</span>
              <strong>{detail.duration.toFixed(1)} ms</strong>
            </div>
            <p>{tr("DNS、TCP 和 TLS 分阶段耗时暂未单独采集。")}</p>
          </div>
        )}
      </div>
      <div className="inspector-footer">
        <span className="dot" />{" "}
        {detail.complete ? tr("Captured locally") : tr("Streaming")}
        <span>{detail.protocol}</span>
      </div>
    </aside>
  );
}
