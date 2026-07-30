#!/usr/bin/env bash
# fetch.sh — Download Atomic Red Team atomics into the local atomics directory.
#
# Usage:
#   ./atomic/fetch.sh [output_dir] [repo_owner] [branch] [--clean]
#
# By default downloads the full upstream atomics tree from GitHub archive
# and merges into the target directory. With --clean, removes every
# top-level technique directory except those listed in SELECTED_TECHNIQUES.
#
# Alternative: set ART_USE_GIT=1 to use sparse git clone instead of zip download.
# This is slower but works if curl/unzip are unavailable.
#
# For local execution of atomics, see also:
#   https://github.com/lcensies/go-atomicredteam (GoART — standalone ART executor in Go)
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

default_output_dir() {
  if [ -d "${SCRIPT_DIR}/../../atomics" ]; then
    printf '%s\n' "${SCRIPT_DIR}/../../atomics"
    return
  fi
  if [ -d "${SCRIPT_DIR}/../atomics" ]; then
    printf '%s\n' "${SCRIPT_DIR}/../atomics"
    return
  fi
  printf '%s\n' "${SCRIPT_DIR}"
}

OUTPUT_DIR=""
REPO_OWNER="${ART_REPO_OWNER:-redcanaryco}"
BRANCH="${ART_REPO_BRANCH:-master}"
CLEAN=0
USE_GIT="${ART_USE_GIT:-0}"
POSITIONAL=()

for arg in "$@"; do
  case "${arg}" in
    --clean) CLEAN=1 ;;
    --git)   USE_GIT=1 ;;
    *)       POSITIONAL+=("${arg}") ;;
  esac
done

[ "${#POSITIONAL[@]}" -ge 1 ] && OUTPUT_DIR="${POSITIONAL[0]}"
[ "${#POSITIONAL[@]}" -ge 2 ] && REPO_OWNER="${POSITIONAL[1]}"
[ "${#POSITIONAL[@]}" -ge 3 ] && BRANCH="${POSITIONAL[2]}"
[ -z "${OUTPUT_DIR}" ] && OUTPUT_DIR="$(default_output_dir)"

ART_REPO="https://github.com/${REPO_OWNER}/atomic-red-team.git"
ARCHIVE_URL="https://github.com/${REPO_OWNER}/atomic-red-team/archive/${BRANCH}.zip"

# Techniques to fetch — one YAML per technique from the official ART repo
SELECTED_TECHNIQUES=(
  # Initial Access
  T1078
  T1190
  T1566.001
  # Execution
  T1059.001
  T1059.003
  T1059.004
  T1203
  # Persistence
  T1547.001
  T1543.003
  T1053.005
  # Privilege Escalation
  T1548.002
  T1055
  T1134
  # Defense Evasion
  T1027
  T1562.001
  T1070.001
  # Credential Access
  T1003.001
  T1003.002
  T1110.003
  T1550.002
  # Discovery
  T1087
  T1082
  T1083
  T1016
  T1049
  # Lateral Movement
  T1021.001
  T1021.002
  # Collection
  T1005
  T1074
  # Exfiltration
  T1041
  T1048
  # Impact
  T1486
  T1490
)

TMPDIR="$(mktemp -d)"
cleanup() { rm -rf "${TMPDIR}"; }
trap cleanup EXIT

clean_unselected() {
  local path base keep
  echo "[fetch] Cleaning ${OUTPUT_DIR} to keep only selected techniques..."
  for path in "${OUTPUT_DIR}"/T*; do
    [ -e "${path}" ] || continue
    base="$(basename "${path}")"
    keep=0
    for tech in "${SELECTED_TECHNIQUES[@]}"; do
      if [ "${base}" = "${tech}" ]; then
        keep=1
        break
      fi
    done
    if [ "${keep}" -eq 0 ]; then
      rm -rf "${path}"
      echo "  [rm]   ${base}"
    fi
  done
}

mkdir -p "${OUTPUT_DIR}"

if [ "${USE_GIT}" -eq 1 ]; then
  # ── Method 1: Sparse git clone (slower, no curl/unzip needed) ──────────────
  echo "[fetch] Cloning Atomic Red Team (sparse checkout)..."
  git clone --depth 1 --filter=blob:none --sparse "${ART_REPO}" "${TMPDIR}/art" 2>/dev/null
  pushd "${TMPDIR}/art" > /dev/null
  git sparse-checkout set atomics/ 2>/dev/null
  popd > /dev/null

  echo "[fetch] Copying ${#SELECTED_TECHNIQUES[@]} technique YAMLs to ${OUTPUT_DIR}..."
  copied=0
  skipped=0
  for tech in "${SELECTED_TECHNIQUES[@]}"; do
    src="${TMPDIR}/art/atomics/${tech}/${tech}.yaml"
    if [ -f "${src}" ]; then
      dst="${OUTPUT_DIR}/${tech}.yaml"
      if [ -f "${dst}" ]; then
        echo "  [skip] ${tech}.yaml (local version exists)"
        ((skipped++)) || true
      else
        cp "${src}" "${dst}"
        echo "  [ok]   ${tech}.yaml"
        ((copied++)) || true
      fi
    else
      echo "  [miss] ${tech}.yaml (not found in ART repo)"
    fi
  done
  echo ""
  echo "Done. Copied: ${copied}  Skipped (local exists): ${skipped}"

else
  # ── Method 2: Zip download (faster, recommended) ───────────────────────────
  ARCHIVE_PATH="${TMPDIR}/${BRANCH}.zip"
  EXTRACT_DIR="${TMPDIR}/extract"
  ART_ROOT="${EXTRACT_DIR}/atomic-red-team-${BRANCH}"

  echo "[fetch] Downloading atomics from ${REPO_OWNER}/atomic-red-team (${BRANCH})..."
  mkdir -p "${EXTRACT_DIR}"
  curl -fsSL "${ARCHIVE_URL}" -o "${ARCHIVE_PATH}"
  unzip -q "${ARCHIVE_PATH}" -d "${EXTRACT_DIR}"

  echo "[fetch] Merging upstream atomics into ${OUTPUT_DIR}..."
  cp -an "${ART_ROOT}/atomics/." "${OUTPUT_DIR}/"

  echo ""
  echo "Done. Atomics available in: ${OUTPUT_DIR}"
fi

if [ "${CLEAN}" -eq 1 ]; then
  clean_unselected
  echo "Clean mode: kept only ${#SELECTED_TECHNIQUES[@]} selected techniques"
fi

echo ""
echo "To run atomics locally using GoART (https://github.com/lcensies/go-atomicredteam):"
echo "  go install github.com/lcensies/go-atomicredteam/cmd/goart@latest"
echo "  goart --technique T1059.001 --index 0 --atomics-path ${OUTPUT_DIR}"
