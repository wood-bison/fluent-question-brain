#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${repo_root}"

if command -v go >/dev/null 2>&1; then
  printf 'Go checks: host toolchain (%s)\n' "$(go version)"
  go test ./... "$@"
  exit 0
fi

if ! command -v docker >/dev/null 2>&1; then
  echo 'Go checks: no host Go toolchain and Docker is unavailable; refusing to skip go test ./...' >&2
  exit 1
fi

go_version="$(awk '$1 == "go" { print $2; exit }' go.mod)"
if [[ -z "${go_version}" ]]; then
  echo 'Go checks: go.mod does not declare a Go version' >&2
  exit 1
fi
image_version="${go_version%.*}"
image="golang:${image_version}-bookworm"
printf 'Go checks: host toolchain absent; running go test ./... in %s\n' "${image}"
docker run --rm \
  -e GOTOOLCHAIN=local \
  -v "${repo_root}:/src" \
  -w /src \
  "${image}" go test ./... "$@"
