#!/usr/bin/env bash

set -euo pipefail

log() {
  echo ""
  echo "[INFO] $1"
}

echo "==============================================="
echo " OpenCode Shared Service Setup (Ubuntu)"
echo "==============================================="
echo ""

if [[ "${EUID}" -ne 0 ]]; then
  echo "Script ini harus dijalankan sebagai root (sudo)."
  exit 1
fi

require_cmd() {
  local cmd="$1"
  if ! command -v "${cmd}" >/dev/null 2>&1; then
    echo "Command '${cmd}' belum tersedia. Install dulu lalu jalankan ulang."
    exit 1
  fi
}

require_cmd curl
require_cmd systemctl
require_cmd ss
require_cmd awk
require_cmd sed

OPENCODE_PORT="${OPENCODE_PORT:-4096}"
OPENCODE_HOSTNAME="${OPENCODE_HOSTNAME:-127.0.0.1}"
OPENCODE_SERVER_USERNAME="${OPENCODE_SERVER_USERNAME:-opencode}"
OPENCODE_SERVER_PASSWORD="${OPENCODE_SERVER_PASSWORD:-t8EG420DqdWB}"
OPENCODE_INSTALL_DIR="${OPENCODE_INSTALL_DIR:-/usr/local/bin}"
OPENCODE_HOME="${OPENCODE_HOME:-/opt/opencode}"
OPENCODE_USER="${OPENCODE_USER:-opencode}"
OPENCODE_GROUP="${OPENCODE_GROUP:-${OPENCODE_USER}}"
OPENCODE_ENV_FILE="${OPENCODE_HOME}/shared/.env"
OPENCODE_WORKSPACE="${OPENCODE_HOME}/workspace"
OPENCODE_LOGS="${OPENCODE_HOME}/logs"
OPENCODE_SERVICE_NAME="opencode.service"
OPENCODE_SYSTEMD_FILE="/etc/systemd/system/${OPENCODE_SERVICE_NAME}"
OPENCODE_BIN="${OPENCODE_INSTALL_DIR}/opencode"

detect_public_ip() {
  local ip=""
  ip="$(curl -4 -fsS --connect-timeout 3 https://ifconfig.me 2>/dev/null || true)"
  if [[ -z "${ip}" ]]; then
    ip="$(curl -4 -fsS --connect-timeout 3 https://api.ipify.org 2>/dev/null || true)"
  fi
  if [[ -z "${ip}" ]]; then
    ip="$(hostname -I 2>/dev/null | awk '{print $1}' || true)"
  fi
  echo "${ip}"
}

port_in_use() {
  local port="$1"
  ss -ltn 2>/dev/null | awk '{print $4}' | sed -n 's/.*:\([0-9][0-9]*\)$/\1/p' | grep -qx "${port}"
}

if ! [[ "${OPENCODE_PORT}" =~ ^[0-9]+$ ]] || (( OPENCODE_PORT < 1 || OPENCODE_PORT > 65535 )); then
  echo "OPENCODE_PORT harus angka 1-65535."
  exit 1
fi

if [[ -z "${OPENCODE_SERVER_PASSWORD}" ]]; then
  echo "OPENCODE_SERVER_PASSWORD tidak boleh kosong."
  exit 1
fi

SERVER_IP="${SERVER_IP:-$(detect_public_ip)}"
OPENCODE_URL="http://${OPENCODE_HOSTNAME}:${OPENCODE_PORT}"

echo "Ringkasan setup:"
echo "  Bind hostname : ${OPENCODE_HOSTNAME}"
echo "  Port          : ${OPENCODE_PORT}"
echo "  Username      : ${OPENCODE_SERVER_USERNAME}"
echo "  Install path  : ${OPENCODE_HOME}"
echo "  Binary        : ${OPENCODE_BIN}"
echo "  Service       : ${OPENCODE_SERVICE_NAME}"
echo "  Server IP     : ${SERVER_IP:-"(tidak terdeteksi)"}"
echo "  App URL       : ${OPENCODE_URL}"
echo ""

if [[ -f "${OPENCODE_SYSTEMD_FILE}" ]]; then
  if systemctl is-active --quiet "${OPENCODE_SERVICE_NAME}"; then
    echo "Service ${OPENCODE_SERVICE_NAME} sudah aktif."
    echo "Update konfigurasi dan restart..."
  else
    echo "Service ${OPENCODE_SERVICE_NAME} sudah ada (tidak aktif). Akan di-update."
  fi
elif port_in_use "${OPENCODE_PORT}"; then
  echo "Port ${OPENCODE_PORT} sudah dipakai proses lain."
  echo "Cek: ss -ltnp | grep :${OPENCODE_PORT}"
  echo "Atau jalankan ulang dengan OPENCODE_PORT=<port-lain>."
  exit 1
fi

resolve_opencode_candidate() {
  local p
  for p in \
    "${OPENCODE_BIN}" \
    "$(command -v opencode 2>/dev/null || true)" \
    /usr/local/bin/opencode \
    /root/.opencode/bin/opencode \
    "${HOME}/.opencode/bin/opencode"; do
    if [[ -n "${p}" && -x "${p}" ]]; then
      echo "${p}"
      return 0
    fi
  done
  return 1
}

ensure_opencode_bin() {
  local candidate="$1"
  mkdir -p "${OPENCODE_INSTALL_DIR}"
  if [[ "${candidate}" == "${OPENCODE_BIN}" ]]; then
    return 0
  fi
  # Salin ke path stabil supaya systemd tidak tergantung HOME root.
  cp -f "${candidate}" "${OPENCODE_BIN}"
  chmod 755 "${OPENCODE_BIN}"
  echo "Binary disalin: ${candidate} -> ${OPENCODE_BIN}"
}

log "1) Install OpenCode binary (jika belum ada)..."
EXISTING_BIN="$(resolve_opencode_candidate || true)"
if [[ -n "${EXISTING_BIN}" ]]; then
  echo "opencode sudah terpasang: ${EXISTING_BIN}"
  ensure_opencode_bin "${EXISTING_BIN}"
else
  # Official installer selalu ke $HOME/.opencode/bin (tidak ada --install-dir).
  curl -fsSL https://opencode.ai/install | bash -s -- --no-modify-path
  EXISTING_BIN="$(resolve_opencode_candidate || true)"
  if [[ -z "${EXISTING_BIN}" ]]; then
    echo "Install gagal: binary opencode tidak ditemukan."
    exit 1
  fi
  ensure_opencode_bin "${EXISTING_BIN}"
fi

"${OPENCODE_BIN}" --version || true

log "2) Buat user dan direktori..."
if ! id -u "${OPENCODE_USER}" >/dev/null 2>&1; then
  useradd --system --home "${OPENCODE_HOME}" --shell /usr/sbin/nologin "${OPENCODE_USER}"
fi

mkdir -p "${OPENCODE_HOME}/shared" "${OPENCODE_WORKSPACE}" "${OPENCODE_LOGS}"
chown -R "${OPENCODE_USER}:${OPENCODE_GROUP}" "${OPENCODE_HOME}"

log "3) Tulis env..."
cat > "${OPENCODE_ENV_FILE}" <<EOF
OPENCODE_SERVER_USERNAME=${OPENCODE_SERVER_USERNAME}
OPENCODE_SERVER_PASSWORD=${OPENCODE_SERVER_PASSWORD}
EOF
chown "${OPENCODE_USER}:${OPENCODE_GROUP}" "${OPENCODE_ENV_FILE}"
chmod 600 "${OPENCODE_ENV_FILE}"

log "4) Buat/Update systemd service..."
cat > "${OPENCODE_SYSTEMD_FILE}" <<EOF
[Unit]
Description=OpenCode Shared Server (for Bebii Blog apps)
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=${OPENCODE_USER}
Group=${OPENCODE_GROUP}
WorkingDirectory=${OPENCODE_WORKSPACE}
EnvironmentFile=${OPENCODE_ENV_FILE}
ExecStart=${OPENCODE_BIN} serve --hostname ${OPENCODE_HOSTNAME} --port ${OPENCODE_PORT}
Restart=always
RestartSec=5
TimeoutStartSec=120
TimeoutStopSec=40
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=full
ProtectHome=true
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
ReadWritePaths=${OPENCODE_HOME}
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable "${OPENCODE_SERVICE_NAME}"
systemctl restart "${OPENCODE_SERVICE_NAME}"

log "5) Health check..."
HEALTH_OK=false
for _ in $(seq 1 60); do
  if curl -sf --connect-timeout 2 \
    -u "${OPENCODE_SERVER_USERNAME}:${OPENCODE_SERVER_PASSWORD}" \
    "http://${OPENCODE_HOSTNAME}:${OPENCODE_PORT}/global/health" >/dev/null 2>&1; then
    HEALTH_OK=true
    echo "✅ OpenCode merespons ${OPENCODE_URL}/global/health"
    break
  fi
  sleep 1
done

if [[ "${HEALTH_OK}" != "true" ]]; then
  echo "⚠️  Health check gagal setelah 60s. Cek log:"
  echo "    sudo journalctl -u ${OPENCODE_SERVICE_NAME} -n 80 --no-pager"
  echo "    sudo systemctl status ${OPENCODE_SERVICE_NAME} --no-pager"
  exit 1
fi

echo ""
echo "==============================================="
echo " OpenCode siap (shared service)"
echo "==============================================="
echo ""
echo "Simpan ke DB / panel (inventory):"
echo "  server_ip     : ${SERVER_IP:-"(isi manual IP publik server)"}"
echo "  opencode_url  : ${OPENCODE_URL}"
echo "  username      : ${OPENCODE_SERVER_USERNAME}"
echo "  password      : ${OPENCODE_SERVER_PASSWORD}"
echo ""
echo "Website bebii di server ini harus memanggil:"
echo "  ${OPENCODE_URL}"
echo "  (basic auth: ${OPENCODE_SERVER_USERNAME} / <password di atas>)"
echo ""
echo "Catatan:"
echo "  - Nginx / site bebii yang sudah ada TIDAK diubah."
echo "  - Port ${OPENCODE_PORT} tidak dibuka ke publik (bind ${OPENCODE_HOSTNAME})."
echo "  - Status: sudo systemctl status ${OPENCODE_SERVICE_NAME}"
echo ""
