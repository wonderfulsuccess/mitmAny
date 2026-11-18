import { tr } from "./i18n";
import { useEffect, useMemo, useRef, useState } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import { ChevronDown, ChevronRight } from "lucide-react";
import { openExternalURL } from "./bridge";
import type { BodyMatch } from "./bodySearch";
import { jsonDocument, visibleJsonLines } from "./jsonDocument";

export function PlainCode({ text, matches = [], active = 0 }: { text: string; matches?: BodyMatch[]; active?: number }) {
  const scroll = useRef<HTMLDivElement>(null);
  const lines = useMemo(() => {
    let start = 0;
    return text.split("\n").map(value => { const line = { value, start }; start += value.length + 1; return line; });
  }, [text]);
  const byLine = useMemo(() => {
    const result = new Map<number, { match: BodyMatch; index: number }[]>();
    matches.forEach((match, index) => {
      // Multi-line searches highlight every covered source line.
      for (let row = match.line; row < lines.length && lines[row].start < match.end; row++) {
        const list = result.get(row) ?? [];
        list.push({ match, index }); result.set(row, list);
      }
    });
    return result;
  }, [matches, lines]);
  const virtual = useVirtualizer({ count: lines.length, getScrollElement: () => scroll.current, estimateSize: () => 18, overscan: 12 });
  useEffect(() => {
    if (!matches[active]) return;
    virtual.scrollToIndex(matches[active].line, { align: "center" });
    const frame = requestAnimationFrame(() => scroll.current?.querySelector("mark.active")?.scrollIntoView({ block: "nearest", inline: "center" }));
    return () => cancelAnimationFrame(frame);
  }, [matches, active, virtual]);
  return <div ref={scroll} className="plain-code-scroll json-code" role="region" aria-label={tr("响应正文")} tabIndex={0}>
    <div className="json-code-canvas" style={{ height: virtual.getTotalSize() }}>
      {virtual.getVirtualItems().map(item => {
        const line = lines[item.index];
        let cursor = 0;
        const parts: React.ReactNode[] = [];
        for (const { match, index } of byLine.get(item.index) ?? []) {
          const start = Math.max(0, match.start - line.start), end = Math.min(line.value.length, match.end - line.start);
          parts.push(line.value.slice(cursor, start), <mark key={index} className={index === active ? "active" : ""}>{line.value.slice(start, end)}</mark>);
          cursor = end;
        }
        parts.push(line.value.slice(cursor));
        return <div className="json-code-line" key={item.index} style={{ transform: `translateY(${item.start}px)` }}>
          <span className="json-line-number">{item.index + 1}</span><code className="json-line-content">{parts}</code>
        </div>;
      })}
    </div>
  </div>;
}

export default function JsonCode({ text, notify }: { text: string; notify: (message: string) => void }) {
  const document = useMemo(() => jsonDocument(text), [text]);
  const [folded, setFolded] = useState<Set<number>>(() => new Set());
  const scroll = useRef<HTMLDivElement>(null);
  const lines = useMemo(() => document ? visibleJsonLines(document, folded) : [], [document, folded]);
  const virtual = useVirtualizer({ count: lines.length, getScrollElement: () => scroll.current, estimateSize: () => 18, overscan: 15 });
  const toggle = (id: number) => setFolded(old => {
    const next = new Set(old);
    if (next.has(id)) next.delete(id); else next.add(id);
    return next;
  });
  const openLink = (event: React.MouseEvent, href: string) => {
    event.preventDefault();
    try { openExternalURL(href); } catch { notify(tr("无法打开链接")); }
  };
  if (!document) return <PlainCode text={text} />;
  return <div ref={scroll} className="json-code" tabIndex={0} role="region" aria-label={tr("JSON 格式化正文")}>
    <div className="json-code-canvas" style={{ height: virtual.getTotalSize() }}>
      {virtual.getVirtualItems().map(item => {
        const line = lines[item.index];
        const fold = line.fold;
        const collapsed = !!fold && folded.has(fold.id);
        const trailingComma = fold && document.lines[fold.end].tokens.at(-1)?.text === ",";
        return <div className="json-code-line" key={line.number} style={{ transform: `translateY(${item.start}px)` }}>
          <span className="json-line-number" aria-hidden="true">{line.number}</span>
          <span className="json-fold-gutter">{fold ? <button type="button" aria-label={tr("{p0}字段 {p1}", {p0: collapsed ? tr("展开") : tr("折叠"), p1: fold.id === 0 ? tr("根节点") : fold.label})} aria-expanded={!collapsed} onClick={() => toggle(fold.id)}
            onKeyDown={event => {
              if ((event.key === "ArrowLeft" && !collapsed) || (event.key === "ArrowRight" && collapsed)) { event.preventDefault(); toggle(fold.id); }
            }}>{collapsed ? <ChevronRight size={11} /> : <ChevronDown size={11} />}</button> : null}</span>
          <code className="json-line-content" style={{ paddingLeft: line.depth * 12 }}>
            {line.tokens.map((token, i) => token.href ? <a key={i} className="json-string json-link" href={token.href} target="_blank" rel="noopener noreferrer" title={tr("在浏览器打开 {p0}", {p0: token.href})} onClick={e => openLink(e, token.href!)}
              onAuxClick={e => { if (e.button === 1) openLink(e, token.href!); }}>{token.text}</a> : <span key={i} className={`json-${token.kind}`}>{token.text}</span>)}
            {collapsed && fold ? <><button className="json-fold-preview" aria-label={tr("展开字段 {p0}", {p0: fold.id === 0 ? tr("根节点") : fold.label})} onClick={() => toggle(fold.id)}>… {fold.count}  {tr("项")}</button><span className="json-punctuation">{fold.close}{trailingComma ? "," : ""}</span></> : null}
          </code>
        </div>;
      })}
    </div>
  </div>;
}
