#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
mkdir -p dist
for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64; do
  os=${target%/*}; arch=${target#*/}; suffix=""
  if [ "$os" = windows ]; then suffix=.exe; fi
  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -trimpath -ldflags='-s -w -buildid=' -o "dist/log-reviewer-$os-$arch$suffix" ./cmd/log-reviewer
  bytes=$(wc -c < "dist/log-reviewer-$os-$arch$suffix")
  if [ "$bytes" -gt 10485760 ]; then echo "Binary exceeds 10 MiB: $os/$arch" >&2; exit 1; fi
done
cd dist
if command -v sha256sum >/dev/null 2>&1; then
  sha256sum log-reviewer-darwin-amd64 log-reviewer-darwin-arm64 log-reviewer-linux-amd64 log-reviewer-linux-arm64 log-reviewer-windows-amd64.exe log-reviewer-windows-arm64.exe > SHA256SUMS.txt
else
  shasum -a 256 log-reviewer-darwin-amd64 log-reviewer-darwin-arm64 log-reviewer-linux-amd64 log-reviewer-linux-arm64 log-reviewer-windows-amd64.exe log-reviewer-windows-arm64.exe > SHA256SUMS.txt
fi
