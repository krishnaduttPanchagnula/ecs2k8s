#!/usr/bin/env bash
# Script to generate WinGet manifests for a new release
# This script creates the YAML manifests needed for WinGet package submission

set -e

GITHUB_REPO="krishnaduttPanchagnula/ecs2k8s"
VERSION=${1:-$(curl -s "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" | grep '"tag_name"' | head -1 | cut -d'"' -f4 | sed 's/v//')}

if [ -z "$VERSION" ]; then
    echo "Error: Could not determine version"
    exit 1
fi

echo "Generating WinGet manifests for version $VERSION..."

# Create output directory
OUTPUT_DIR="winget-manifests/${VERSION}"
mkdir -p "$OUTPUT_DIR"

# Download checksums from the release
CHECKSUMS_FILE=$(mktemp)
curl -sL "https://github.com/${GITHUB_REPO}/releases/download/v${VERSION}/checksums.txt" -o "$CHECKSUMS_FILE"

# Extract SHA256 hash for Windows AMD64
SHA256_WINDOWS_AMD64=$(grep "ecs2k8s_${VERSION}_windows_amd64" "$CHECKSUMS_FILE" | awk '{print $1}')

# Get release date (current date or from release info)
RELEASE_DATE=$(date -u +'%Y-%m-%d')

# Get release notes
RELEASE_NOTES=$(curl -s "https://api.github.com/repos/${GITHUB_REPO}/releases/tags/v${VERSION}" | jq -r '.body')

rm "$CHECKSUMS_FILE"

# Create installer manifest
cat > "$OUTPUT_DIR/KrishnaDuttPanchagnula.ecs2k8s.installer.yaml" << 'MANIFEST'
# yaml-language-server: $schema=https://aka.ms/winget-manifest.installer.1.6.0.schema.json
PackageIdentifier: KrishnaDuttPanchagnula.ecs2k8s
PackageVersion: VERSION_PLACEHOLDER
MinimumOSVersion: 10.0.0.0
InstallModes:
  - silent
  - silentWithProgress
Installers:
  - Architecture: x64
    InstallerType: portable
    InstallerUrl: https://github.com/krishnaduttPanchagnula/ecs2k8s/releases/download/vVERSION_PLACEHOLDER/ecs2k8s_VERSION_PLACEHOLDER_windows_amd64.zip
    InstallerSha256: SHA256_PLACEHOLDER
    ReleaseDate: RELEASE_DATE_PLACEHOLDER
ManifestType: installer
ManifestVersion: 1.6.0
MANIFEST

# Replace placeholders in installer manifest
sed -i "s/VERSION_PLACEHOLDER/${VERSION}/g" "$OUTPUT_DIR/KrishnaDuttPanchagnula.ecs2k8s.installer.yaml"
sed -i "s/SHA256_PLACEHOLDER/${SHA256_WINDOWS_AMD64}/g" "$OUTPUT_DIR/KrishnaDuttPanchagnula.ecs2k8s.installer.yaml"
sed -i "s/RELEASE_DATE_PLACEHOLDER/${RELEASE_DATE}/g" "$OUTPUT_DIR/KrishnaDuttPanchagnula.ecs2k8s.installer.yaml"

# Create locale manifest
cat > "$OUTPUT_DIR/KrishnaDuttPanchagnula.ecs2k8s.locale.en-US.yaml" << 'MANIFEST'
# yaml-language-server: $schema=https://aka.ms/winget-manifest.defaultLocale.1.6.0.schema.json
PackageIdentifier: KrishnaDuttPanchagnula.ecs2k8s
PackageVersion: VERSION_PLACEHOLDER
PackageLocale: en-US
Publisher: Krishna Dutt Panchagnula
PublisherUrl: https://github.com/krishnaduttPanchagnula
PublisherSupportUrl: https://github.com/krishnaduttPanchagnula/ecs2k8s/issues
Author: Krishna Dutt Panchagnula
AuthorUrl: https://github.com/krishnaduttPanchagnula
PackageName: ecs2k8s
PackageUrl: https://github.com/krishnaduttPanchagnula/ecs2k8s
License: Apache-2.0
LicenseUrl: https://github.com/krishnaduttPanchagnula/ecs2k8s/blob/main/LICENSE
Copyright: Copyright (c) Krishna Dutt Panchagnula
ShortDescription: AWS ECS to Kubernetes migration tool
Description: ecs2k8s is a command-line tool that automates the conversion of AWS ECS task definitions into Kubernetes manifests, enabling seamless migration of containerized applications from ECS to Kubernetes.
Tags:
  - kubernetes
  - aws
  - ecs
  - migration
  - container
  - k8s
  - docker
ReleaseNotes: |
  RELEASE_NOTES_PLACEHOLDER
ReleaseNotesUrl: https://github.com/krishnaduttPanchagnula/ecs2k8s/releases/tag/vVERSION_PLACEHOLDER
ManifestType: defaultLocale
ManifestVersion: 1.6.0
MANIFEST

# Replace placeholders in locale manifest
sed -i "s/VERSION_PLACEHOLDER/${VERSION}/g" "$OUTPUT_DIR/KrishnaDuttPanchagnula.ecs2k8s.locale.en-US.yaml"
sed -i "s|RELEASE_NOTES_PLACEHOLDER|${RELEASE_NOTES}|g" "$OUTPUT_DIR/KrishnaDuttPanchagnula.ecs2k8s.locale.en-US.yaml"

# Create version manifest
cat > "$OUTPUT_DIR/KrishnaDuttPanchagnula.ecs2k8s.yaml" << 'MANIFEST'
# yaml-language-server: $schema=https://aka.ms/winget-manifest.version.1.6.0.schema.json
PackageIdentifier: KrishnaDuttPanchagnula.ecs2k8s
PackageVersion: VERSION_PLACEHOLDER
DefaultLocale: en-US
ManifestType: version
ManifestVersion: 1.6.0
MANIFEST

# Replace placeholders in version manifest
sed -i "s/VERSION_PLACEHOLDER/${VERSION}/g" "$OUTPUT_DIR/KrishnaDuttPanchagnula.ecs2k8s.yaml"

echo "✓ WinGet manifests generated successfully!"
echo "  Location: $OUTPUT_DIR"
echo "  Version: $VERSION"
echo "  Release Date: $RELEASE_DATE"
echo ""
echo "Next steps:"
echo "1. Copy these manifests to microsoft/winget-pkgs repository"
echo "2. Path: manifests/k/KrishnaDuttPanchagnula/ecs2k8s/${VERSION}/"
echo "3. Submit a pull request to microsoft/winget-pkgs"
