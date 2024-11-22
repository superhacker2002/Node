#!/bin/bash

set -euo pipefail
# allow specifying different destination directory
DIR="${DIR:-"/usr/local/bin"}"

RELEASE="${1:-"latest"}"

TAG=$(curl -L -s -H 'Accept: application/json' https://github.com/denetpro/node/releases/$RELEASE | sed -e 's/.*"tag_name":"\([^"]*\)".*/\1/')

ARCH=$(uname -m)
case "$ARCH" in
    x86_64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) error "Unsupported architecture: $ARCH" ;;
esac


GITHUB_FILE=$(echo "denode-$(uname -s)-$ARCH.tar.gz" | tr '[:upper:]' '[:lower:]')
GITHUB_URL="https://github.com/denetpro/node/releases/download/${TAG}/${GITHUB_FILE}"

echo "Downloading $GITHUB_FILE from $GITHUB_URL..."
curl -L -o denode.tar.gz "$GITHUB_URL" --fail || { echo "Failed to download $GITHUB_FILE"; exit 1; }

echo "Extracting $GITHUB_FILE..."
tar -xzf denode.tar.gz -C "$DIR" || { echo "Failed to extract $GITHUB_FILE"; rm -f denode.tar.gz; exit 1; }

rm -f denode.tar.gz

if [ -x "$DIR/denode" ]; then
  echo "$(denode -v) successfully installed in $DIR"
else
  echo "Failed to install denode"; exit 1
fi
