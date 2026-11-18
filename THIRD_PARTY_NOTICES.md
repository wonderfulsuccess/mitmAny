# Third-party notices

mitmAny's own code is MIT licensed. Dependencies retain their original licenses; the root license does not replace them.

| Component | Source / license |
| --- | --- |
| go-mitmproxy v1.9.4, locally patched | [upstream](https://github.com/lqqyt2423/go-mitmproxy), MIT; [license](third_party/go-mitmproxy/LICENSE), [patches](third_party/go-mitmproxy/PATCHES.md) |
| Wails | [upstream](https://github.com/wailsapp/wails), MIT |
| React / React DOM | [upstream](https://github.com/facebook/react), MIT |
| TanStack Virtual | [upstream](https://github.com/TanStack/virtual), MIT |
| Lucide React icons | [upstream](https://github.com/lucide-icons/lucide), ISC and retained upstream notices |
| Other Go / npm dependencies | Exact versions: `go.mod`, `go.sum`, `frontend/package-lock.json`; license texts included in release packages |

Run `go mod download`, `npm --prefix frontend ci`, then `node scripts/collect-licenses.mjs` to collect installed dependency license and notice texts. This includes build-time dependencies conservatively and fails when an installed dependency has no license file (esbuild/Rollup platform binaries use their same-version parent license). Review the resulting file when updating dependencies. Platform-specific optional npm dependencies are collected by the corresponding native CI runner.

## Repository assets

- `frontend/public/favicon.svg`: project mark as SVG source, present in the initial repository commit.
- `build/appicon.png`, `build/windows/icon.ico`: existing project application icons, present in the initial commit. On 2026-11-18, the maintainer confirmed having permission to redistribute these assets publicly. This records redistribution permission, not a claim of third-party authorship or exclusive ownership.
- `docs/assets/workspace.png` and demo frames: screenshots of this project's UI using the committed synthetic fixture; no real account/device/traffic data.
- UI uses system fonts. No proprietary font file is bundled by this project.

Preserve upstream copyright, license and NOTICE text in redistributed source and binaries. License text collection is an inventory aid, not proof of compatibility or asset ownership.
