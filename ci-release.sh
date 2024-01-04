#!/bin/bash
set -e

# Extract version from VERSION.md
VERSION=$(grep 'Version:' VERSION.md | awk '{print $2}')

# Check if version is extracted correctly
if [ -z "$VERSION" ]; then
    echo "Error: Failed to extract version from VERSION.md"
    exit 1
fi

# Git configurations
git config user.name "CI release"
git config user.email "ci-test@release.com"

# Ensure working on the latest main
git fetch upstream
git checkout upstream/main

# Tagging the release
git tag -a "v${VERSION}" -m "Release v${VERSION}"
git push upstream "v${VERSION}"
