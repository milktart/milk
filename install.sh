#!/bin/bash

# --- Configuration ---
OWNER="milktart"
REPO="milk"
BINARY_NAME="milk"
INSTALL_PATH="/usr/local/bin" # Common location for user-installed commands
RELEASE_TAG="v0.0.1"          # **MUST MATCH YOUR GITHUB RELEASE TAG**
# ---------------------

echo "🚀 Starting installation of $BINARY_NAME..."

# 1. Detect OS and Architecture (GoReleaser format)
OS=$(uname -s) # Note: We use the raw output here
ARCH=$(uname -m)

case $ARCH in
  x86_64) ARCH="x86_64" ;; # GoReleaser uses x86_64 for Linux/Windows AMD64
  arm64) ARCH="arm64" ;; 
  *) echo "Unsupported architecture: $ARCH" && exit 1 ;;
esac

# GoReleaser artifact naming convention:
# {{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}
# Example: milk_1.0.0_Linux_x86_64
FILENAME="$BINARY_NAME-$(echo $OS | sed 's/Darwin/darwin/g' | sed 's/Linux/linux/g' | sed 's/Windows/windows/g')-$ARCH" 
# ^--- This is still custom, let's use the full official GoReleaser template for simplicity:

# Let's use the GoReleaser official template naming for the URL construction
# $BINARY_NAME gets replaced with 'milk' by goreleaser
# The correct template for the asset name is: {{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}
# e.g., milk_v1.0.0_Linux_x86_64

# Extract the version tag from the download script itself (we don't have it here, 
# so we must assume a hardcoded tag or get the LATEST tag from the API)
# Getting the latest tag via API is more robust, but adds complexity.
# For simplicity, let's stick to a hardcoded tag for now, but always update the script:

# Construct the exact file name created by GoReleaser
GORELEASER_OS=$(uname -s | sed 's/Darwin/Darwin/g' | sed 's/Linux/Linux/g') # Note: Title case required by GoReleaser
GORELEASER_ARCH=$(uname -m | sed 's/x86_64/x86_64/g' | sed 's/arm64/arm64/g')

# Final file name template: milk_{{ .Tag }}_{{ .Os }}_{{ .Arch }}
DOWNLOAD_ASSET_NAME="${BINARY_NAME}_${RELEASE_TAG}_${GORELEASER_OS}_${GORELEASER_ARCH}"

DOWNLOAD_URL="https://github.com/$OWNER/$REPO/releases/download/$RELEASE_TAG/$DOWNLOAD_ASSET_NAME"

echo "Detected OS/Arch: $OS/$ARCH"
echo "Downloading binary from: $DOWNLOAD_URL"

# 2. Download the Binary
if command -v curl >/dev/null 2>&1; then
  DOWNLOAD_CMD="curl -sSL -o /tmp/$BINARY_NAME-$RELEASE_TAG $DOWNLOAD_URL"
elif command -v wget >/dev/null 2>&1; then
  DOWNLOAD_CMD="wget -qO /tmp/$BINARY_NAME-$RELEASE_TAG $DOWNLOAD_URL"
else
  echo "Error: Neither curl nor wget found. Please install one to continue."
  exit 1
fi

if ! $DOWNLOAD_CMD; then
    echo "Error: Failed to download binary. Check if the release tag '$RELEASE_TAG' and assets exist."
    exit 1
fi

echo "Download complete. Installing to $INSTALL_PATH..."

# 3. Install the Binary
chmod +x /tmp/$BINARY_NAME-$RELEASE_TAG

# Use 'sudo' for /usr/local/bin, as standard users often lack write permission
if ! sudo mv /tmp/$BINARY_NAME-$RELEASE_TAG $INSTALL_PATH/$BINARY_NAME; then
  echo "Error: Installation failed. You may need to run this script with elevated permissions."
  exit 1
fi

echo "✅ $BINARY_NAME installed successfully to $INSTALL_PATH/$BINARY_NAME."
echo "You can now run '$BINARY_NAME' from anywhere."
