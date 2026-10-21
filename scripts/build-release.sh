#!/bin/bash
# Run on macOS: ./scripts/build-release.sh
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUTPUT="$ROOT/build/releases"
STAGING=""

fail() { echo "错误：$*" >&2; exit 1; }
cleanup() { if [[ -n "$STAGING" ]]; then rm -rf "$STAGING"; fi; }
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
  echo "用法：./scripts/build-release.sh"
  echo "在 macOS 上生成 Windows x64、macOS Intel、macOS Apple Silicon ZIP。"
  echo "依赖：Go、Node.js 22.12+、Wails CLI 2.15、Xcode Command Line Tools。"
  echo "输出：build/releases/（版本号读取 wails.json）"
  exit 0
fi
[[ $# -eq 0 ]] || fail "不支持的参数；使用 --help 查看用法。"
[[ "$(uname -s)" == "Darwin" ]] || fail "请在 macOS 上执行，macOS 原生编译需要 Apple SDK。"
for tool in go node npm wails rsync xcrun ditto lipo unzip shasum; do
  command -v "$tool" >/dev/null 2>&1 || fail "缺少 $tool，请先安装对应依赖。"
done
node -e 'const [major, minor] = process.versions.node.split(".").map(Number); if (major < 22 || (major === 22 && minor < 12)) process.exit(1)' \
  || fail "需要 Node.js 22.12+；当前 $(node --version)。如使用 nvm，请先运行 nvm use 22。"
xcrun --find clang >/dev/null || fail "请先运行 xcode-select --install。"
xcrun --show-sdk-path >/dev/null || fail "未找到 macOS SDK。"

VERSION="$(node -e 'const c = require(process.argv[1]); const v = c.info.productVersion; if (typeof v !== "string" || !/^[0-9A-Za-z][0-9A-Za-z._-]*$/.test(v)) process.exit(1); process.stdout.write(v)' "$ROOT/wails.json")"
STAGING="$(mktemp -d "${TMPDIR:-/tmp}/mitmany-release.XXXXXX")"
PROJECT="$STAGING/project"
PACKAGES="$STAGING/packages"
mkdir -p "$PROJECT" "$PACKAGES"

echo "准备 mitmAny $VERSION 构建副本……"
# Isolate generated bindings, frontend installs and app bundles from the working app.
rsync -a \
  --exclude '.git/' --exclude '.DS_Store' --exclude 'node_modules/' \
  --exclude '/build/bin/' --exclude '/build/releases/' \
  --exclude '/frontend/dist/' --exclude '/frontend/wailsjs/' \
  --exclude '/output/' --exclude '/.playwright-cli/' \
  "$ROOT/" "$PROJECT/"
cd "$PROJECT"
# Use the lockfile for the single frontend build. Changes affect only the copy.
node -e 'const fs = require("fs"); const p = "wails.json"; const c = JSON.parse(fs.readFileSync(p, "utf8")); c["frontend:install"] = "npm ci"; fs.writeFileSync(p, JSON.stringify(c, null, 2))'

build_mac() {
  local arch="$1" label="$2" native_arch="$1"
  if [[ "$arch" == "amd64" ]]; then native_arch="x86_64"; fi
  shift 2
  echo "构建 macOS ${label}……"
  CGO_ENABLED=1 wails build -platform "darwin/$arch" -o mitmAny -trimpath "$@"
  local binary="$PROJECT/build/bin/mitmAny.app/Contents/MacOS/mitmAny"
  [[ "$(lipo -archs "$binary")" == "$native_arch" ]] || fail "macOS $label 架构校验失败。"
  ditto -c -k --sequesterRsrc --keepParent \
    "$PROJECT/build/bin/mitmAny.app" "$PACKAGES/mitmAny-$VERSION-macos-$arch.zip"
  # Avoid carrying an Intel executable into the next app bundle.
  rm -rf "$PROJECT/build/bin/mitmAny.app"
}

# Wails builds the frontend and generates bindings once, then reuses them.
build_mac amd64 Intel
build_mac arm64 'Apple Silicon' -s -skipbindings
echo "构建 Windows x64……"
CGO_ENABLED=0 wails build -platform windows/amd64 -o mitmAny.exe -trimpath -s -skipbindings
[[ -s "$PROJECT/build/bin/mitmAny.exe" ]] || fail "未生成 Windows 可执行文件。"
ditto -c -k --keepParent "$PROJECT/build/bin/mitmAny.exe" "$PACKAGES/mitmAny-$VERSION-windows-amd64.zip"

cd "$PACKAGES"
for archive in *.zip; do unzip -tq "$archive"; done
shasum -a 256 ./*.zip > SHA256SUMS.txt
# Publish only after all three archives have built and passed integrity checks.
mkdir -p "$OUTPUT"
for artifact in *.zip; do mv -f "$artifact" "$OUTPUT/$artifact"; done
mv -f SHA256SUMS.txt "$OUTPUT/SHA256SUMS-$VERSION.txt"
echo "完成，输出目录：$OUTPUT"
ls -lh "$OUTPUT/mitmAny-$VERSION-"*.zip "$OUTPUT/SHA256SUMS-$VERSION.txt"
