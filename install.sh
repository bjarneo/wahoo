#!/bin/sh

set -eu

repository="${WAHOO_REPOSITORY:-bjarneo/wahoo}"
version="${WAHOO_VERSION:-}"
install_dir="${WAHOO_INSTALL_DIR:-$HOME/.local/bin}"

if [ -z "$version" ]; then
  printf '%s\n' "wahoo: set WAHOO_VERSION to an explicit release tag" >&2
  exit 1
fi

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

if ! command -v gpg >/dev/null 2>&1; then
  printf '%s\n' "wahoo: gpg is required to verify release signatures" >&2
  exit 1
fi

tmp_dir="$(mktemp -d)"
cleanup() {
  rm -rf "$tmp_dir"
}
trap cleanup EXIT HUP INT TERM

archive="wahoo_${os}_${arch}.tar.gz"
base_url="https://github.com/${repository}/releases/download/${version}"

curl --fail --location --silent --show-error "${base_url}/${archive}" --output "${tmp_dir}/${archive}"
curl --fail --location --silent --show-error "${base_url}/checksums.txt" --output "${tmp_dir}/checksums.txt"
curl --fail --location --silent --show-error "${base_url}/checksums.txt.asc" --output "${tmp_dir}/checksums.txt.asc"

gpg_home="${tmp_dir}/gnupg"
mkdir -m 700 "$gpg_home"
cat >"${tmp_dir}/release-signing-key.asc" <<'EOF'
-----BEGIN PGP PUBLIC KEY BLOCK-----

mQINBGp3ZzwBEAC3ukgVYfnnfbgz3hMXniHFRhjbq3LKfK5i+sJdcSJrh5W7P1mI
N2Ho8FzAFovPoRRb+VxB7CWZ6svA/hVJG17kQ8aPkeyvTF+HYion9XXLic/6izbv
iHg8mBH4zevmPOerxP3tIL2H7s8J92s3X2QHiXxIFgTpFA2aPJRBS0yTDT7xUhFj
XDkdotG2/Kwa7xIq6B7V8+Bia2NX/JayLqKC/1Ou2DvKwAaq6SMteefBnXEyK0VN
Ef/UMdwGc5cVEW+BF+GKz+V1jZR/7lhJA/pXJTeaGaTp7V760CQ0b/OfqqO7nE2A
MKU5Ew9wd8jb55AuqRW9k7G8XZLxCzirl/EFizwiCc4q8Zt8+MYPY53/8qzwEtix
Hmu3Xl8117wxlJhg6S4yD34otgl/zhnI+bHz9g6X6+bns4Gg48e4V5wsemoGCBIW
3E+l97pzFMs6e/4drFZOg7fWyc0KlvnYIWh1wEdS5yJMm0eFTsdI6oLBf2UjY7QX
oWEsWTH7k5rkQ3FAi5XRzxMl6TgffZiH/M6tQHOQD7gN1BR0JeZEQuGHJJC9b1AO
eUg0ePeuU8X3rgyRHvTz2XcOoE6Eeo33Egb7McwBb/UTlM9akNh0IRG2FuBK9s3f
qyFUg567XGYWUCH0S9ojQkeK3WlUUWV6P1j0ppIWNxfVMzF9I/bZ/YLCjQARAQAB
tDJXYWhvbyBSZWxlYXNlIFNpZ25pbmcgPHJlbGVhc2VzQGJqYXJuZW8uZ2l0aHVi
LmlvPokCVAQTAQoAPhYhBEYMlm65zf16rSzgGPHSUgDnt5oRBQJqd2c8AhsDBQkB
4TOABQsJCAcCBhUKCQgLAgQWAgMBAh4BAheAAAoJEPHSUgDnt5oRbXIQAIjvEJvF
0hP4iN8lyMtf0n6kmCLgj5lkqizrObFefWlVxVu6LFvfve07mW3eEg1AXTce1llx
YgVYvjvmt7Mr47ZtfZg7ANEdCqwfIjqAen5tT5Zq7IiB4XwGL8xL586cQsVNO2ZQ
rtYVhmuDw581ox7/IclO/oGxV3h3/8BN01Z8kTXmbQedHKH1GDiRTIYbaW3CQzCa
yVKMIeFWDztBXtQRhQDLwviMwWLxnAc8ZULwXOrJdivmOsUfwkX1EZPoHSFbA+mK
Ew5lA3e8WBxmNcpfTWsHsKeVJkMwyeZQBJb9gb6Qc1Dj5DPcbNddAxQxL3ACriU1
nZYl02fks5Zy+Av68XEkDxcWS5BzJt3ZbrEXWrz50qeURt2Z1ohLCDqTwT2aTtgh
k2ZNLubDZI3BmAmx7lYZ0hYbkpN3eJDrBI6APOvB7Gg40KEQG8g6MZWOpTlqTjpn
QozeEGh17MBMXinCOVTXQ1XT1DF0JXzJuqW+Htn1H6lqBhMxeBMbVPvkEy/UIX3v
GwojjV0ZnEF3duSJPF60CIlvv1HhWdxb9uKp9JMD7V1iiIkNw0lizTNLC9SfnceQ
8/sm2Gi5l7V0j6wlat4M9G3eqadBMj8phZ11tGkP4RqCoikNqBBsAR0UAFHhfGRT
Kqrvj3gdpGUGck4b6lnDINYizkqnTApq85qa
=t/IP
-----END PGP PUBLIC KEY BLOCK-----
EOF
gpg --homedir "$gpg_home" --batch --import "${tmp_dir}/release-signing-key.asc" >/dev/null 2>&1
gpg --homedir "$gpg_home" --batch --verify "${tmp_dir}/checksums.txt.asc" "${tmp_dir}/checksums.txt"

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
