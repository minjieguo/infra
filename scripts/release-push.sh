#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage:
  scripts/release-push.sh [patch|minor|major] [remote]

Examples:
  scripts/release-push.sh
  scripts/release-push.sh minor
  scripts/release-push.sh patch origin

Creates the next semantic version tag and pushes the current branch plus tags.
Default bump: patch
Default remote: origin
USAGE
}

bump="${1:-patch}"
remote="${2:-origin}"

if [[ "${bump}" == "-h" || "${bump}" == "--help" ]]; then
  usage
  exit 0
fi

case "${bump}" in
  patch|minor|major) ;;
  *)
    echo "Unsupported bump '${bump}'. Use patch, minor, or major." >&2
    exit 1
    ;;
esac

if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  echo "This script must be run inside a git repository." >&2
  exit 1
fi

branch="$(git branch --show-current)"
if [[ -z "${branch}" ]]; then
  echo "You are not on a branch. Checkout a branch before releasing." >&2
  exit 1
fi

if [[ -n "$(git status --porcelain)" ]]; then
  echo "Working tree is not clean. Commit or stash your changes before releasing." >&2
  git status --short
  exit 1
fi

git fetch --tags "${remote}" >/dev/null 2>&1 || true

latest_tag="$(git tag --list 'v[0-9]*.[0-9]*.[0-9]*' --sort=-v:refname | head -n 1)"
if [[ -z "${latest_tag}" ]]; then
  major=0
  minor=0
  patch=0
else
  version="${latest_tag#v}"
  IFS='.' read -r major minor patch <<<"${version}"
fi

case "${bump}" in
  major)
    major=$((major + 1))
    minor=0
    patch=0
    ;;
  minor)
    minor=$((minor + 1))
    patch=0
    ;;
  patch)
    patch=$((patch + 1))
    ;;
esac

next_tag="v${major}.${minor}.${patch}"

if git rev-parse "${next_tag}" >/dev/null 2>&1; then
  echo "Tag ${next_tag} already exists." >&2
  exit 1
fi

echo "Creating ${next_tag} from $(git rev-parse --short HEAD)"
git tag -a "${next_tag}" -m "Release ${next_tag}"

echo "Pushing ${branch} and ${next_tag} to ${remote}"
git push "${remote}" "${branch}"
git push "${remote}" "${next_tag}"

echo "Released ${next_tag}"
