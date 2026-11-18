import { readFileSync, existsSync } from 'node:fs';
import assert from 'node:assert/strict';
const read = path => readFileSync(new URL(`../${path}`, import.meta.url), 'utf8');
const wails = JSON.parse(read('wails.json'));
const frontend = JSON.parse(read('frontend/package.json'));
const lock = JSON.parse(read('frontend/package-lock.json'));
const version = wails.info.productVersion;
assert.match(version, /^\d+\.\d+\.\d+$/);
assert.equal(frontend.version, version, 'frontend version differs');
assert.equal(lock.version, version, 'lockfile version differs');
assert.equal(lock.packages[''].version, version, 'lockfile root version differs');
assert.ok(read('internal/version/version.go').includes(`const Version = "${version}"`), 'Go version differs');
assert.equal(wails['frontend:install'], 'npm ci');
if (process.env.GITHUB_REF_TYPE === 'tag') {
  assert.equal(process.env.GITHUB_REF_NAME, `v${version}`, 'release tag must match product version');
}
for (const path of ['LICENSE','SECURITY.md','CONTRIBUTING.md','THIRD_PARTY_NOTICES.md','README.en.md','README.zh-CN.md','docs/assets/workspace.png']) {
  assert.ok(existsSync(new URL(`../${path}`,import.meta.url)), `missing ${path}`);
}
console.log(`Release metadata consistent: ${version}`);
