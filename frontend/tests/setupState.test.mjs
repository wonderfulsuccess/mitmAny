import test from 'node:test';
import assert from 'node:assert/strict';
import { certificateHealth, shouldShowSetup } from '../src/setupState.ts';
const ready = { exists: true, valid: true, trusted: true, subject: 'Local CA' };
test('CA readiness requires a present, valid, trusted certificate', () => {
  assert.equal(certificateHealth(null), 'checking');
  assert.equal(certificateHealth(null, 'native check failed'), 'error');
  assert.equal(certificateHealth({exists:false, error:'initialization failed'}), 'error');
  assert.equal(certificateHealth({...ready, exists:false}), 'missing');
  assert.equal(certificateHealth({...ready, valid:false}), 'invalid');
  assert.equal(certificateHealth({...ready, trusted:false, error:'not trusted'}), 'untrusted');
  assert.equal(certificateHealth(ready), 'ready');
  assert.equal(certificateHealth(ready, 'refresh failed'), 'error');
});
test('first-use guidance waits for the check; returning users are alerted to problems', () => {
  for (const seen of [true, false]) assert.equal(shouldShowSetup(seen, 'checking'), false);
  assert.equal(shouldShowSetup(false, 'ready'), true);
  assert.equal(shouldShowSetup(true, 'ready'), false);
  for (const health of ['error', 'missing', 'invalid', 'untrusted']) {
    assert.equal(shouldShowSetup(true, health), true);
    assert.equal(shouldShowSetup(false, health), true);
  }
});
