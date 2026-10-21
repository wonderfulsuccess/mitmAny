export const REQUEST_ROW_HEIGHT = 38;
export type ListAnchor = { id: string; offset: number };
export function listAnchor(rows: { id: string }[], scrollTop: number): ListAnchor | null {
  const index = Math.max(0, Math.min(rows.length - 1, Math.floor(scrollTop / REQUEST_ROW_HEIGHT)));
  return rows[index] ? { id: rows[index].id, offset: scrollTop - index * REQUEST_ROW_HEIGHT } : null;
}
export function anchorOffset(rows: { id: string }[], anchor: ListAnchor | null): number | null {
  if (!anchor) return null;
  const index = rows.findIndex(row => row.id === anchor.id);
  return index < 0 ? null : Math.max(0, index * REQUEST_ROW_HEIGHT + anchor.offset);
}
