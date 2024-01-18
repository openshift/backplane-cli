#!/bin/bash
set -e

# Extract version from VERSION.md
VERSION=$(grep 'Version:' VERSION.md | awk '{print $2}')

# Check if version is extracted correctly
if [ -z "$VERSION" ]; then
    echo "Error: Failed to extract version from VERSION.md"
    exit 1
fi

# Check if the tag already exists in the repository
if git rev-parse "v$VERSION" >/dev/null 2>&1; then
    echo "Error: Tag v$VERSION already exists. Aborting release."
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
