#!/bin/sh

set -eu

repository="${WAHOO_REPOSITORY:-bjarneo/wahoo}"
version="${WAHOO_VERSION:-latest}"
install_dir="${WAHOO_INSTALL_DIR:-$HOME/.local/bin}"

case "$(uname -s)" in
  Linux) os="linux" ;;
  Darwin) os="darwin" ;;
  *)
    printf '%s\n' "wahoo: unsupported operating system: $(uname -s)" >&2
    exit 1
    ;;
esac

case "$(uname -m)" in
  x86_64 | amd64) arch="amd64" ;;
  arm64 | aarch64) arch="arm64" ;;
  *)
    printf '%s\n' "wahoo: unsupported architecture: $(uname -m)" >&2
    exit 1
    ;;
esac

if ! command -v curl >/dev/null 2>&1; then
  printf '%s\n' "wahoo: curl is required" >&2
  exit 1
fi

if ! command -v tar >/dev/null 2>&1; then
  printf '%s\n' "wahoo: tar is required" >&2
  exit 1
fi

tmp_dir="$(mktemp -d)"
cleanup() {
  rm -rf "$tmp_dir"
}
trap cleanup EXIT HUP INT TERM

archive="wahoo_${os}_${arch}.tar.gz"
base_url="https://github.com/${repository}/releases/${version}/download"

curl --fail --location --silent --show-error "${base_url}/${archive}" --output "${tmp_dir}/${archive}"
curl --fail --location --silent --show-error "${base_url}/checksums.txt" --output "${tmp_dir}/checksums.txt"

checksum="$(grep " ${archive}$" "${tmp_dir}/checksums.txt" || true)"
if [ -z "$checksum" ]; then
  printf '%s\n' "wahoo: checksum is missing for ${archive}" >&2
  exit 1
fi
printf '%s\n' "$checksum" >"${tmp_dir}/checksum.txt"

case "$os" in
  Linux)
    (cd "$tmp_dir" && sha256sum --check checksum.txt)
    ;;
  Darwin)
    (cd "$tmp_dir" && shasum -a 256 --check checksum.txt)
    ;;
esac

tar -xzf "${tmp_dir}/${archive}" -C "$tmp_dir"
mkdir -p "$install_dir"
install -m 0755 "${tmp_dir}/wahoo" "${install_dir}/wahoo"

printf '%s\n' "wahoo installed to ${install_dir}/wahoo"
