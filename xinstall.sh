#!/usr/bin/env bash
set -e

TOOL_NAME="milk"
INSTALL_DIR="$HOME/.local/bin"
REPO="milktart/$TOOL_NAME"   # Change to your GitHub repo
VERSION="v1.0.0"
BUILD_FROM_SOURCE=true     # Set to false to always use precompiled binaries

mkdir -p "$INSTALL_DIR"

# ------------------------
# 1. Install Go (if building from source)
# ------------------------
install_go() {
    if ! command -v go &>/dev/null; then
        echo "Go not found. Installing..."
        if [[ "$OSTYPE" == "darwin"* ]]; then
            if ! command -v brew &>/dev/null; then
                echo "Installing Homebrew..."
                /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
                eval "$(/opt/homebrew/bin/brew shellenv)"
            fi
            brew install go
        elif [[ "$OSTYPE" == "linux"* ]]; then
            ARCH=$(uname -m)
            [[ "$ARCH" == "x86_64" ]] && ARCH="amd64"
            [[ "$ARCH" == "aarch64" ]] && ARCH="arm64"
            wget "https://go.dev/dl/go1.25.4.linux-${ARCH}.tar.gz" -O /tmp/go.tar.gz
            if [ -w /usr/local ]; then
                rm -rf /usr/local/go
                tar -C /usr/local -xzf /tmp/go.tar.gz
            else
                echo "Installing Go to $HOME/.local/go instead (no write permission to /usr/local)..."
                mkdir -p "$HOME/.local"
                rm -rf "$HOME/.local/go"
                tar -C "$HOME/.local" -xzf /tmp/go.tar.gz
                export PATH=$PATH:$HOME/.local/go/bin
            fi
            rm /tmp/go.tar.gz
            export PATH=$PATH:/usr/local/go/bin
        else
            echo "Unsupported OS: $OSTYPE"
            exit 1
        fi
        echo "Go installed: $(go version)"
    else
        echo "Go is already installed: $(go version)"
    fi
}

# ------------------------
# 2. Add install directory to PATH permanently
# ------------------------
update_shell_path() {
    PATH_LINE="export PATH=\"\$PATH:$INSTALL_DIR\""
    UPDATED=false

    # Cover interactive and login shells
    SHELL_RC_FILES=("$HOME/.zshrc" "$HOME/.zprofile" "$HOME/.bashrc" "$HOME/.bash_profile")

    for rc in "${SHELL_RC_FILES[@]}"; do
        if [ -f "$rc" ] && [ -w "$rc" ]; then
            grep -qxF "$PATH_LINE" "$rc" || echo "$PATH_LINE" >> "$rc"
            UPDATED=true
            echo "â Added $INSTALL_DIR to PATH in $rc"
        elif [ ! -f "$rc" ]; then
            echo "$PATH_LINE" > "$rc"
            UPDATED=true
            echo "â Created $rc and added $INSTALL_DIR to PATH"
        fi
    done

    if [ "$UPDATED" = false ]; then
        echo "â ï¸ Could not automatically update PATH. Please add manually:"
        echo "    export PATH=\"\$PATH:$INSTALL_DIR\""
    fi

    # Apply immediately for the current shell
    export PATH="$PATH:$INSTALL_DIR"
}

# ------------------------
# 3. Build from Go source
# ------------------------
build_from_source() {
    if [ ! -f "main.go" ]; then
        echo "No main.go found, skipping source build."
        return
    fi

    if [ ! -f "go.mod" ]; then
        go mod init github.com/yourusername/$TOOL_NAME
    fi

    echo "Installing Go dependencies..."
    go mod tidy

    echo "Building $TOOL_NAME..."
    go build -o "$INSTALL_DIR/$TOOL_NAME"
    echo "â $TOOL_NAME built and installed to $INSTALL_DIR/$TOOL_NAME"
}

# ------------------------
# 4. Download precompiled binary (fallback)
# ------------------------
download_binary() {
    OS=$(uname | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)
    [[ "$ARCH" == "x86_64" ]] && ARCH="amd64"
    [[ "$ARCH" == "arm64" ]] && ARCH="arm64"

    BINARY_NAME="${TOOL_NAME}_${OS}_${ARCH}"
    URL="https://github.com/$REPO/releases/download/$VERSION/$BINARY_NAME"

    echo "Downloading precompiled binary from $URL..."
    curl -L -o "$INSTALL_DIR/$TOOL_NAME" "$URL"
    chmod +x "$INSTALL_DIR/$TOOL_NAME"
    echo "â $TOOL_NAME installed to $INSTALL_DIR/$TOOL_NAME"
}

# ------------------------
# Main installer flow
# ------------------------
echo "Starting installation of $TOOL_NAME..."
update_shell_path

if $BUILD_FROM_SOURCE; then
    install_go
    build_from_source
else
    download_binary
fi

echo "Installation complete. You can now run '$TOOL_NAME' from your terminal."
