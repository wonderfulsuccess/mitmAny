export type BodyMatch = { start: number; end: number; line: number };
/** Literal, case-insensitive matches, with source offsets preserved for Unicode. */
export function searchBody(text: string, query: string): BodyMatch[] {
  if (!query) return [];
  const pattern = new RegExp(query.replace(/[.*+?^${}()|[\]\\]/g, "\\$&"), "gi");
  const matches: BodyMatch[] = [];
  let line = 0, cursor = 0;
  for (const match of text.matchAll(pattern)) {
    const start = match.index;
    for (; cursor < start; cursor++) if (text.charCodeAt(cursor) === 10) line++;
    matches.push({ start, end: start + match[0].length, line });
  }
  return matches;
}
