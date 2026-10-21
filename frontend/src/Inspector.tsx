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
        <h3>{missing ? "正文副本不可用" : waiting ? "等待正文…" : request ? "此请求没有正文" : "此响应没有正文"}</h3>
        <p>{missing
          ? "已检测到正文传输，但未保存到正文副本，可能因采集繁忙或存储失败而缺失。"
          : waiting ? "收到数据后会自动显示。"
          : request ? "URL 参数显示在上方；只有实际发送了 Body 的请求才会在这里显示正文。"
          : detail.method === "HEAD" || detail.status === 204 || detail.status === 304 ? "此类响应不包含正文。" : "服务器未返回正文。"}</p>
      </div>
    );
  }
  return (
    <div ref={bodyView} className={`body-view ${expanded ? "response-expanded" : ""}`}
      role={expanded ? "dialog" : undefined} aria-modal={expanded || undefined} aria-label={expanded ? "Response 大屏浏览" : undefined}
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
              {v}
            </button>
          ))}
        </div>
        <span title={decoded ? `已解压 ${contentEncoding}，显示解压后的正文` : undefined}>{decoded ? `已解压 ${contentEncoding} · ` : ""}{size(bytes.length)}</span>
        <button
          title="复制正文"
          aria-label="复制正文"
          disabled={!isText || encoding !== "base64"}
          onClick={() =>
            navigator.clipboard
              .writeText(text)
              .then(() => notify("正文已复制"))
              .catch(() => notify("无法访问剪贴板"))
          }
        >
          <Copy size={14} />
        </button>
        <button
          title="下载正文（自动选择文件格式）"
          aria-label="下载正文（自动选择文件格式）"
          onClick={() =>
            api.SaveBody(detail.id, request).catch((e) => notify(String(e)))
          }
        >
          <Download size={14} />
        </button>
        {!request ? <button ref={expandButton} title={expanded ? "退出大屏浏览（Esc）" : "大屏浏览 Response"} aria-label={expanded ? "退出大屏浏览" : "大屏浏览 Response"} aria-expanded={expanded} onClick={() => setExpanded(value => !value)}>
          {expanded ? <Minimize2 size={14} /> : <Maximize2 size={14} />}
        </button> : null}
      </div>
      {!request && isText && encoding === "base64" ? <div className="body-search">
        <Search size={13} />
        <input ref={searchInput} aria-label="搜索响应正文" placeholder="搜索 Response…" value={query} onChange={event => setQuery(event.target.value)} onKeyDown={event => {
          if (event.key === "Enter") { event.preventDefault(); moveMatch(event.shiftKey ? -1 : 1); }
          if (event.key === "Escape") { event.stopPropagation(); setQuery(""); }
        }} />
        <span role="status" aria-live="polite">{query !== deferredQuery ? "搜索中…" : query ? `${matches.length ? activeMatch + 1 : 0} / ${matches.length}` : ""}</span>
        <button aria-label="上一个匹配" title="上一个（Shift+Enter）" disabled={!matches.length} onClick={() => moveMatch(-1)}><ArrowUp size={13} /></button>
        <button aria-label="下一个匹配" title="下一个（Enter）" disabled={!matches.length} onClick={() => moveMatch(1)}><ArrowDown size={13} /></button>
        {query ? <button aria-label="清除搜索" onClick={() => { setQuery(""); searchInput.current?.focus(); }}><X size={13} /></button> : null}
      </div> : null}
      {encoding === "compressed" ? (
        <div className="notice">
          { /html/i.test(mime) ? "压缩正文不完整或编码暂不支持，可保存已捕获的原始字节。" : "压缩正文不完整、解压后超过 1 MiB 或编码暂不支持，可保存已捕获的原始字节。" }
        </div>
      ) : isImage && mode === "Pretty" ? (
        <div className="image-preview">
          {imageFailed ? (
            <p>图片副本不完整或格式不受支持，可保存已捕获的字节。</p>
          ) : (
            <img
              src={objectURL}
              alt="Captured response"
              onError={() => setImageFailed(true)}
            />
          )}
          <span>{mime}</span>
        </div>
      ) : mime.includes("pdf") && mode === "Pretty" && objectURL ? (
        <iframe
          sandbox=""
          title="PDF preview"
          src={objectURL}
          className="pdf-preview"
        />
      ) : isText ? (
        mode === "Pretty" && !deferredQuery ? <JsonCode text={text} notify={notify} /> : <PlainCode text={text} matches={matches} active={activeMatch} />
      ) : (
        <div className="body-empty">
          <FileCode2 size={30} />
          <h3>Binary response</h3>
          <p>
            {mime} · {size(bytes.length)}
          </p>
          <button
            className="secondary"
            onClick={() =>
              api.SaveBody(detail.id, request).catch((e) => notify(String(e)))
            }
          >
            保存正文
          </button>
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
      <summary>URL 参数 <span>{params.length}</span></summary>
      {params.length ? <dl>{params.map(([name, value], index) => <div key={index}><dt>{name}</dt><dd>{value || "（空值）"}</dd></div>)}</dl> : <p>此 URL 没有查询参数。</p>}
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
        <h3>{loading ? "Loading request…" : "A closer look."}</h3>
        <p>选择一条请求，查看每一个细节。</p>
        <div className="inspect-hints">
          <span>
            <LockKeyhole size={14} /> HTTPS inspection
          </span>
          <span>
            <ArrowDown size={14} /> Streaming responses
          </span>
          <span>
            <FileCode2 size={14} /> Rich body previews
          </span>
        </div>
        <kbd>↑</kbd>
        <kbd>↓</kbd>
        <small>浏览请求</small>
      </aside>
    );
  const copy = async (text: string) => {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      setTimeout(() => setCopied(false), 1400);
    } catch {
      notify("无法访问剪贴板");
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
        <button aria-label={compact ? "返回请求列表" : "关闭详情"} title={compact ? "返回请求列表" : "关闭详情"} onClick={onClose}>
          {compact ? <><ArrowLeft size={14} /> 返回列表</> : <X size={16} />}
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
          title="复制 URL"
          aria-label="复制 URL"
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
            {t}
          </button>
        ))}
      </div>
      {detail.method === "CONNECT" && !detail.error ? <div className="notice">此连接仅通过 HTTPS 隧道转发，未解密内部请求或正文。请检查 CA 信任与 HTTPS 抓包设置。</div> : null}
      {detail.truncated ? (
        <div className="notice">
          正文副本不完整，可能因采集或存储失败而缺失；非 HTML 正文保留最多 1 MiB。
        </div>
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
                {detail.error ? "Failed" : detail.status || "Pending"}
              </span>
              <span>{detail.complete ? "Completed" : "In progress"}</span>
            </div>
            <h4>GENERAL</h4>
            <dl className="properties">
              <dt>Request URL</dt>
              <dd className="break">{detail.url}</dd>
              <dt>Request method</dt>
              <dd>{detail.method}</dd>
              <dt>Status code</dt>
              <dd>{detail.status || "Waiting"}</dd>
              <dt>Protocol</dt>
              <dd>{detail.protocol}</dd>
              <dt>Content type</dt>
              <dd>{detail.contentType || "—"}</dd>
              <dt>Started at</dt>
              <dd>
                {new Date(detail.started).toLocaleTimeString([], {
                  hour12: false,
                  hour: "2-digit",
                  minute: "2-digit",
                  second: "2-digit",
                  fractionalSecondDigits: 3,
                })}
              </dd>
            </dl>
            <h4>TRANSFER</h4>
            <div className="transfer-cards">
              <div>
                <ArrowUp size={15} />
                <strong>{size(detail.requestSize)}</strong>
                <span>Request body</span>
              </div>
              <div>
                <ArrowDown size={15} />
                <strong>{size(detail.responseSize)}</strong>
                <span>Response body</span>
              </div>
            </div>
            {detail.error ? (
              <div className="error-box">{detail.error}</div>
            ) : null}
            {params.length ? (
              <>
                <h4>QUERY PARAMETERS</h4>
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
              title="Request headers"
              headers={detail.requestHeaders}
            />
            <HeaderGroup
              title="Response headers"
              headers={detail.responseHeaders}
            />
            {Object.keys(detail.requestTrailers ?? {}).length ? <HeaderGroup title="Request trailers" headers={detail.requestTrailers!} /> : null}
            {Object.keys(detail.responseTrailers ?? {}).length ? <HeaderGroup title="Response trailers" headers={detail.responseTrailers!} /> : null}
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
            <h4>REQUEST TIMELINE</h4>
            <p>从代理接收请求头开始计算。</p>
            <div className="timing-row">
              <span>Waiting for response</span>
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
              <span>Content download</span>
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
              <span>Total duration</span>
              <strong>{detail.duration.toFixed(1)} ms</strong>
            </div>
            <p>DNS、TCP 和 TLS 分阶段耗时暂未单独采集。</p>
          </div>
        )}
      </div>
      <div className="inspector-footer">
        <span className="dot" />{" "}
        {detail.complete ? "Captured locally" : "Streaming"}
        <span>{detail.protocol}</span>
      </div>
    </aside>
  );
}
