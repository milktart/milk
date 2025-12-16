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

# Use curl to get the latest release data. 
# We look for the 'tag_name' field in the JSON response.
# We use 'grep' and 'awk' to extract the tag name without needing 'jq'.
RELEASE_TAG=$(
  curl -sS "https://api.github.com/repos/$OWNER/$REPO/releases/latest" \
  | grep '"tag_name":' \
  | awk -F '"' '{print $4}'
)

if [ -z "$RELEASE_TAG" ]; then
    echo "Error: Could not determine the latest release tag. Exiting."
    exit 1
fi

echo "Latest release detected: $RELEASE_TAG"

# --- 2. Detect OS and Architecture (GoReleaser format) ---
OS=$(uname -s)
ARCH=$(uname -m)

case $ARCH in
  x86_64) ARCH="x86_64" ;; 
  arm64) ARCH="arm64" ;; 
  *) echo "Unsupported architecture: $ARCH" && exit 1 ;;
esac

# GoReleaser artifacts are named: {{ .ProjectName }}_{{ .Tag }}_{{ .Os }}_{{ .Arch }}
# e.g., milk_v1.0.0_Linux_x86_64

# Capitalize OS names as GoReleaser does (Linux, Darwin)
GORELEASER_OS=$(echo $OS | sed 's/Darwin/Darwin/g' | sed 's/Linux/Linux/g')

# Final file name template: milk_{{ .Tag }}_{{ .Os }}_{{ .Arch }}
DOWNLOAD_ASSET_NAME="${BINARY_NAME}_${RELEASE_TAG}_${GORELEASER_OS}_${ARCH}"

DOWNLOAD_URL="https://github.com/$OWNER/$REPO/releases/download/$RELEASE_TAG/$DOWNLOAD_ASSET_NAME"

echo "Detected OS/Arch: $GORELEASER_OS/$ARCH"
echo "Attempting to download binary: $DOWNLOAD_ASSET_NAME"

# --- 3. Download the Binary ---
TEMP_FILE="/tmp/$BINARY_NAME-$RELEASE_TAG"

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
