#!/usr/bin/env bash
# fetch.sh — Download Atomic Red Team atomics into the local atomics directory.
#
# Usage:
#   ./atomic/fetch.sh [output_dir] [repo_owner] [branch] [--clean]
#
# By default the script downloads the full upstream atomics tree from the GitHub
# archive and merges it into the target directory. With --clean, it then removes
# every top-level technique directory except those listed in SELECTED_TECHNIQUES.
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

POSITIONAL=()
for arg in "$@"; do
  case "${arg}" in
    --clean)
      CLEAN=1
      ;;
    *)
      POSITIONAL+=("${arg}")
      ;;
  esac
done

if [ "${#POSITIONAL[@]}" -ge 1 ]; then
  OUTPUT_DIR="${POSITIONAL[0]}"
else
  OUTPUT_DIR="$(default_output_dir)"
fi
if [ "${#POSITIONAL[@]}" -ge 2 ]; then
  REPO_OWNER="${POSITIONAL[1]}"
fi
if [ "${#POSITIONAL[@]}" -ge 3 ]; then
  BRANCH="${POSITIONAL[2]}"
fi

ARCHIVE_URL="https://github.com/${REPO_OWNER}/atomic-red-team/archive/${BRANCH}.zip"
TMPDIR="$(mktemp -d)"
ARCHIVE_PATH="${TMPDIR}/${BRANCH}.zip"
EXTRACT_DIR="${TMPDIR}/extract"
ART_ROOT="${EXTRACT_DIR}/atomic-red-team-${BRANCH}"

# Techniques to fetch — one YAML per technique from the official ART repo
# SELECTED_TECHNIQUES=(
#   # Initial Access
#   T1078
#   T1190
#   T1566.001

#   # Execution
#   T1059.001
#   T1059.003
#   T1059.004
#   T1203

#   # Persistence
#   T1547.001
#   T1543.003
#   T1053.005

#   # Privilege Escalation
#   T1548.002
#   T1055
#   T1134

#   # Defense Evasion
#   T1027
#   T1562.001
#   T1070.001

#   # Credential Access
#   T1003.001
#   T1003.002
#   T1110.003
#   T1550.002

#   # Discovery
#   T1087
#   T1082
#   T1083
#   T1016
#   T1049

#   # Lateral Movement
#   T1021.001
#   T1021.002

#   # Collection
#   T1005
#   T1074

#   # Exfiltration
#   T1041
#   T1048

#   # Impact
#   T1486
#   T1490
# )

cleanup() {
  rm -rf "${TMPDIR}"
}
trap cleanup EXIT

clean_unselected() {
  local path base keep

  echo "[fetch] Cleaning ${OUTPUT_DIR} to keep only selected techniques..."
  for path in "${OUTPUT_DIR}"/*; do
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

echo "[fetch] Downloading atomics from ${REPO_OWNER}/atomic-red-team (${BRANCH})..."
mkdir -p "${EXTRACT_DIR}" "${OUTPUT_DIR}"
curl -fsSL "${ARCHIVE_URL}" -o "${ARCHIVE_PATH}"
unzip -q "${ARCHIVE_PATH}" -d "${EXTRACT_DIR}"

echo "[fetch] Merging upstream atomics into ${OUTPUT_DIR}..."
cp -an "${ART_ROOT}/atomics/." "${OUTPUT_DIR}/"

if [ "${CLEAN}" -eq 1 ]; then
  clean_unselected
fi

echo ""
echo "Done. Atomics available in: ${OUTPUT_DIR}"
if [ "${CLEAN}" -eq 1 ]; then
  echo "Clean mode: kept only ${#SELECTED_TECHNIQUES[@]} selected techniques"
fi
echo ""
echo "To run atomics locally using GoART (https://github.com/lcensies/go-atomicredteam):"
echo "  go install github.com/lcensies/go-atomicredteam/cmd/goart@latest"
echo "  goart --technique T1059.001 --index 0 --atomics-path ${OUTPUT_DIR}"
