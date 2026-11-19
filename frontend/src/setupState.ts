import type { Certificate } from './types.ts';

export type CertificateHealth = 'checking' | 'error' | 'missing' | 'invalid' | 'untrusted' | 'ready';
export function certificateHealth(c: Certificate | null, error = ''): CertificateHealth {
  if (error || (c?.error && !c.subject)) return 'error';
  if (!c) return 'checking';
  if (!c.exists) return 'missing';
  if (!c.valid) return 'invalid';
  return c.trusted ? 'ready' : 'untrusted';
}
export const certificateLabels: Record<CertificateHealth, string> = {
  checking: 'Checking CA certificate…', error: 'CA check failed', missing: 'CA certificate missing',
  invalid: 'CA certificate invalid', untrusted: 'CA not trusted · Install', ready: 'CA ready',
};
export const SETUP_KEY = 'mitmAny:capture-setup:v1';
export function shouldShowSetup(seen: boolean, health: CertificateHealth): boolean {
  return health !== 'checking' && (!seen || health !== 'ready');
}
