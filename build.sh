#!/bin/bash
set -e

VERSION="0.1.0"
BINARY_NAME="pew"
BUILD_DIR="./build"

mkdir -p $BUILD_DIR

echo "Building for macOS (Intel)..."
GOOS=darwin GOARCH=amd64 go build -o $BUILD_DIR/${BINARY_NAME}_${VERSION}_darwin_amd64
echo "Building for macOS (Apple Silicon)..."
GOOS=darwin GOARCH=arm64 go build -o $BUILD_DIR/${BINARY_NAME}_${VERSION}_darwin_arm64

echo "Building for Linux (amd64)..."
GOOS=linux GOARCH=amd64 go build -o $BUILD_DIR/${BINARY_NAME}_${VERSION}_linux_amd64
echo "Building for Linux (arm64)..."
GOOS=linux GOARCH=arm64 go build -o $BUILD_DIR/${BINARY_NAME}_${VERSION}_linux_arm64

echo "Creating archives..."
cd $BUILD_DIR
tar -czf ${BINARY_NAME}_${VERSION}_darwin_amd64.tar.gz ${BINARY_NAME}_${VERSION}_darwin_amd64
tar -czf ${BINARY_NAME}_${VERSION}_darwin_arm64.tar.gz ${BINARY_NAME}_${VERSION}_darwin_arm64
tar -czf ${BINARY_NAME}_${VERSION}_linux_amd64.tar.gz ${BINARY_NAME}_${VERSION}_linux_amd64
tar -czf ${BINARY_NAME}_${VERSION}_linux_arm64.tar.gz ${BINARY_NAME}_${VERSION}_linux_arm64

echo "Build complete!"
