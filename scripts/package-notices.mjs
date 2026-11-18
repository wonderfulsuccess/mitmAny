// Include browsable documentation and full notices alongside each binary.
import { cpSync, copyFileSync, mkdirSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
if (!process.argv[2]) throw new Error('Usage: node scripts/package-notices.mjs <package-dir> [license-bundle]');
const target = resolve(process.argv[2]);
mkdirSync(target,{recursive:true});
for (const file of ['LICENSE','THIRD_PARTY_NOTICES.md','README.md','README.en.md','README.zh-CN.md','README.zh-TW.md','README.ja.md','README.ko.md','CONTRIBUTING.md','SECURITY.md','CHANGELOG.md']) {
  copyFileSync(resolve(root,file),resolve(target,file));
}
cpSync(resolve(root,'docs'),resolve(target,'docs'),{recursive:true});
const vendor = 'third_party/go-mitmproxy';
mkdirSync(resolve(target,vendor),{recursive:true});
for (const file of ['LICENSE','PATCHES.md']) copyFileSync(resolve(root,vendor,file),resolve(target,vendor,file));
copyFileSync(resolve(process.argv[3] || resolve(root,'build/notices/THIRD_PARTY_LICENSES.txt')),resolve(target,'THIRD_PARTY_LICENSES.txt'));
