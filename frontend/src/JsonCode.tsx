import { useMemo, useRef, useState } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import { ChevronDown, ChevronRight } from "lucide-react";
import { openExternalURL } from "./bridge";
import { jsonDocument, visibleJsonLines } from "./jsonDocument";

export function PlainCode({ text }: { text: string }) {
  const lines = useMemo(() => text.split("\n", 5001), [text]);
  return <div className="plain-code-scroll"><pre className="code-view">
    {lines.slice(0, 5000).map((line, i) => <div className="code-line" key={i}><span>{i + 1}</span><code>{line || " "}</code></div>)}
    {lines.length > 5000 ? <div className="notice">仅显示前 5,000 行；复制或保存可获取完整已捕获正文。</div> : null}
  </pre></div>;
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
    try { openExternalURL(href); } catch { notify("无法打开链接"); }
  };
  if (!document) return <PlainCode text={text} />;
  return <div ref={scroll} className="json-code" tabIndex={0} role="region" aria-label="JSON 格式化正文">
    <div className="json-code-canvas" style={{ height: virtual.getTotalSize() }}>
      {virtual.getVirtualItems().map(item => {
        const line = lines[item.index];
        const fold = line.fold;
        const collapsed = !!fold && folded.has(fold.id);
        const trailingComma = fold && document.lines[fold.end].tokens.at(-1)?.text === ",";
        return <div className="json-code-line" key={line.number} style={{ transform: `translateY(${item.start}px)` }}>
          <span className="json-line-number" aria-hidden="true">{line.number}</span>
          <span className="json-fold-gutter">{fold ? <button type="button" aria-label={`${collapsed ? "展开" : "折叠"}字段 ${fold.label}`} aria-expanded={!collapsed} onClick={() => toggle(fold.id)}
            onKeyDown={event => {
              if ((event.key === "ArrowLeft" && !collapsed) || (event.key === "ArrowRight" && collapsed)) { event.preventDefault(); toggle(fold.id); }
            }}>{collapsed ? <ChevronRight size={11} /> : <ChevronDown size={11} />}</button> : null}</span>
          <code className="json-line-content" style={{ paddingLeft: line.depth * 12 }}>
            {line.tokens.map((token, i) => token.href ? <a key={i} className="json-string json-link" href={token.href} target="_blank" rel="noopener noreferrer" title={`在浏览器打开 ${token.href}`} onClick={e => openLink(e, token.href!)}
              onAuxClick={e => { if (e.button === 1) openLink(e, token.href!); }}>{token.text}</a> : <span key={i} className={`json-${token.kind}`}>{token.text}</span>)}
            {collapsed && fold ? <><button className="json-fold-preview" aria-label={`展开字段 ${fold.label}`} onClick={() => toggle(fold.id)}>… {fold.count} 项</button><span className="json-punctuation">{fold.close}{trailingComma ? "," : ""}</span></> : null}
          </code>
        </div>;
      })}
    </div>
  </div>;
}
