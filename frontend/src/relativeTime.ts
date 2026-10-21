export function relativeRequestTime(started: number, now: number): string {
  if (!Number.isFinite(started) || started <= 0) return "—";
  const seconds = Math.max(0, Math.floor((now - started) / 1000));
  if (seconds >= 3600) return `${Math.floor(seconds / 3600)}H`;
  if (seconds >= 60) return `${Math.floor(seconds / 60)}M`;
  return `${seconds}S`;
}
