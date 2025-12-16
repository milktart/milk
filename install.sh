#!/bin/bash

# --- Configuration ---
OWNER="milktart"
REPO="milk"
BINARY_NAME="milk"
INSTALL_PATH="/usr/local/bin" 
# ---------------------

echo "🚀 Starting installation of $BINARY_NAME..."

# --- 1. Dynamically Fetch the Latest Release Tag ---
echo "Fetching latest release tag from GitHub..."

# Fetch the full tag (e.g., v0.0.1)
RELEASE_TAG=$(
  curl -sS "https://api.github.com/repos/$OWNER/$REPO/releases/latest" \
  | grep '"tag_name":' \
  | awk -F '"' '{print $4}'
)

if [ -z "$RELEASE_TAG" ]; then
    echo "Error: Could not determine the latest release tag. Exiting."
    exit 1
fi

# --- FIX 1: Strip the 'v' from the tag for the asset name ---
# GoReleaser asset name uses '0.0.1', not 'v0.0.1'
ASSET_TAG_NAME="${RELEASE_TAG#v}"

echo "Latest release detected: $RELEASE_TAG"

# --- 2. Detect OS and Architecture (GoReleaser format) ---
OS=$(uname -s)
ARCH=$(uname -m)

case $ARCH in
  x86_64) GORELEASER_ARCH="amd64" ;; # GoReleaser uses 'amd64'
  arm64) GORELEASER_ARCH="arm64" ;; # GoReleaser uses 'arm64'
  *) echo "Unsupported architecture: $ARCH" && exit 1 ;;
esac

# --- FIX 2: Convert OS name to lowercase ---
# GoReleaser asset name uses 'darwin' and 'linux' (lowercase)
GORELEASER_OS=$(echo $OS | tr '[:upper:]' '[:lower:]')

# Final file name template: milk_0.0.1_darwin_arm64
DOWNLOAD_ASSET_NAME="${BINARY_NAME}_${ASSET_TAG_NAME}_${GORELEASER_OS}_${GORELEASER_ARCH}"

# The URL still uses the full tag name with 'v'
DOWNLOAD_URL="https://github.com/$OWNER/$REPO/releases/download/$RELEASE_TAG/$DOWNLOAD_ASSET_NAME"

echo "Detected OS/Arch: $GORELEASER_OS/$GORELEASER_ARCH"
echo "Attempting to download binary: $DOWNLOAD_ASSET_NAME"

# --- 3. Download the Binary ---
TEMP_FILE="/tmp/$BINARY_NAME-$ASSET_TAG_NAME"

if command -v curl >/dev/null 2>&1; then
  DOWNLOAD_CMD="curl -fsSL -o $TEMP_FILE $DOWNLOAD_URL"
elif command -v wget >/dev/null 2>&1; then
  DOWNLOAD_CMD="wget -qO $TEMP_FILE $DOWNLOAD_URL"
else
  echo "Error: Neither curl nor wget found. Please install one to continue."
  exit 1
fi

if ! $DOWNLOAD_CMD; then
    echo "Error: Failed to download binary from $DOWNLOAD_URL."
    echo "Check if the release tag '$RELEASE_TAG' and the asset '$DOWNLOAD_ASSET_NAME' exist."
    exit 1
fi

echo "Download complete. Installing to $INSTALL_PATH..."

# --- 4. Install the Binary ---
chmod +x $TEMP_FILE

# Use 'sudo' for /usr/local/bin
if ! sudo mv $TEMP_FILE $INSTALL_PATH/$BINARY_NAME; then
  echo "Error: Installation failed. You may need to run this script with elevated permissions."
  exit 1
fi

echo "✅ $BINARY_NAME installed successfully to $INSTALL_PATH/$BINARY_NAME."
echo "You can now run '$BINARY_NAME' from anywhere."
