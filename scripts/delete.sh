#!/usr/bin/env bash

set -euo pipefail

log() { echo ""; echo "[INFO] $1"; }
warn() { echo "[WARN] $1"; }
err() { echo "[ERROR] $1" >&2; }

usage() {
  cat <<'EOF'
Delete Bebii Blog production instances on Ubuntu.

Usage:
  sudo ./scripts/delete-production-ubuntu.sh [options]

Selectors:
  --domains a.com,b.com   Delete by domain(s)
  --apps slug1,slug2      Delete by app slug(s)
  --all                   Delete all detected instances

Options:
  --keep-db               Keep database and db user
  --keep-files            Keep /opt/<slug> directory
  --keep-cert             Keep certbot certificate
  --yes, -y               Skip final confirmation
  --help, -h              Show this help

Notes:
  - Jika tidak memberi selector (--domains/--apps/--all), script akan
    menampilkan list domain bernomor untuk dipilih.
  - Default action deletes app files, systemd service, nginx config,
    database, DB user, and certbot cert (if exists).
EOF
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || { err "Command '$1' belum tersedia."; exit 1; }
}

ensure_ubuntu() {
  [[ -f /etc/os-release ]] || { err "/etc/os-release tidak ditemukan."; exit 1; }
  # shellcheck disable=SC1091
  source /etc/os-release
  [[ "${ID:-}" == "ubuntu" ]] || { err "Script ini khusus Ubuntu. Terdeteksi: ${ID:-unknown}"; exit 1; }
}

slug_exists() {
  local slug="$1"
  [[ -d "/opt/${slug}" && -f "/opt/${slug}/shared/.env.production" ]]
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

find_domains_for_slug() {
  local slug="$1"
  shopt -s nullglob
  local files=(/etc/nginx/sites-available/"${slug}"-*)
  shopt -u nullglob
  local f base domain
  for f in "${files[@]}"; do
    base="$(basename "${f}")"
    domain="${base#${slug}-}"
    [[ -n "${domain}" ]] && echo "${domain}"
  done | unique_lines
}

discover_all_domains() {
  local slug domain
  while IFS= read -r slug; do
    [[ -n "${slug}" ]] || continue
    while IFS= read -r domain; do
      [[ -n "${domain}" ]] && echo "${domain}"
    done < <(find_domains_for_slug "${slug}" || true)
  done < <(discover_all_slugs || true)
}

prompt_select_domains() {
  local -n _all=$1
  local -n _out=$2
  local total="${#_all[@]}"
  [[ "${total}" -gt 0 ]] || return 1

  echo ""
  echo "Pilih domain yang ingin dihapus:"
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

read_env_value() {
  local env_file="$1"
  local key="$2"
  awk -F= -v k="${key}" '$1==k {print $2}' "${env_file}" 2>/dev/null | head -n1 | tr -d '[:space:]'
}

delete_one_slug() {
  local slug="$1"
  local keep_db="$2"
  local keep_files="$3"
  local keep_cert="$4"

  local app_home="/opt/${slug}"
  local env_file="${app_home}/shared/.env.production"
  local app_user="${slug}"
  local app_group="${slug}"
  local svc=""

  if ! slug_exists "${slug}"; then
    warn "[${slug}] struktur app tidak lengkap. Lewati."
    RESULT_LINE="${slug}|skipped|missing-structure"
    return 2
  fi

  local db_name db_user
  db_name="$(read_env_value "${env_file}" "DB_NAME")"
  db_user="$(read_env_value "${env_file}" "DB_USER")"

  local domains=()
  mapfile -t domains < <(find_domains_for_slug "${slug}" || true)

  svc="$(find_service_for_slug "${slug}" || true)"
  if [[ -n "${svc}" ]]; then
    log "[${slug}] Stop + disable service ${svc}"
    systemctl stop "${svc}" || true
    systemctl disable "${svc}" || true
    rm -f "/etc/systemd/system/${svc}"
    systemctl daemon-reload
  else
    warn "[${slug}] service tidak ditemukan."
  fi

  local domain conf link
  for domain in "${domains[@]}"; do
    conf="/etc/nginx/sites-available/${slug}-${domain}"
    link="/etc/nginx/sites-enabled/${slug}-${domain}"
    log "[${slug}] Hapus Nginx config untuk domain ${domain}"
    rm -f "${link}" "${conf}"
    if [[ "${keep_cert}" != "true" ]]; then
      if certbot certificates 2>/dev/null | grep -Fq "Domains: ${domain}"; then
        certbot delete --cert-name "${domain}" --non-interactive || warn "[${slug}] gagal hapus cert ${domain}"
      fi
    fi
  done
  nginx -t && systemctl reload nginx || warn "[${slug}] nginx reload gagal"

  if [[ "${keep_db}" != "true" ]]; then
    if [[ -n "${db_name}" || -n "${db_user}" ]]; then
      log "[${slug}] Hapus database/user"
      mysql <<EOF || warn "[${slug}] mysql drop gagal"
$( [[ -n "${db_name}" ]] && echo "DROP DATABASE IF EXISTS \`${db_name}\`;")
$( [[ -n "${db_user}" ]] && echo "DROP USER IF EXISTS '${db_user}'@'127.0.0.1';")
FLUSH PRIVILEGES;
EOF
    else
      warn "[${slug}] DB_NAME/DB_USER tidak ditemukan, skip drop DB."
    fi
  fi

  if [[ "${keep_files}" != "true" ]]; then
    log "[${slug}] Hapus direktori app ${app_home}"
    rm -rf "${app_home}"
  fi

  if id -u "${app_user}" >/dev/null 2>&1; then
    log "[${slug}] Hapus user/group sistem"
    userdel "${app_user}" || true
  fi
  if getent group "${app_group}" >/dev/null 2>&1; then
    groupdel "${app_group}" || true
  fi

  RESULT_LINE="${slug}|deleted|ok"
  return 0
}

[[ "${EUID}" -eq 0 ]] || { err "Jalankan sebagai root (sudo)."; exit 1; }
ensure_ubuntu

require_cmd systemctl
require_cmd nginx
require_cmd mysql
require_cmd awk
require_cmd sed
require_cmd tr
require_cmd xargs
require_cmd certbot

DOMAINS_CSV=""
APPS_CSV=""
SELECT_ALL=false
KEEP_DB=false
KEEP_FILES=false
KEEP_CERT=false
AUTO_YES=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --domains) DOMAINS_CSV="${2:-}"; shift 2 ;;
    --apps) APPS_CSV="${2:-}"; shift 2 ;;
    --all) SELECT_ALL=true; shift ;;
    --keep-db) KEEP_DB=true; shift ;;
    --keep-files) KEEP_FILES=true; shift ;;
    --keep-cert) KEEP_CERT=true; shift ;;
    --yes|-y) AUTO_YES=true; shift ;;
    --help|-h) usage; exit 0 ;;
    *) err "Argumen tidak dikenal: $1"; usage; exit 1 ;;
  esac
done

declare -a targets=()
declare -a discovered_domains=()
declare -a selected_domains=()
if [[ -n "${APPS_CSV}" ]]; then
  while IFS= read -r s; do [[ -n "${s}" ]] && targets+=("${s}"); done < <(parse_csv "${APPS_CSV}" | unique_lines)
elif [[ -n "${DOMAINS_CSV}" ]]; then
  mapfile -t _domains < <(parse_csv "${DOMAINS_CSV}" | unique_lines)
  while IFS= read -r s; do [[ -n "${s}" ]] && targets+=("${s}"); done < <(slugs_from_domains "${_domains[@]}")
elif [[ "${SELECT_ALL}" == "true" ]]; then
  while IFS= read -r s; do [[ -n "${s}" ]] && targets+=("${s}"); done < <(discover_all_slugs || true)
else
  mapfile -t discovered_domains < <(discover_all_domains | unique_lines)
  if ! prompt_select_domains discovered_domains selected_domains; then
    err "Gagal membaca pilihan domain."
    exit 1
  fi
  while IFS= read -r s; do [[ -n "${s}" ]] && targets+=("${s}"); done < <(slugs_from_domains "${selected_domains[@]}")
fi

mapfile -t targets < <(printf '%s\n' "${targets[@]}" | unique_lines)
[[ "${#targets[@]}" -gt 0 ]] || { err "Target kosong."; exit 1; }

echo "==============================================="
echo "Delete Bebii Blog Production (Ubuntu)"
echo "==============================================="
echo "Target slugs : ${targets[*]}"
echo "Keep DB      : ${KEEP_DB}"
echo "Keep files   : ${KEEP_FILES}"
echo "Keep cert    : ${KEEP_CERT}"
echo ""

if [[ "${AUTO_YES}" != "true" ]]; then
  read -r -p "Ketik 'delete' untuk melanjutkan: " CONFIRM
  [[ "${CONFIRM}" == "delete" ]] || { echo "Dibatalkan."; exit 0; }
fi

RESULTS=()
OK=0
SKIPPED=0
FAILED=0

for slug in "${targets[@]}"; do
  if delete_one_slug "${slug}" "${KEEP_DB}" "${KEEP_FILES}" "${KEEP_CERT}"; then
    RESULTS+=("${RESULT_LINE}")
    OK=$((OK + 1))
  else
    rc=$?
    if [[ "${rc}" -eq 2 ]]; then
      RESULTS+=("${RESULT_LINE:-${slug}|skipped|missing}")
      SKIPPED=$((SKIPPED + 1))
    else
      RESULTS+=("${slug}|failed|error")
      FAILED=$((FAILED + 1))
    fi
  fi
done

echo ""
echo "==============================================="
echo "Ringkasan Delete"
echo "==============================================="
printf "%-24s | %-8s | %-16s\n" "Slug" "Status" "Detail"
printf "%-24s-+-%-8s-+-%-16s\n" "------------------------" "--------" "----------------"
for line in "${RESULTS[@]}"; do
  IFS='|' read -r s st dt <<< "${line}"
  printf "%-24s | %-8s | %-16s\n" "${s}" "${st}" "${dt}"
done
echo "-----------------------------------------------"
echo "Deleted : ${OK}"
echo "Skipped : ${SKIPPED}"
echo "Failed  : ${FAILED}"

