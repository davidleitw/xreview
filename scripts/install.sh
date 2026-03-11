#!/usr/bin/env bash
set -euo pipefail

REPO="davidleitw/xreview"
INSTALL_DIR="${HOME}/.local/bin"
BINARY_NAME="xreview"

# --- Detect OS ---
OS="$(uname -s)"
case "${OS}" in
  Linux*)  OS_NAME="linux" ;;
  Darwin*) OS_NAME="darwin" ;;
  *)
    echo "Error: Unsupported OS: ${OS}" >&2
    echo "Supported: Linux, macOS" >&2
    exit 1
    ;;
esac

# --- Detect Architecture ---
ARCH="$(uname -m)"
case "${ARCH}" in
  x86_64)  ARCH_NAME="amd64" ;;
  amd64)   ARCH_NAME="amd64" ;;
  aarch64) ARCH_NAME="arm64" ;;
  arm64)   ARCH_NAME="arm64" ;;
  *)
    echo "Error: Unsupported architecture: ${ARCH}" >&2
    echo "Supported: x86_64/amd64, aarch64/arm64" >&2
    exit 1
    ;;
esac

ASSET_NAME="${BINARY_NAME}-${OS_NAME}-${ARCH_NAME}"
DOWNLOAD_URL="https://github.com/${REPO}/releases/latest/download/${ASSET_NAME}"

echo "Detected: ${OS_NAME}/${ARCH_NAME}"
echo "Downloading ${BINARY_NAME} from GitHub Releases..."

# --- Create install directory ---
mkdir -p "${INSTALL_DIR}"

# --- Download ---
DOWNLOAD_OK=false
if command -v curl &>/dev/null; then
  HTTP_CODE=$(curl -fSL -o "${INSTALL_DIR}/${BINARY_NAME}" -w "%{http_code}" "${DOWNLOAD_URL}" 2>/dev/null) || true
  if [ "${HTTP_CODE}" = "200" ] && [ -f "${INSTALL_DIR}/${BINARY_NAME}" ] && [ -s "${INSTALL_DIR}/${BINARY_NAME}" ]; then
    DOWNLOAD_OK=true
  fi
elif command -v wget &>/dev/null; then
  if wget -q -O "${INSTALL_DIR}/${BINARY_NAME}" "${DOWNLOAD_URL}" 2>/dev/null; then
    if [ -f "${INSTALL_DIR}/${BINARY_NAME}" ] && [ -s "${INSTALL_DIR}/${BINARY_NAME}" ]; then
      DOWNLOAD_OK=true
    fi
  fi
fi

if [ "${DOWNLOAD_OK}" = true ]; then
  chmod +x "${INSTALL_DIR}/${BINARY_NAME}"
  echo "Installed ${BINARY_NAME} to ${INSTALL_DIR}/${BINARY_NAME}"
else
  # Clean up partial download
  rm -f "${INSTALL_DIR}/${BINARY_NAME}"
  echo "Download failed. Trying go install as fallback..." >&2

  if command -v go &>/dev/null; then
    go install "github.com/${REPO}/cmd/xreview@latest"
    echo "Installed via go install."
  else
    echo "Error: Could not download binary and Go is not installed." >&2
    echo "Please download manually from: https://github.com/${REPO}/releases" >&2
    exit 1
  fi
fi

# --- Verify PATH ---
case ":${PATH}:" in
  *":${INSTALL_DIR}:"*)
    ;;
  *)
    echo ""
    echo "Warning: ${INSTALL_DIR} is not in your PATH."
    echo "Add this to your shell profile (~/.bashrc or ~/.zshrc):"
    echo "  export PATH=\"${INSTALL_DIR}:\${PATH}\""
    ;;
esac

# --- Verify installation ---
if command -v "${BINARY_NAME}" &>/dev/null; then
  echo ""
  "${BINARY_NAME}" version
else
  echo ""
  echo "${BINARY_NAME} installed to ${INSTALL_DIR}/${BINARY_NAME}"
  echo "Run '${BINARY_NAME} version' after adding ${INSTALL_DIR} to PATH."
fi
