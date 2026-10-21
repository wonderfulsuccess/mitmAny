import type { Flow } from "./types";
export type DomainGroup = { name: string; count: number; children: { name: string; count: number }[] };
export function domainName(flow: Flow): string {
  if (flow.domain) return flow.domain;
  try { return new URL(flow.url).hostname.replace(/^\[|\]$/g, "").toLowerCase(); } catch { return flow.host; }
}
export function groupDomains(flows: Flow[], grouped = true): DomainGroup[] {
  type Activity = { count: number; lastRequest: number };
  const groups = new Map<string, Activity & { children: Map<string, Activity> }>();
  for (const flow of flows) {
    const domain = domainName(flow);
    const root = grouped ? flow.rootDomain || domain : domain;
    if (!root) continue;
    const started = Number.isFinite(flow.started) ? flow.started : 0;
    let group = groups.get(root);
    if (!group) { group = { count: 0, lastRequest: started, children: new Map() }; groups.set(root, group); }
    group.count++;
    group.lastRequest = Math.max(group.lastRequest, started);
    const child = group.children.get(domain) || { count: 0, lastRequest: started };
    child.count++;
    child.lastRequest = Math.max(child.lastRequest, started);
    group.children.set(domain, child);
  }
  const latestFirst = (a: [string, Activity], b: [string, Activity]) => b[1].lastRequest - a[1].lastRequest || a[0].localeCompare(b[0]);
  return [...groups].sort(latestFirst).map(([name, group]) => ({
    name, count: group.count,
    children: [...group.children].sort(latestFirst).map(([name, child]) => ({ name, count: child.count })),
  }));
}
export function searchDomainGroups(groups: DomainGroup[], search: string): DomainGroup[] {
  const q = search.trim().toLowerCase();
  if (!q) return groups;
  return groups.flatMap(group => {
    if (group.name.toLowerCase().includes(q)) return [group];
    const children = group.children.filter(child => child.name.toLowerCase().includes(q));
    return children.length ? [{ ...group, children }] : [];
  });
}
