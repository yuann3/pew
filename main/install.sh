#!/bin/bash
set -e

VERSION="0.1.0"
BINARY_NAME="pew"
INSTALL_DIR="/usr/local/bin"
GITHUB_USER="yuann3"
GITHUB_REPO="pew"

# Determine OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

if [ "$ARCH" = "x86_64" ]; then
  ARCH="amd64"
elif [ "$ARCH" = "aarch64" ] || [ "$ARCH" = "arm64" ]; then
  ARCH="arm64"
else
  echo "Unsupported architecture: $ARCH"
  exit 1
fi

# Download URL
DOWNLOAD_URL="https://github.com/${GITHUB_USER}/${GITHUB_REPO}/releases/download/v${VERSION}/${BINARY_NAME}_${VERSION}_${OS}_${ARCH}.tar.gz"

echo "Downloading ${BINARY_NAME} ${VERSION} for ${OS}/${ARCH}..."
curl -L -o /tmp/${BINARY_NAME}.tar.gz ${DOWNLOAD_URL}

echo "Installing to ${INSTALL_DIR}..."
tar -xzf /tmp/${BINARY_NAME}.tar.gz -C /tmp
sudo mv /tmp/${BINARY_NAME}_${VERSION}_${OS}_${ARCH} ${INSTALL_DIR}/${BINARY_NAME}
sudo chmod +x ${INSTALL_DIR}/${BINARY_NAME}

rm /tmp/${BINARY_NAME}.tar.gz

echo "${BINARY_NAME} ${VERSION} installed successfully!"
