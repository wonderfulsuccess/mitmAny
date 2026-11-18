// Run after npm ci and go mod download. Includes build dependencies conservatively.
import { readFileSync, readdirSync, writeFileSync, mkdirSync, existsSync } from 'node:fs';
import { execFileSync } from 'node:child_process';
import { resolve, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const output = resolve(process.argv[2] || resolve(root, 'build/notices/THIRD_PARTY_LICENSES.txt'));
const chunks = ['mitmAny third-party license texts\nGenerated from installed locked dependencies. Includes build-time dependencies.\n'];
function collect(label, directory) {
  if (!directory || !existsSync(directory)) throw new Error(`Missing dependency directory: ${label}; download dependencies first`);
  const files = readdirSync(directory).filter(n => /^(licen[sc]e|copying|notice)(\.|$|-)/i.test(n));
  if (!files.length) throw new Error(`No license file for ${label}; review manually before distribution`);
  for (const name of files) chunks.push(`\n===== ${label} / ${name} =====\n${readFileSync(resolve(directory,name),'utf8')}`);
}
const used = new Set(execFileSync('go',['list','-deps','-f','{{if .Module}}{{.Module.Path}}{{end}}','./...'],{cwd:root,encoding:'utf8'}).trim().split(/\s+/));
const raw = execFileSync('go',['list','-m','-json','all'],{cwd:root,encoding:'utf8',maxBuffer:16*1024*1024});
for (const block of raw.trim().split(/\n(?=\{)/)) {
  const module = JSON.parse(block);
  if (module.Main || !used.has(module.Path)) continue;
  collect(`${module.Path} ${module.Version}`, (module.Replace || module).Dir);
}
const lock = JSON.parse(readFileSync(resolve(root,'frontend/package-lock.json'),'utf8'));
for (const [path,pkg] of Object.entries(lock.packages)) {
  if (!path || !existsSync(resolve(root,'frontend',path))) continue; // other-platform optional packages
  let directory = resolve(root,'frontend',path);
  // esbuild's platform binary package shares the same-version parent license.
  if (path.startsWith('node_modules/@esbuild/') || path.startsWith('node_modules/@rollup/rollup-')) {
    const parent = resolve(root, path.startsWith('node_modules/@esbuild/') ? 'frontend/node_modules/esbuild' : 'frontend/node_modules/rollup');
    const manifest = JSON.parse(readFileSync(resolve(parent,'package.json'),'utf8'));
    if (manifest.version !== pkg.version) throw new Error('platform package license version mismatch');
    directory = parent;
  }
  collect(`${path.replace(/^node_modules\//,'')} ${pkg.version} (${pkg.license || 'see text'})`, directory);
}
mkdirSync(dirname(output),{recursive:true});
writeFileSync(output,chunks.join('\n'));
console.log(`Wrote dependency license texts: ${output}`);
