import { tr } from "./i18n";
import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { Copy, Download, Filter, Star } from "lucide-react";
import { api, errorText } from "./bridge";
import { requestCurl } from "./curl";
import type { Detail, Flow, Headers } from "./types";

type Props = {
  flow: Flow; x: number; y: number; starred: boolean; anchor: HTMLElement;
  onClose: () => void; onStar: () => void; onFilter: () => void;
  notify: (message: string) => void;
};
const headerText = (headers: Headers) => Object.entries(headers ?? {}).flatMap(([name, values]) => values.map(value => `${name}: ${value}`)).join("\r\n");

export default function RequestMenu({ flow, x, y, starred, anchor, onClose, onStar, onFilter, notify }: Props) {
  const menu = useRef<HTMLDivElement>(null);
  const [detail, setDetail] = useState<Detail | null>(null);
  const [failed, setFailed] = useState(false);
  const [position, setPosition] = useState({ left: x, top: y });
  useEffect(() => {
    let active = true;
    api.GetFlow(flow.id).then(value => { if (active) setDetail(value); }).catch(e => {
      if (active) { setFailed(true); notify(errorText(e)); }
    });
    return () => { active = false; };
  }, [flow.id, notify]);
  useLayoutEffect(() => {
    const box = menu.current!.getBoundingClientRect();
    setPosition({ left: Math.max(8, Math.min(x, window.innerWidth - box.width - 8)), top: Math.max(8, Math.min(y, window.innerHeight - box.height - 8)) });
    menu.current?.querySelector<HTMLButtonElement>('button:not(:disabled)')?.focus();
  }, [x, y]);
  useEffect(() => {
    const dismiss = (event: Event) => { if (!menu.current?.contains(event.target as Node)) onClose(); };
    window.addEventListener("pointerdown", dismiss);
    window.addEventListener("scroll", dismiss, true);
    window.addEventListener("resize", onClose);
    window.addEventListener("blur", onClose);
    return () => {
      window.removeEventListener("pointerdown", dismiss);
      window.removeEventListener("scroll", dismiss, true);
      window.removeEventListener("resize", onClose);
      window.removeEventListener("blur", onClose);
    };
  }, [onClose]);
  const run = (action: () => void | Promise<unknown>) => {
    try { Promise.resolve(action()).catch(e => notify(errorText(e))); }
    catch (e) { notify(errorText(e)); }
    onClose();
  };
  const copy = async (value: string) => { await navigator.clipboard.writeText(value); notify(tr("已复制到剪贴板")); };
  const save = (request: boolean) => api.SaveBody(flow.id, request);
  return createPortal(
    <div ref={menu} className="request-context-menu" style={position} role="menu" aria-label={tr("请求操作")}
      onContextMenu={e => e.preventDefault()}
      onKeyDown={e => {
        e.stopPropagation();
        if (e.key === "Escape" || e.key === "Tab") { if (e.key === "Escape") e.preventDefault(); anchor.focus({ preventScroll: true }); onClose(); return; }
        const buttons = [...menu.current!.querySelectorAll<HTMLButtonElement>('button:not(:disabled)')];
        const current = buttons.indexOf(document.activeElement as HTMLButtonElement);
        let next = current;
        if (e.key === "ArrowDown") next = (current + 1) % buttons.length;
        else if (e.key === "ArrowUp") next = (current - 1 + buttons.length) % buttons.length;
        else if (e.key === "Home") next = 0;
        else if (e.key === "End") next = buttons.length - 1;
        else return;
        e.preventDefault(); buttons[next]?.focus();
      }}>
      <div className="request-menu-label" title={flow.url}>{flow.method} · {flow.host}</div>
      <button role="menuitem" disabled={!detail || flow.method === "CONNECT"} onClick={() => run(() => copy(requestCurl(detail!)))}><Copy size={13} />{tr("复制 cURL")}</button>
      <button role="menuitem" onClick={() => run(() => copy(flow.url))}><Copy size={13} />{tr("复制 URL")}</button>
      <button role="menuitem" disabled={!detail} onClick={() => run(() => copy(headerText(detail!.requestHeaders)))}><Copy size={13} />{tr("复制请求头")}</button>
      <button role="menuitem" disabled={!detail} onClick={() => run(() => copy(headerText(detail!.responseHeaders)))}><Copy size={13} />{tr("复制响应头")}</button>
      <div role="separator" />
      <button role="menuitem" disabled={!detail?.requestBody} onClick={() => run(() => save(true))}><Download size={13} />{tr("保存请求正文…")}</button>
      <button role="menuitem" disabled={!detail?.responseBody} onClick={() => run(() => save(false))}><Download size={13} />{tr("保存响应正文…")}</button>
      <div role="separator" />
      <button role="menuitem" onClick={() => run(onStar)}><Star size={13} fill={starred ? "currentColor" : "none"} />{starred ? tr("取消收藏") : tr("收藏请求")}</button>
      <button role="menuitem" onClick={() => run(onFilter)}><Filter size={13} />{tr("仅查看此域名")}</button>
      {!detail ? <div className="request-menu-label">{failed ? tr("请求详情已不可用") : tr("正在读取请求详情…")}</div> : null}
    </div>, document.body,
  );
}
