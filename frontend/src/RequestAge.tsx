import { useSyncExternalStore } from "react";
import { relativeRequestTime } from "./relativeTime";

// One shared timer, active only while visible request rows subscribe.
const listeners = new Set<() => void>();
let now = Date.now();
let timer: ReturnType<typeof setInterval> | undefined;
const snapshot = () => now;
function subscribe(listener: () => void) {
  listeners.add(listener);
  if (!timer) {
    now = Date.now();
    timer = setInterval(() => { now = Date.now(); for (const notify of listeners) notify(); }, 1000);
  }
  return () => {
    listeners.delete(listener);
    if (!listeners.size) { clearInterval(timer); timer = undefined; }
  };
}
export default function RequestAge({ started }: { started: number }) {
  const current = useSyncExternalStore(subscribe, snapshot);
  const valid = Number.isFinite(started) && started > 0;
  return <time dateTime={valid ? new Date(started).toISOString() : undefined} title={valid ? `请求时间：${new Date(started).toLocaleString()}` : "请求时间未知"}>{relativeRequestTime(started, current)}</time>;
}
