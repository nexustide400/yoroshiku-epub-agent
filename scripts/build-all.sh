#!/usr/bin/env sh
set -eu

project_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
output_dir="$project_root/bin"
mkdir -p "$output_dir"

build() {
  target_os=$1
  target_arch=$2
  target_name=$3
  CGO_ENABLED=0 GOOS="$target_os" GOARCH="$target_arch" \
    go build -trimpath -ldflags "-s -w" -o "$output_dir/$target_name" ./cmd/book-builder
}

cd "$project_root"
build windows amd64 book-builder-windows-amd64.exe
build windows arm64 book-builder-windows-arm64.exe
build darwin amd64 book-builder-macos-amd64
build darwin arm64 book-builder-macos-arm64
build linux amd64 book-builder-linux-amd64
build linux arm64 book-builder-linux-arm64
