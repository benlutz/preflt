#!/bin/bash
set -e

BRANCH="main"
VERSION_FILE="internal/cmd/root.go"

DRY_RUN=false

usage() {
  echo "Usage: $0 -v major|minor|patch [-d]" >&2
  exit 1
}

# Parse args
while getopts "v:d" opt; do
  case $opt in
    v) BUMP="$OPTARG" ;;
    d) DRY_RUN=true ;;
    *) usage ;;
  esac
done

[[ "$BUMP" =~ ^(major|minor|patch)$ ]] || usage

# Must be on main
CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)
if [ "$CURRENT_BRANCH" != "$BRANCH" ]; then
  echo "Error: must be on '$BRANCH' (currently on '$CURRENT_BRANCH')" >&2
  exit 1
fi

# Must have a clean working tree
if ! git diff --quiet || ! git diff --cached --quiet; then
  echo "Error: working tree has uncommitted changes" >&2
  exit 1
fi

# Get current version from latest tag, fall back to root.go
LATEST_TAG=$(git tag --sort=-v:refname | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | head -1)
if [ -n "$LATEST_TAG" ]; then
  CURRENT="${LATEST_TAG#v}"
else
  CURRENT=$(grep 'var version' "$VERSION_FILE" | sed 's/.*"\(.*\)".*/\1/')
fi

# Parse semver
MAJOR=$(echo "$CURRENT" | cut -d. -f1)
MINOR=$(echo "$CURRENT" | cut -d. -f2)
PATCH=$(echo "$CURRENT" | cut -d. -f3)

case "$BUMP" in
  major) MAJOR=$((MAJOR + 1)); MINOR=0; PATCH=0 ;;
  minor) MINOR=$((MINOR + 1)); PATCH=0 ;;
  patch) PATCH=$((PATCH + 1)) ;;
esac

NEW_VERSION="${MAJOR}.${MINOR}.${PATCH}"
NEW_TAG="v${NEW_VERSION}"

if $DRY_RUN; then
  echo "[dry run] Would run: go test ./..."
  echo "[dry run] Would bump $CURRENT → $NEW_VERSION"
  echo "[dry run] Would update $VERSION_FILE"
  echo "[dry run] Would commit: chore: bump version to ${NEW_TAG}"
  echo "[dry run] Would tag: $NEW_TAG"
  echo "[dry run] Would push origin $BRANCH $NEW_TAG"
  exit 0
fi

# Run tests before touching anything
echo "Running tests..."
go test ./...

echo "Bumping $CURRENT → $NEW_VERSION"

# Update version in root.go
sed -i "s/var version = \".*\"/var version = \"${NEW_VERSION}\"/" "$VERSION_FILE"

# Commit and tag
git add "$VERSION_FILE"
git commit -m "chore: bump version to ${NEW_TAG}"
git tag "$NEW_TAG"

echo ""
echo "Created commit and tag $NEW_TAG."
read -r -p "Push to origin? [y/N] " confirm
if [[ "$confirm" =~ ^[Yy]$ ]]; then
  git push origin "$BRANCH"
  git push origin "$NEW_TAG"
  echo "Pushed. GitHub Actions will build and publish the release."
else
  echo "Not pushed. Run: git push origin $BRANCH && git push origin $NEW_TAG"
fi
