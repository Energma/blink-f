#!/bin/sh
# Installs the latest blink release for this machine. Linux and macOS.
#   curl -fsSL https://raw.githubusercontent.com/Energma/blink-f/main/install.sh | sh
# Afterwards, `blink update` keeps it current.
set -eu

repo="Energma/blink-f"
bin_dir="${BLINK_INSTALL_DIR:-}"

os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$os" in
  linux|darwin) ;;
  *) echo "install.sh: unsupported OS $os (download a release by hand)" >&2; exit 1 ;;
esac
arch=$(uname -m)
case "$arch" in
  x86_64|amd64) arch=amd64 ;;
  arm64|aarch64) arch=arm64 ;;
  *) echo "install.sh: unsupported architecture $arch" >&2; exit 1 ;;
esac

tag=$(curl -fsSL "https://api.github.com/repos/$repo/releases/latest" \
  | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -n1)
[ -n "$tag" ] || { echo "install.sh: no release found for $repo" >&2; exit 1; }

name="blink-$tag-$os-$arch"
url="https://github.com/$repo/releases/download/$tag/$name.tar.gz"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

echo "downloading $name"
curl -fsSL -o "$tmp/$name.tar.gz" "$url"
curl -fsSL -o "$tmp/$name.tar.gz.sha256" "$url.sha256"
(cd "$tmp" && if command -v sha256sum >/dev/null; then sha256sum -c --quiet "$name.tar.gz.sha256"; else shasum -a 256 -c --quiet "$name.tar.gz.sha256"; fi)
tar xzf "$tmp/$name.tar.gz" -C "$tmp"

if [ -z "$bin_dir" ]; then
  if [ -w /usr/local/bin ]; then bin_dir=/usr/local/bin; else bin_dir="$HOME/.local/bin"; fi
fi
mkdir -p "$bin_dir"
install -m 755 "$tmp/$name/blink" "$bin_dir/blink"
echo "installed blink $tag to $bin_dir/blink"
case ":$PATH:" in
  *":$bin_dir:"*) ;;
  *) echo "note: add $bin_dir to your PATH" ;;
esac
