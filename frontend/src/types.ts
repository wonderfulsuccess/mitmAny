export type Headers = Record<string, string[]>;
export interface Flow {
  id: string;
  method: string;
  url: string;
  host: string;
  path: string;
  scheme: string;
  protocol: string;
  status: number;
  contentType: string;
  started: number;
  duration: number;
  ttfb: number;
  requestSize: number;
  responseSize: number;
  complete: boolean;
  error?: string;
  truncated: boolean;
  revision: number;
}
export interface Detail extends Flow {
  requestTrailers?: Headers;
  responseTrailers?: Headers;
  requestHeaders: Headers;
  responseHeaders: Headers;
  requestBody: string;
  responseBody: string;
  requestEncoding: string;
  responseEncoding: string;
  requestCaptured: number;
  responseCaptured: number;
}
export interface AppState {
  running: boolean;
  systemProxy: boolean;
  recording: boolean;
  address: string;
  platform: string;
  recoveryPending: boolean;
  error: string;
  storageError: string;
  dataDirectory: string;
}
export interface Certificate {
  exists: boolean;
  trusted: boolean;
  valid: boolean;
  subject: string;
  fingerprint: string;
  expires: string;
  path: string;
  error?: string;
}
export interface Batch {
  removed?: string[];
  flows: Flow[];
  cursor: number;
  reset: boolean;
  total: number;
  bytes: number;
  dropped: number;
}

export interface AndroidState {
  enabled: boolean;
  serial: string;
  listening: boolean;
  address: string;
  pending: string[];
}
export interface AndroidDevice {
  serial: string;
  state: string;
  model: string;
}
