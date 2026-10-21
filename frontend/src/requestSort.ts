import type { Flow } from "./types";
export type RequestSortKey = "status" | "method" | "name" | "type" | "size" | "age" | "duration" | "waterfall";
export type RequestSort = { key: RequestSortKey; direction: "asc" | "desc" };
const text = new Intl.Collator(undefined, { numeric: true, sensitivity: "base" });

export function sortRequests(flows: Flow[], sort: RequestSort, resourceType: (mime: string, url: string) => string): Flow[] {
  const labels = sort.key === "type" ? new Map(flows.map(flow => [flow.id, resourceType(flow.contentType, flow.url)])) : null;
  const direction = sort.direction === "asc" ? 1 : -1;
  return [...flows].sort((a, b) => {
    let result = 0;
    switch (sort.key) {
      case "status": result = (a.error ? -1 : a.status) - (b.error ? -1 : b.status); break;
      case "method": result = text.compare(a.method, b.method); break;
      case "name": result = text.compare(a.path.split("?")[0], b.path.split("?")[0]) || text.compare(a.host, b.host); break;
      case "type": result = text.compare(labels!.get(a.id)!, labels!.get(b.id)!); break;
      case "size": result = a.responseSize - b.responseSize; break;
      // The Age column orders by the event timestamp; descending means newest first.
      case "age": result = a.started - b.started; break;
      case "duration": case "waterfall": result = a.duration - b.duration; break;
    }
    return result * direction || b.started - a.started || text.compare(a.id, b.id);
  });
}

export function newestRequestIndex(flows: Flow[]): number {
  let newest = -1;
  for (let index = 0; index < flows.length; index++) {
    if (newest < 0 || flows[index].started > flows[newest].started || (flows[index].started === flows[newest].started && flows[index].id > flows[newest].id)) newest = index;
  }
  return newest;
}
