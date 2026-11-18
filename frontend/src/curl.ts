import { tr } from "./i18n.ts";
import type { Detail } from "./types";

const quote = (value: string) => `'${value.replaceAll("'", "'\\''")}'`;

/** POSIX shell command. Body bytes never pass through a lossy text decoder. */
export function requestCurl(detail: Detail): string {
  if (detail.method === "CONNECT") throw new Error(tr("CONNECT 仅为连接隧道，请选择隧道内的实际请求"));
  if (detail.requestCaptured < detail.requestSize || (!detail.requestBody && detail.requestSize > 0))
    throw new Error(tr("请求正文尚未完整捕获，无法生成可完整复现的 cURL"));
  if (Object.keys(detail.requestTrailers ?? {}).length)
    throw new Error(tr("此请求带有尾部字段，暂不能完整导出为 cURL"));
  const parts = ["curl --globoff --path-as-is --compressed", `--request ${quote(detail.method)}`, `--url ${quote(detail.url)}`];
  if (detail.method === "HEAD") parts.push("--head");
  const skip = new Set(["content-length", "transfer-encoding", "connection", "proxy-connection", "keep-alive", "upgrade", "te", "trailer", "proxy-authorization"]);
  for (const [key, values] of Object.entries(detail.requestHeaders ?? {})) {
    if (key.toLowerCase() === "connection") for (const value of values) for (const field of value.split(",")) skip.add(field.trim().toLowerCase());
  }
  if (detail.requestEncoding !== "compressed") skip.add("content-encoding");
  let hasHost = false;
  for (const [key, values] of Object.entries(detail.requestHeaders ?? {})) {
    if (skip.has(key.toLowerCase()) || key.startsWith(":")) continue;
    if (key.toLowerCase() === "host") hasHost = true;
    for (const value of values) parts.push(`--header ${quote(value ? `${key}: ${value}` : `${key};`)}`);
  }
  if (detail.requestBody && !Object.keys(detail.requestHeaders ?? {}).some(key => key.toLowerCase() === "content-type"))
    parts.push("--header 'Content-Type:'");
  if (!hasHost && detail.host) parts.push(`--header ${quote(`Host: ${detail.host}`)}`);
  if (detail.requestBody) {
    // Validate base64 before putting it in a runnable command.
    atob(detail.requestBody);
    parts.push("--data-binary @-");
    return `printf '%s' ${quote(detail.requestBody)} | base64 -d | ` + parts.join(" \\\n  ");
  }
  return parts.join(" \\\n  ");
}
