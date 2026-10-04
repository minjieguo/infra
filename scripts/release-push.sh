#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
用法:
  scripts/release-push.sh [patch|minor|major] [remote]

说明:
  基于当前最新的语义化版本 tag, 递增出下一个版本号并打 tag,
  随后推送当前分支与所有 tag。

  参数1 版本递增量 (可省略, 默认 patch):
    patch   修订号 +1, 适用于 Bug 修复、小幅调整
    minor   次版本号 +1 并清零修订号, 适用于新增功能(向后兼容)
    major   主版本号 +1 并清零次版本与修订, 适用于不兼容的变更

  参数2 远程仓库名 (可省略, 默认 origin)

  递增规则示例 (假设当前最新 tag 为 v0.0.30):
    patch   v0.0.30 -> v0.0.31
    minor   v0.0.30 -> v0.1.0
    major   v0.0.30 -> v1.0.0

    再假设当前最新 tag 为 v1.2.9:
    patch   v1.2.9  -> v1.2.10
    minor   v1.2.9  -> v1.3.0
    major   v1.2.9  -> v2.0.0

示例:
  scripts/release-push.sh
      等价于 patch + origin, 将 v0.0.30 递增为 v0.0.31

  scripts/release-push.sh patch
      只发修订版, 将 v0.0.30 递增为 v0.0.31

  scripts/release-push.sh minor
      只发次版本, 将 v0.0.30 递增为 v0.1.0

  scripts/release-push.sh major
      只发主版本, 将 v0.0.30 递增为 v1.0.0

  scripts/release-push.sh patch origin
      显式指定 patch 与远程仓库 origin

  scripts/release-push.sh minor upstream
      递增次版本, 并推送到远程仓库 upstream

注意:
  - 工作区必须干净(无未提交变更), 且需处于某个分支上。
  - 生成的 tag 若已存在, 脚本会报错退出。
  - 运行前会自动 fetch 远程 tag, 版本号以远程最新 tag 为准。
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
