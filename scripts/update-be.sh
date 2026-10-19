#!/usr/bin/env bash

set -euo pipefail

BEBII_REGISTRY_URL="${BEBII_REGISTRY_URL:-https://domain.myexams.web.id}"
REGISTRY_ARTIFACT="${REGISTRY_ARTIFACT:-bebii-blog}"
REGISTRY_BINARY="${REGISTRY_BINARY:-app}"
BEBII_REGISTRY_TOKEN_FILE="${BEBII_REGISTRY_TOKEN_FILE:-/root/.bebii_registry_token}"
# Sama seperti setup-production-be.sh — override via env atau BEBII_REGISTRY_TOKEN_FILE.
BEBII_REGISTRY_READ_TOKEN="${BEBII_REGISTRY_READ_TOKEN:-649648419abaff74c4b54e9aec38b9cbdc247f6dcf2154e2cbdf416c145443ec}"

log() { echo ""; echo "[INFO] $1"; }
warn() { echo "[WARN] $1"; }
err() { echo "[ERROR] $1" >&2; }

load_registry_token() {
  local f="$1"
  local line

  if [[ -n "${BEBII_REGISTRY_READ_TOKEN}" ]]; then
    return 0
  fi

  if [[ ! -f "${f}" ]]; then
    return 0
  fi

  while IFS= read -r line || [[ -n "${line}" ]]; do
    line="$(printf "%s" "${line}" | sed 's/\r$//' | xargs)"
    [[ -z "${line}" || "${line}" == \#* ]] && continue

    if [[ "${line}" == *"="* ]]; then
      local key val
      key="$(printf "%s" "${line%%=*}" | tr '[:lower:]' '[:upper:]' | xargs)"
      val="$(printf "%s" "${line#*=}" | xargs)"
      case "${key}" in
        BEBII_REGISTRY_READ_TOKEN|TOKEN|REGISTRY_READ_TOKEN)
          [[ -z "${BEBII_REGISTRY_READ_TOKEN}" ]] && BEBII_REGISTRY_READ_TOKEN="${val}"
          ;;
      esac
    else
      [[ -z "${BEBII_REGISTRY_READ_TOKEN}" ]] && BEBII_REGISTRY_READ_TOKEN="${line}"
    fi
  done < "${f}"
}

usage() {
  cat <<EOF
Ubuntu deploy updater for Bebii Blog production registry (bebii-blog / setup-production-be.sh).

Usage:
  sudo ./scripts/deploy-update-ubuntu-be.sh [options]

Selectors:
  --all
  --apps slug1,slug2
  --domains a.com,b.com

Options:
  --seed          Jalankan -seed-only setelah restart
  --no-health     Lewati health check /health
  --dry-run       Hanya tampilkan rencana update
  --help, -h

Environment (sama seperti setup-production-be.sh):
  BEBII_REGISTRY_URL          Base URL registry (default: https://domain.myexams.web.id)
  REGISTRY_ARTIFACT           Channel registry (default: bebii-blog, bukan dev)
  REGISTRY_BINARY             Nama binary di dalam bundle (default: app)
  BEBII_REGISTRY_READ_TOKEN   Bearer token untuk unduh registry
  BEBII_REGISTRY_TOKEN_FILE   File token (default: /root/.bebii_registry_token)

Jika tidak ada selector, script menampilkan instance registry production (bebii-blog) di /opt/*.
Untuk channel dev (bebii-blog-dev) pakai update-be-dev.sh.
Input contoh: 1 | 1,2 | semua
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

# Instance dari setup-production-be.sh: tidak punya repo git, punya releases + env.
slug_exists() {
  local slug="$1"
  local app_home="/opt/${slug}"
  [[ -d "${app_home}" && -f "${app_home}/shared/.env.production" && -d "${app_home}/releases" ]]
}

is_git_based_slug() {
  local slug="$1"
  [[ -d "/opt/${slug}/repo/.git" ]]
}

registry_artifact_for_slug() {
  local slug="$1"
  local envf="/opt/${slug}/shared/.env.production"
  local art=""
  [[ -f "${envf}" ]] || { echo ""; return 0; }
  art="$(awk -F= '$1=="REGISTRY_ARTIFACT"{gsub(/^[ \t]+|[ \t]+$/, "", $2); print $2; exit}' "${envf}")"
  echo "${art}"
}

is_prod_registry_slug() {
  local slug="$1"
  local art
  art="$(registry_artifact_for_slug "${slug}")"
  [[ -z "${art}" || "${art}" == "bebii-blog" ]]
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
    if slug_exists "${slug}" && ! is_git_based_slug "${slug}" && is_prod_registry_slug "${slug}"; then
      echo "${slug}"
    fi
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
  echo "Pilih aplikasi registry production (bebii-blog) yang ingin di-update:"
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

bundle_binary_in_dir() {
  local dest_dir="$1"
  local name
  for name in "${REGISTRY_BINARY}" "bebii-blog" "${REGISTRY_ARTIFACT}"; do
    [[ -n "${name}" && -f "${dest_dir}/${name}" ]] && { printf '%s' "${name}"; return 0; }
  done
  return 1
}

BUNDLE_BINARY_NAME=""

download_registry_bundle() {
  local dest_dir="$1"
  local registry_base archive url meta version bin_name

  if [[ -z "${BEBII_REGISTRY_READ_TOKEN}" ]]; then
    err "BEBII_REGISTRY_READ_TOKEN kosong. Set env atau isi ${BEBII_REGISTRY_TOKEN_FILE}."
    exit 1
  fi

  registry_base="${BEBII_REGISTRY_URL%/}"
  url="${registry_base}/api/registry/${REGISTRY_ARTIFACT}/latest?redirect=1"
  archive="$(mktemp "/tmp/${REGISTRY_ARTIFACT}-download.XXXXXX.tar.gz")"

  meta="$(curl -fsS -H "Authorization: Bearer ${BEBII_REGISTRY_READ_TOKEN}" \
    "${registry_base}/api/registry/${REGISTRY_ARTIFACT}/latest" 2>/dev/null || true)"
  version="unknown"
  if [[ -n "${meta}" ]]; then
    version="$(printf "%s" "${meta}" | sed -n 's/.*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)"
    [[ -z "${version}" ]] && version="unknown"
    echo "Registry latest: ${REGISTRY_ARTIFACT} v${version}"
  fi

  echo "Unduh bundle dari ${url} ..."
  if ! curl -fsSL -H "Authorization: Bearer ${BEBII_REGISTRY_READ_TOKEN}" -o "${archive}" "${url}"; then
    rm -f "${archive}"
    err "Gagal unduh artefak registry. Periksa BEBII_REGISTRY_URL, token, dan koneksi jaringan."
    return 1
  fi

  if ! tar -tzf "${archive}" >/dev/null 2>&1; then
    rm -f "${archive}"
    err "File unduhan bukan arsip tar.gz valid."
    return 1
  fi

  tar -xzf "${archive}" -C "${dest_dir}"
  rm -f "${archive}"

  bin_name="$(bundle_binary_in_dir "${dest_dir}")" || {
    err "Binary bundle tidak ditemukan (cari: ${REGISTRY_BINARY}, bebii-blog, ${REGISTRY_ARTIFACT})."
    return 1
  }
  if [[ ! -d "${dest_dir}/public" ]]; then
    err "Direktori public/ tidak ditemukan di dalam arsip."
    return 1
  fi

  chmod +x "${dest_dir}/${bin_name}"
  BUNDLE_BINARY_NAME="${bin_name}"
  REGISTRY_DOWNLOADED_VERSION="${version}"
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

run_reapply_layout_once() {
  local user="$1" group="$2" home="$3" envf="$4" bin="$5"
  if systemd-run --uid="${user}" --gid="${group}" --working-directory="${home}" -p "EnvironmentFile=${envf}" --wait --collect "${bin}" -reapply-layout; then
    return 0
  fi
  sudo -u "${user}" bash -lc "set -a; source '${envf}'; set +a; exec '${bin}' -reapply-layout"
}

REGISTRY_DOWNLOADED_VERSION=""

update_one_slug() {
  local slug="$1" run_seed="$2" do_health="$3" dry_run="$4"
  local app_home="/opt/${slug}"
  local app_user="${slug}"
  local app_group="${slug}"
  local app_env="${app_home}/shared/.env.production"
  local app_releases="${app_home}/releases"
  local app_bin_link="${app_home}/bin/${slug}"
  local app_service=""
  UPDATE_RESULT_LINE=""

  if is_git_based_slug "${slug}"; then
    warn "[${slug}] instance git-based (ada repo/.git). Pakai deploy-update-ubuntu.sh, bukan -be."
    UPDATE_RESULT_LINE="${slug}|skipped|n/a|n/a"
    return 2
  fi

  if ! is_prod_registry_slug "${slug}"; then
    local inst_art
    inst_art="$(registry_artifact_for_slug "${slug}")"
    [[ -z "${inst_art}" ]] && inst_art="bebii-blog"
    warn "[${slug}] channel registry '${inst_art}', bukan bebii-blog. Pakai update-be-dev.sh jika dev."
    UPDATE_RESULT_LINE="${slug}|skipped|n/a|n/a"
    return 2
  fi

  if ! slug_exists "${slug}"; then
    warn "[${slug}] struktur app registry tidak lengkap (butuh shared/.env.production + releases/)."
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

  log "[${slug}] Unduh release bundle dari registry"
  local rid rdir
  rid="$(date +%Y%m%d%H%M%S)"
  rdir="${app_releases}/${rid}"
  mkdir -p "${rdir}"
  chown -R "${app_user}:${app_group}" "${rdir}"

  REGISTRY_DOWNLOADED_VERSION="unknown"
  if ! download_registry_bundle "${rdir}"; then
    UPDATE_RESULT_LINE="${slug}|failed|n/a|n/a"
    return 1
  fi
  chown -R "${app_user}:${app_group}" "${rdir}"

  log "[${slug}] Aktifkan release v${REGISTRY_DOWNLOADED_VERSION}"
  ln -sfn "${rdir}/public" "${app_home}/public"
  chown -h "${app_user}:${app_group}" "${app_home}/public"
  ln -sfn "${rdir}/${BUNDLE_BINARY_NAME}" "${app_bin_link}"
  chown -h "${app_user}:${app_group}" "${app_bin_link}"

  # Bundle public menimpa template. Re-apply layout aktif SEBELUM restart.
  local seed_status="skipped"
  if [[ "${run_seed}" == "true" ]]; then
    log "[${slug}] Seed -seed-only (termasuk re-apply layout aktif)"
    if run_seed_once "${app_user}" "${app_group}" "${app_home}" "${app_env}" "${app_bin_link}"; then
      seed_status="ok"
    else
      seed_status="failed"
      warn "[${slug}] seed gagal."
    fi
  else
    log "[${slug}] Re-apply layout aktif (-reapply-layout)"
    if run_reapply_layout_once "${app_user}" "${app_group}" "${app_home}" "${app_env}" "${app_bin_link}"; then
      seed_status="reapplied"
    else
      seed_status="reapply-failed"
      warn "[${slug}] re-apply layout gagal — mungkin perlu Apply manual di Admin."
    fi
  fi

  log "[${slug}] Restart ${app_service}"
  systemctl restart "${app_service}"

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

  UPDATE_RESULT_LINE="${slug}|updated|${health_status}|${seed_status}|${REGISTRY_DOWNLOADED_VERSION}"
}

[[ "${EUID}" -eq 0 ]] || { err "Jalankan sebagai root (sudo)."; exit 1; }
ensure_ubuntu

require_cmd systemctl
require_cmd curl
require_cmd tar
require_cmd awk
require_cmd sed
require_cmd tr
require_cmd head
require_cmd mktemp

load_registry_token "${BEBII_REGISTRY_TOKEN_FILE}"

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
    --seed) RUN_SEED=true; shift ;;
    --no-health) DO_HEALTH=false; shift ;;
    --dry-run) DRY_RUN=true; shift ;;
    --help|-h) usage; exit 0 ;;
    --branch)
      warn "Opsi --branch diabaikan: deploy registry selalu mengambil artefak /latest."
      shift 2
      ;;
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
  if [[ "${#discovered[@]}" -eq 0 ]]; then
    err "Tidak ada instance registry production (bebii-blog) di /opt/* (setup-production-be.sh)."
    exit 1
  fi
  prompt_select_slugs discovered targets || { err "Gagal membaca pilihan."; exit 1; }
fi

mapfile -t targets < <(printf '%s\n' "${targets[@]}" | unique_lines)
[[ "${#targets[@]}" -gt 0 ]] || { err "Target app kosong."; exit 1; }

echo ""
echo "Registry : ${BEBII_REGISTRY_URL%/}/api/registry/${REGISTRY_ARTIFACT}/latest"
echo "Targets  : ${targets[*]}"

RESULT_LINES=()
UPDATED_COUNT=0
SKIPPED_COUNT=0
FAILED_COUNT=0

for slug in "${targets[@]}"; do
  if update_one_slug "${slug}" "${RUN_SEED}" "${DO_HEALTH}" "${DRY_RUN}"; then
    RESULT_LINES+=("${UPDATE_RESULT_LINE}")
    UPDATED_COUNT=$((UPDATED_COUNT + 1))
  else
    rc=$?
    if [[ "${rc}" -eq 2 ]]; then
      RESULT_LINES+=("${UPDATE_RESULT_LINE:-${slug}|skipped|n/a|n/a|n/a}")
      SKIPPED_COUNT=$((SKIPPED_COUNT + 1))
    else
      RESULT_LINES+=("${slug}|failed|n/a|n/a|n/a")
      FAILED_COUNT=$((FAILED_COUNT + 1))
    fi
  fi
done

echo ""
echo "==============================================="
echo "Ringkasan Deploy Update (Registry)"
echo "==============================================="
printf "%-24s | %-8s | %-10s | %-8s | %-12s\n" "Slug" "Status" "Health" "Seed" "Version"
printf "%-24s-+-%-8s-+-%-10s-+-%-8s-+-%-12s\n" "------------------------" "--------" "----------" "--------" "------------"
for line in "${RESULT_LINES[@]}"; do
  IFS='|' read -r s status health seed version <<< "${line}"
  printf "%-24s | %-8s | %-10s | %-8s | %-12s\n" "${s}" "${status}" "${health}" "${seed}" "${version:-n/a}"
done
echo "-----------------------------------------------"
echo "Updated : ${UPDATED_COUNT}"
echo "Skipped : ${SKIPPED_COUNT}"
echo "Failed  : ${FAILED_COUNT}"
echo ""
echo "Contoh cek log:"
echo "  sudo journalctl -u <service>.service -n 120 --no-pager"
echo "  sudo systemctl status <service>.service"
