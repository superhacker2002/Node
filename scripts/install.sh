#!/bin/bash

set -euo pipefail
# allow specifying different destination directory
DIR="${DIR:-"$HOME/.local/bin"}"

RELEASE="${1:-"latest"}"

# prepare the download URL
TAG=$(curl -L -s -H 'Accept: application/json' https://github.com/denetpro/node/releases/$RELEASE | sed -e 's/.*"tag_name":"\([^"]*\)".*/\1/')

ARCH=$(uname -m)
case "$ARCH" in
    x86_64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) error "Unsupported architecture: $ARCH" ;;
esac


GITHUB_FILE=$(echo "denode-$(uname -s)-$ARCH" | tr '[:upper:]' '[:lower:]')
GITHUB_URL="https://github.com/denetpro/node/releases/download/${TAG}/${GITHUB_FILE}"

# install/update the local binary
curl -L -o denode $GITHUB_URL
install -m 555 denode -t "$DIR"
rm -f denode
echo "`denode -v` successfully installed"
