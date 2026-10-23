#!/usr/bin/env bash

set -euo pipefail

# GitHub auth configuration (override via env).
GITHUB_TOKEN_FILE="${GITHUB_TOKEN_FILE:-/root/.github_token}"
GITHUB_USERNAME="${GITHUB_USERNAME:-}"
GITHUB_TOKEN="${GITHUB_TOKEN:-}"
GIT_URL="${GIT_URL:-https://github.com/nikiaprian/portofolio.git}"

log() { echo ""; echo "[INFO] $1"; }
warn() { echo "[WARN] $1"; }
err() { echo "[ERROR] $1" >&2; }

load_github_credentials() {
  local f="$1"
  local raw_lines=()
  local line key val

  if [[ -f "${f}" ]]; then
    while IFS= read -r line || [[ -n "${line}" ]]; do
      line="$(printf "%s" "${line}" | sed 's/\r$//' | xargs)"
      [[ -z "${line}" || "${line}" == \#* ]] && continue

      if [[ "${line}" == *"="* ]]; then
        key="$(printf "%s" "${line%%=*}" | tr '[:lower:]' '[:upper:]' | xargs)"
        val="$(printf "%s" "${line#*=}" | xargs)"
        case "${key}" in
          GITHUB_USERNAME|USERNAME) [[ -z "${GITHUB_USERNAME}" ]] && GITHUB_USERNAME="${val}" ;;
          GITHUB_TOKEN|TOKEN) [[ -z "${GITHUB_TOKEN}" ]] && GITHUB_TOKEN="${val}" ;;
        esac
      else
        raw_lines+=("${line}")
      fi
    done < "${f}"
  fi

  if [[ "${#raw_lines[@]}" -gt 0 ]]; then
    [[ -z "${GITHUB_USERNAME}" ]] && GITHUB_USERNAME="${raw_lines[0]}"
    [[ -z "${GITHUB_TOKEN}" && "${#raw_lines[@]}" -gt 1 ]] && GITHUB_TOKEN="${raw_lines[1]}"
    if [[ -z "${GITHUB_TOKEN}" && "${#raw_lines[@]}" -eq 1 ]]; then
      GITHUB_TOKEN="${raw_lines[0]}"
    fi
  fi

  if [[ -z "${GITHUB_USERNAME}" ]]; then
    read -r -p "Masukkan GitHub username: " GITHUB_USERNAME
  fi
  if [[ -z "${GITHUB_TOKEN}" ]]; then
    read -r -s -p "Masukkan GitHub token: " GITHUB_TOKEN
    echo ""
  fi
}

usage() {
  cat <<'EOF'
Ubuntu deploy updater for Tirtayasa multi-instance.

Usage:
  sudo ./scripts/update.sh [options]

Selectors:
  --all
  --apps slug1,slug2
  --domains a.com,b.com

Options:
  --branch blog
  --seed
  --no-health
  --dry-run
  --help, -h

If no selector is passed, script will show interactive list:
  1) <slug-1>
  2) <slug-2>
  3) semua
Input examples: 1 | 1,2 | 3
EOF
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || { err "Command '$1' belum tersedia."; exit 1; }
}

ensure_ubuntu() {
  if [[ ! -f /etc/os-release ]]; then
    err "Tidak menemukan /etc/os-release."
    exit 1
  fi
  # shellcheck disable=SC1091
  source /etc/os-release
  if [[ "${ID:-}" != "ubuntu" ]]; then
    err "Script ini khusus Ubuntu. Terdeteksi: ${ID:-unknown}"
    exit 1
  fi
}

detect_go_bin() {
  if command -v go >/dev/null 2>&1; then
    command -v go
    return 0
  fi
  [[ -x /usr/local/go/bin/go ]] && { echo "/usr/local/go/bin/go"; return 0; }
  return 1
}

has_node_toolchain() {
  command -v node >/dev/null 2>&1 && command -v npm >/dev/null 2>&1
}

# Generate public/static/js/*.min.js (file tidak di-commit ke Git).
run_repo_minify() {
  local repo_dir="$1" app_user="$2"
  if [[ ! -f "${repo_dir}/minify.js" || ! -f "${repo_dir}/package.json" ]]; then
    warn "minify.js / package.json tidak ada di repo — lewati minify JS."
    return 0
  fi
  if ! has_node_toolchain; then
    err "Node.js/npm diperlukan untuk generate .min.js. Install: apt install -y nodejs npm"
    return 1
  fi
  sudo -u "${app_user}" bash -lc "cd '${repo_dir}' && npm ci && npm run dev && rm -rf node_modules"
}

slug_exists() {
  local slug="$1"
  [[ -d "/opt/${slug}" && -d "/opt/${slug}/repo/.git" && -f "/opt/${slug}/shared/.env.production" ]]
}

unique_lines() { awk 'NF && !seen[$0]++ {print $0}'; }

parse_csv() {
  local csv="$1"
  local old_ifs="${IFS}"
  IFS=','
  # shellcheck disable=SC2206
  local parts=(${csv})
  IFS="${old_ifs}"
  local p
  for p in "${parts[@]}"; do
    p="$(echo "${p}" | xargs)"
    [[ -n "${p}" ]] && echo "${p}"
  done
}

discover_all_slugs() {
  shopt -s nullglob
  local dirs=(/opt/*)
  shopt -u nullglob
  local d slug
  for d in "${dirs[@]}"; do
    [[ -d "${d}" ]] || continue
    slug="$(basename "${d}")"
    slug_exists "${slug}" && echo "${slug}"
  done | unique_lines
}

slugs_from_domains() {
  local domain
  for domain in "$@"; do
    [[ -n "${domain}" ]] || continue
    local conf slug
    conf="$(ls /etc/nginx/sites-available/*"${domain}" 2>/dev/null | head -n1 || true)"
    if [[ -z "${conf}" ]]; then
      warn "Domain '${domain}' tidak ditemukan di /etc/nginx/sites-available."
      continue
    fi
    slug="$(basename "${conf}")"
    slug="${slug%-${domain}}"
    [[ -n "${slug}" ]] && echo "${slug}"
  done | unique_lines
}

find_service_for_slug() {
  local slug="$1"
  local expected="${slug}.service"
  if systemctl list-unit-files --type=service --no-legend 2>/dev/null | awk '{print $1}' | grep -qx "${expected}"; then
    echo "${expected}"
    return 0
  fi

  local envf="/opt/${slug}/shared/.env.production"
  local svc
  while IFS= read -r svc; do
    [[ -n "${svc}" ]] || continue
    if systemctl cat "${svc}" 2>/dev/null | grep -Fq "EnvironmentFile=${envf}"; then
      echo "${svc}"
      return 0
    fi
  done < <(systemctl list-unit-files --type=service --no-legend 2>/dev/null | awk '{print $1}')

  return 1
}

prompt_select_slugs() {
  local -n _all=$1
  local -n _out=$2
  local total="${#_all[@]}"
  [[ "${total}" -gt 0 ]] || return 1

  echo ""
  echo "Pilih aplikasi yang ingin di-update:"
  local i
  for ((i=0; i<total; i++)); do
    echo "  $((i+1)). ${_all[$i]}"
  done
  local all_idx=$((total+1))
  echo "  ${all_idx}. semua"
  echo "Contoh input: 1 | 1,2 | ${all_idx}"

  local ans
  read -r -p "Pilihan: " ans
  ans="$(echo "${ans}" | tr ',' ' ' | xargs || true)"
  [[ -n "${ans}" ]] || { err "Pilihan kosong."; return 1; }

  local tok
  for tok in ${ans}; do
    [[ "${tok}" =~ ^[0-9]+$ ]] || { err "Input '${tok}' bukan angka."; return 1; }
    (( tok >= 1 && tok <= all_idx )) || { err "Pilihan '${tok}' di luar range."; return 1; }
    if (( tok == all_idx )); then
      _out=("${_all[@]}")
      return 0
    fi
    _out+=("${_all[$((tok-1))]}")
  done

  mapfile -t _out < <(printf '%s\n' "${_out[@]}" | unique_lines)
}

ensure_health() {
  local port="$1" slug="$2"
  local ok=false _
  for _ in $(seq 1 120); do
    if curl -sf --connect-timeout 2 "http://127.0.0.1:${port}/health" >/dev/null 2>&1; then
      ok=true
      break
    fi
    sleep 1
  done
  [[ "${ok}" == "true" ]] && echo "ok" || { warn "[${slug}] /health gagal di ${port}"; echo "failed"; }
}

run_seed_once() {
  local user="$1" group="$2" home="$3" envf="$4" bin="$5"
  if systemd-run --uid="${user}" --gid="${group}" --working-directory="${home}" -p "EnvironmentFile=${envf}" --wait --collect "${bin}" -seed-only; then
    return 0
  fi
  sudo -u "${user}" bash -lc "set -a; source '${envf}'; set +a; exec '${bin}' -seed-only"
}

update_one_slug() {
  local slug="$1" branch="$2" go_bin="$3" run_seed="$4" do_health="$5" dry_run="$6" gh_user="$7" gh_header="$8"
  local app_home="/opt/${slug}"
  local app_user="${slug}"
  local app_group="${slug}"
  local app_repo="${app_home}/repo"
  local app_env="${app_home}/shared/.env.production"
  local app_releases="${app_home}/releases"
  local app_bin_link="${app_home}/bin/${slug}"
  local app_service=""
  UPDATE_RESULT_LINE=""

  if ! slug_exists "${slug}"; then
    warn "[${slug}] struktur app tidak lengkap."
    UPDATE_RESULT_LINE="${slug}|skipped|n/a|n/a"
    return 2
  fi
  if ! id -u "${app_user}" >/dev/null 2>&1; then
    warn "[${slug}] user '${app_user}' tidak ada."
    UPDATE_RESULT_LINE="${slug}|skipped|n/a|n/a"
    return 2
  fi
  if ! app_service="$(find_service_for_slug "${slug}")"; then
    warn "[${slug}] service tidak ditemukan."
    UPDATE_RESULT_LINE="${slug}|skipped|n/a|n/a"
    return 2
  fi

  if [[ "${dry_run}" == "true" ]]; then
    UPDATE_RESULT_LINE="${slug}|planned|n/a|n/a"
    return 0
  fi

  log "[${slug}] Fetch + reset origin/${branch}"
  sudo -u "${app_user}" git -C "${app_repo}" remote set-url origin "${GIT_URL}"
  if [[ -n "${gh_header}" ]]; then
    sudo -u "${app_user}" git -c credential.username="${gh_user}" -c http.extraHeader="${gh_header}" -C "${app_repo}" fetch --prune origin "${branch}"
  else
    sudo -u "${app_user}" git -C "${app_repo}" fetch --prune origin "${branch}"
  fi
  sudo -u "${app_user}" git -C "${app_repo}" checkout -B "${branch}" "origin/${branch}"
  sudo -u "${app_user}" git -C "${app_repo}" reset --hard "origin/${branch}"

  log "[${slug}] Minify asset JS"
  if ! run_repo_minify "${app_repo}" "${app_user}"; then
    UPDATE_RESULT_LINE="${slug}|failed|n/a|n/a"
    return 1
  fi

  log "[${slug}] Build release"
  local rid rdir
  rid="$(date +%Y%m%d%H%M%S)"
  rdir="${app_releases}/${rid}"
  mkdir -p "${rdir}"
  chown -R "${app_user}:${app_group}" "${rdir}"
  local go_build_tmpdir="${GO_BUILD_TMPDIR:-/var/tmp}"
  mkdir -p "${go_build_tmpdir}"
  sudo -u "${app_user}" bash -lc "cd '${app_repo}' && TMPDIR='${go_build_tmpdir}' '${go_bin}' mod download && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 TMPDIR='${go_build_tmpdir}' '${go_bin}' build -ldflags='-s -w' -o '${rdir}/${slug}' ./cmd"
  chmod +x "${rdir}/${slug}"
  chown "${app_user}:${app_group}" "${rdir}/${slug}"
  ln -sfn "${rdir}/${slug}" "${app_bin_link}"
  chown -h "${app_user}:${app_group}" "${app_bin_link}"

  log "[${slug}] Restart ${app_service}"
  systemctl restart "${app_service}"

  local seed_status="skipped"
  if [[ "${run_seed}" == "true" ]]; then
    log "[${slug}] Seed -seed-only"
    if run_seed_once "${app_user}" "${app_group}" "${app_home}" "${app_env}" "${app_bin_link}"; then
      seed_status="ok"
    else
      seed_status="failed"
      warn "[${slug}] seed gagal."
    fi
  fi

  local health_status="skipped"
  if [[ "${do_health}" == "true" ]]; then
    local port
    port="$(awk -F= '$1=="HTTP_PORT"{print $2}' "${app_env}" | tr -d '[:space:]' || true)"
    if [[ -n "${port}" ]]; then
      health_status="$(ensure_health "${port}" "${slug}")"
    else
      health_status="no-port"
      warn "[${slug}] HTTP_PORT tidak ditemukan."
    fi
  fi

  UPDATE_RESULT_LINE="${slug}|updated|${health_status}|${seed_status}"
}

[[ "${EUID}" -eq 0 ]] || { err "Jalankan sebagai root (sudo)."; exit 1; }
ensure_ubuntu

require_cmd git
require_cmd systemctl
require_cmd curl
require_cmd awk
require_cmd sed
require_cmd tr
require_cmd head
require_cmd base64

GO_BIN="$(detect_go_bin || true)"
[[ -n "${GO_BIN}" ]] || { err "Go tidak ditemukan."; exit 1; }

BRANCH="${GIT_BRANCH:-blog}"
SELECT_ALL=false
APPS_CSV=""
DOMAINS_CSV=""
RUN_SEED=false
DO_HEALTH=true
DRY_RUN=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --all) SELECT_ALL=true; shift ;;
    --apps) APPS_CSV="${2:-}"; shift 2 ;;
    --domains) DOMAINS_CSV="${2:-}"; shift 2 ;;
    --branch) BRANCH="${2:-blog}"; shift 2 ;;
    --seed) RUN_SEED=true; shift ;;
    --no-health) DO_HEALTH=false; shift ;;
    --dry-run) DRY_RUN=true; shift ;;
    --help|-h) usage; exit 0 ;;
    *) err "Argumen tidak dikenal: $1"; usage; exit 1 ;;
  esac
done

declare -a discovered=()
declare -a targets=()
mapfile -t discovered < <(discover_all_slugs || true)

if [[ -n "${APPS_CSV}" ]]; then
  while IFS= read -r s; do [[ -n "${s}" ]] && targets+=("${s}"); done < <(parse_csv "${APPS_CSV}" | unique_lines)
elif [[ -n "${DOMAINS_CSV}" ]]; then
  mapfile -t _domains < <(parse_csv "${DOMAINS_CSV}" | unique_lines)
  while IFS= read -r s; do [[ -n "${s}" ]] && targets+=("${s}"); done < <(slugs_from_domains "${_domains[@]}")
elif [[ "${SELECT_ALL}" == "true" ]]; then
  targets=("${discovered[@]}")
else
  prompt_select_slugs discovered targets || { err "Gagal membaca pilihan."; exit 1; }
fi

mapfile -t targets < <(printf '%s\n' "${targets[@]}" | unique_lines)
[[ "${#targets[@]}" -gt 0 ]] || { err "Target app kosong."; exit 1; }

load_github_credentials "${GITHUB_TOKEN_FILE}"
GITHUB_AUTH_HEADER=""
if [[ -n "${GITHUB_TOKEN}" ]]; then
  GITHUB_AUTH_HEADER="AUTHORIZATION: basic $(printf "%s:%s" "${GITHUB_USERNAME}" "${GITHUB_TOKEN}" | base64 | tr -d '\n')"
fi

RESULT_LINES=()
UPDATED_COUNT=0
SKIPPED_COUNT=0
FAILED_COUNT=0

for slug in "${targets[@]}"; do
  if update_one_slug "${slug}" "${BRANCH}" "${GO_BIN}" "${RUN_SEED}" "${DO_HEALTH}" "${DRY_RUN}" "${GITHUB_USERNAME}" "${GITHUB_AUTH_HEADER}"; then
    RESULT_LINES+=("${UPDATE_RESULT_LINE}")
    UPDATED_COUNT=$((UPDATED_COUNT + 1))
  else
    rc=$?
    if [[ "${rc}" -eq 2 ]]; then
      RESULT_LINES+=("${UPDATE_RESULT_LINE:-${slug}|skipped|n/a|n/a}")
      SKIPPED_COUNT=$((SKIPPED_COUNT + 1))
    else
      RESULT_LINES+=("${slug}|failed|n/a|n/a")
      FAILED_COUNT=$((FAILED_COUNT + 1))
    fi
  fi
done

echo ""
echo "==============================================="
echo "Ringkasan Deploy Update"
echo "==============================================="
printf "%-24s | %-8s | %-10s | %-8s\n" "Slug" "Status" "Health" "Seed"
printf "%-24s-+-%-8s-+-%-10s-+-%-8s\n" "------------------------" "--------" "----------" "--------"
for line in "${RESULT_LINES[@]}"; do
  IFS='|' read -r s status health seed <<< "${line}"
  printf "%-24s | %-8s | %-10s | %-8s\n" "${s}" "${status}" "${health}" "${seed}"
done
echo "-----------------------------------------------"
echo "Updated : ${UPDATED_COUNT}"
echo "Skipped : ${SKIPPED_COUNT}"
echo "Failed  : ${FAILED_COUNT}"
echo ""
echo "Contoh cek log:"
echo "  sudo journalctl -u <service>.service -n 120 --no-pager"
echo "  sudo systemctl status <service>.service"

