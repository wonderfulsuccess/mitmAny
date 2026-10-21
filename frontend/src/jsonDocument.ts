export type JsonToken = {
  kind: "key" | "string" | "number" | "boolean" | "null" | "punctuation";
  text: string;
  href?: string;
};
export type JsonFold = { id: number; end: number; count: number; close: string; label: string };
export type JsonLine = { number: number; depth: number; tokens: JsonToken[]; fold?: JsonFold };
export type JsonDocument = { lines: JsonLine[] };

export function httpURL(value: string): string | undefined {
  // Reject whitespace/control-character tricks; only explicit HTTP(S) URLs open externally.
  if (!/^https?:\/\//i.test(value) || /[\u0000-\u0020\u007f]/.test(value)) return;
  try {
    const url = new URL(value);
    if ((url.protocol === "http:" || url.protocol === "https:") && url.hostname) return url.href;
  } catch { /* Keep non-URLs as ordinary JSON strings. */ }
}

// Retain source tokens rather than re-stringifying parsed values: large integers,
// exponent notation, duplicate keys and escaped strings must remain faithful.
export function jsonDocument(text: string): JsonDocument | null {
  try { JSON.parse(text); } catch { return null; }
  const lines: JsonLine[] = [];
  let current: JsonLine | undefined;
  let depth = 0;
  const stack: { line: JsonLine; empty: boolean; count: number }[] = [];
  const pattern = /"(?:\\[\s\S]|[^"\\])*"|-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?|true|false|null|[{}\[\],:]/g;
  const push = (token: JsonToken) => {
    current ??= { number: lines.length + 1, depth, tokens: [] };
    current.tokens.push(token);
  };
  const flush = () => {
    if (current) { lines.push(current); current = undefined; }
  };
  for (const match of text.matchAll(pattern)) {
    if (lines.length >= 50000 || stack.length > 128) return null;
    const raw = match[0];
    const next = text.slice(match.index! + raw.length).match(/^\s*(.)/)?.[1];
    if (raw === "{" || raw === "[") {
      push({ kind: "punctuation", text: raw });
      const empty = next === (raw === "{" ? "}" : "]");
      if (!empty) {
        const key = current!.tokens.find(token => token.kind === "key");
        const label = key ? JSON.parse(key.text) as string : stack.length ? `[${stack[stack.length - 1].count}]` : "根节点";
        current!.fold = { id: match.index!, end: 0, count: 0, close: raw === "{" ? "}" : "]", label };
      }
      stack.push({ line: current!, empty, count: 0 });
      if (!empty) { flush(); depth++; }
    } else if (raw === "}" || raw === "]") {
      const container = stack.pop()!;
      if (!container.empty) {
        flush(); depth--;
        container.line.fold!.end = lines.length;
        container.line.fold!.count = container.count + 1;
      }
      push({ kind: "punctuation", text: raw });
    } else if (raw === ",") {
      push({ kind: "punctuation", text: raw });
      if (stack.length) stack[stack.length - 1].count++;
      flush();
    } else if (raw === ":") {
      push({ kind: "punctuation", text: ": " });
    } else {
      const kind = raw.startsWith('"') ? next === ":" ? "key" : "string" : raw === "null" ? "null" : raw === "true" || raw === "false" ? "boolean" : "number";
      push({ kind, text: raw, href: kind === "string" ? httpURL(JSON.parse(raw) as string) : undefined });
    }
  }
  flush();
  return { lines };
}

export function visibleJsonLines(document: JsonDocument, folded: ReadonlySet<number>) {
  const visible: JsonLine[] = [];
  for (let i = 0; i < document.lines.length; i++) {
    const line = document.lines[i];
    visible.push(line);
    if (line.fold && folded.has(line.fold.id)) i = line.fold.end;
  }
  return visible;
}
