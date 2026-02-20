#!/usr/bin/env bash
# Script to update the Homebrew tap formula with new release information
# This script should be run in the homebrew-ecs2k8s tap repository

set -e

GITHUB_REPO="krishnaduttPanchagnula/ecs2k8s"
FORMULA_PATH="Formula/ecs2k8s.rb"

# Get version from input or latest release
VERSION=${1:-$(curl -s "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" | grep '"tag_name"' | head -1 | cut -d'"' -f4 | sed 's/v//')}

if [ -z "$VERSION" ]; then
    echo "Error: Could not determine version"
    exit 1
fi

echo "Updating ecs2k8s formula to version $VERSION..."

# Download checksums from the release
CHECKSUMS_FILE=$(mktemp)
curl -sL "https://github.com/${GITHUB_REPO}/releases/download/v${VERSION}/checksums.txt" -o "$CHECKSUMS_FILE"

# Extract SHA256 hashes for each architecture
SHA256_DARWIN_AMD64=$(grep "ecs2k8s_${VERSION}_darwin_amd64" "$CHECKSUMS_FILE" | awk '{print $1}')
SHA256_DARWIN_ARM64=$(grep "ecs2k8s_${VERSION}_darwin_arm64" "$CHECKSUMS_FILE" | awk '{print $1}')
SHA256_LINUX_AMD64=$(grep "ecs2k8s_${VERSION}_linux_amd64" "$CHECKSUMS_FILE" | awk '{print $1}')
SHA256_LINUX_ARM64=$(grep "ecs2k8s_${VERSION}_linux_arm64" "$CHECKSUMS_FILE" | awk '{print $1}')

rm "$CHECKSUMS_FILE"

# Create new formula file with updated values
cat > "$FORMULA_PATH" << EOF
class Ecs2k8s < Formula
  desc "AWS ECS to Kubernetes migration tool"
  homepage "https://github.com/krishnaduttPanchagnula/ecs2k8s"
  license "Apache-2.0"

  version "${VERSION}"

  # Disable binary detection for this formula
  pour_bottle? false if OS.linux?

  on_macos do
    on_arm64 do
      url "https://github.com/krishnaduttPanchagnula/ecs2k8s/releases/download/v${VERSION}/ecs2k8s_${VERSION}_darwin_arm64.tar.gz"
      sha256 "${SHA256_DARWIN_ARM64}"
    end
    on_intel do
      url "https://github.com/krishnaduttPanchagnula/ecs2k8s/releases/download/v${VERSION}/ecs2k8s_${VERSION}_darwin_amd64.tar.gz"
      sha256 "${SHA256_DARWIN_AMD64}"
    end
  end

  on_linux do
    on_intel do
      url "https://github.com/krishnaduttPanchagnula/ecs2k8s/releases/download/v${VERSION}/ecs2k8s_${VERSION}_linux_amd64.tar.gz"
      sha256 "${SHA256_LINUX_AMD64}"
    end
    on_arm do
      url "https://github.com/krishnaduttPanchagnula/ecs2k8s/releases/download/v${VERSION}/ecs2k8s_${VERSION}_linux_arm64.tar.gz"
      sha256 "${SHA256_LINUX_ARM64}"
    end
  end

  def install
    bin.install "ecs2k8s"
  end

  def post_install
    puts "ecs2k8s has been installed successfully!"
    puts "Run 'ecs2k8s --help' to get started"
  end

  test do
    system "#{bin}/ecs2k8s", "--help"
    assert_match "AWS ECS to Kubernetes", shell_output("#{bin}/ecs2k8s --help")
  end
end
EOF

echo "✓ Formula updated successfully!"
echo "  Version: $VERSION"
echo "  Darwin ARM64: ${SHA256_DARWIN_ARM64:0:16}..."
echo "  Darwin AMD64: ${SHA256_DARWIN_AMD64:0:16}..."
echo "  Linux AMD64:  ${SHA256_LINUX_AMD64:0:16}..."
echo "  Linux ARM64:  ${SHA256_LINUX_ARM64:0:16}..."
