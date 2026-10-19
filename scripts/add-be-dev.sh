#!/usr/bin/env bash

set -euo pipefail

log() {
  echo ""
  echo "[INFO] $1"
}

echo "==============================================="
echo " Bebii Blog Setup — Registry DEV (bebii-blog-dev)"
echo "==============================================="
echo ""

if [[ "${EUID}" -ne 0 ]]; then
  echo "Script ini harus dijalankan sebagai root (sudo)."
  exit 1
fi

require_cmd() {
  local cmd="$1"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "Command '$cmd' belum tersedia. Install dulu lalu jalankan ulang."
    exit 1
  fi
}

require_cmd apt
require_cmd systemctl
require_cmd openssl
require_cmd sed
require_cmd awk
require_cmd tar
require_cmd nginx
require_cmd mysql
require_cmd ufw
require_cmd ss
require_cmd curl

# Selain socket listen (ss), juga skip port yang sudah dipesan di .env / nginx proxy_pass
# agar tidak tabrakan saat instance lama sedang down / crash-loop.
find_free_tcp_port_from() {
  local start="${1:-8080}"
  local p used

  used="$(
    {
      if compgen -G "/opt/*/shared/.env.production" >/dev/null 2>&1; then
        awk -F= '$1=="HTTP_PORT" {
          gsub(/[[:space:]]/, "", $2)
          if ($2 ~ /^[0-9]+$/) print $2
        }' /opt/*/shared/.env.production 2>/dev/null || true
      fi
      if [[ -d /etc/nginx/sites-enabled ]]; then
        grep -hREo '127\.0\.0\.1:[0-9]+' /etc/nginx/sites-enabled 2>/dev/null | cut -d: -f2 || true
      fi
      ss -ltn 2>/dev/null | awk '{print $4}' | sed -n 's/.*:\([0-9][0-9]*\)$/\1/p' || true
    } | sort -n | uniq
  )"

  for ((p=start; p<=65535; p++)); do
    if ! printf '%s\n' "${used}" | grep -qx "${p}"; then
      echo "${p}"
      return 0
    fi
  done
  echo "Tidak ada port TCP kosong di rentang ${start}-65535." >&2
  return 1
}

BEBII_REGISTRY_URL="${BEBII_REGISTRY_URL:-https://domain.myexams.web.id}"
REGISTRY_ARTIFACT="${REGISTRY_ARTIFACT:-bebii-blog-dev}"
REGISTRY_BINARY="${REGISTRY_BINARY:-app}"
# TODO: pindah ke env/file secret — override via BEBII_REGISTRY_READ_TOKEN
BEBII_REGISTRY_READ_TOKEN="${BEBII_REGISTRY_READ_TOKEN:-649648419abaff74c4b54e9aec38b9cbdc247f6dcf2154e2cbdf416c145443ec}"

MYSQL_ROOT_PASSWORD="${MYSQL_ROOT_PASSWORD:-}"

mysql_admin() {
  if [[ -n "${MYSQL_ROOT_PASSWORD:-}" ]]; then
    mysql -u root -p"${MYSQL_ROOT_PASSWORD}" "$@"
  else
    mysql "$@"
  fi
}

download_registry_bundle() {
  local dest_dir="$1"
  local registry_base archive url meta version bin_name

  registry_base="${BEBII_REGISTRY_URL%/}"
  url="${registry_base}/api/registry/${REGISTRY_ARTIFACT}/latest?redirect=1"
  archive="$(mktemp "/tmp/${REGISTRY_ARTIFACT}-download.XXXXXX.tar.gz")"

  meta="$(curl -fsS -H "Authorization: Bearer ${BEBII_REGISTRY_READ_TOKEN}" \
    "${registry_base}/api/registry/${REGISTRY_ARTIFACT}/latest" 2>/dev/null || true)"
  if [[ -n "${meta}" ]]; then
    version="$(printf "%s" "${meta}" | sed -n 's/.*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)"
    if [[ -n "${version}" ]]; then
      echo "Registry latest: ${REGISTRY_ARTIFACT} v${version}"
    fi
  fi

  echo "Unduh bundle dari ${url} ..."
  if ! curl -fsSL -H "Authorization: Bearer ${BEBII_REGISTRY_READ_TOKEN}" -o "${archive}" "${url}"; then
    rm -f "${archive}"
    echo "Gagal unduh artefak registry. Periksa BEBII_REGISTRY_URL, BEBII_REGISTRY_READ_TOKEN, dan koneksi jaringan." >&2
    exit 1
  fi

  if ! tar -tzf "${archive}" >/dev/null 2>&1; then
    rm -f "${archive}"
    echo "File unduhan bukan arsip tar.gz valid." >&2
    exit 1
  fi

  tar -xzf "${archive}" -C "${dest_dir}"
  rm -f "${archive}"

  bin_name="$(bundle_binary_in_dir "${dest_dir}")" || {
    echo "Binary bundle tidak ditemukan (cari: ${REGISTRY_BINARY}, bebii-blog-dev, bebii-blog)." >&2
    exit 1
  }
  if [[ ! -d "${dest_dir}/public" ]]; then
    echo "Direktori public/ tidak ditemukan di dalam arsip." >&2
    exit 1
  fi

  chmod +x "${dest_dir}/${bin_name}"
  BUNDLE_BINARY_NAME="${bin_name}"
}

bundle_binary_in_dir() {
  local dest_dir="$1"
  local name
  for name in "${REGISTRY_BINARY}" "bebii-blog-dev" "bebii-blog"; do
    [[ -n "${name}" && -f "${dest_dir}/${name}" ]] && { printf '%s' "${name}"; return 0; }
  done
  return 1
}

BUNDLE_BINARY_NAME=""

read -r -p "Masukkan nama domain (contoh: blog.example.com): " DOMAIN_NAME
if [[ -z "${DOMAIN_NAME}" ]]; then
  echo "Domain tidak boleh kosong."
  exit 1
fi
DOMAIN_BASE="${DOMAIN_NAME#www.}"
DOMAIN_NON_WWW="${DOMAIN_BASE}"
DOMAIN_WWW="www.${DOMAIN_BASE}"
echo "Pilih canonical redirect domain:"
echo "  1) non-www (${DOMAIN_NON_WWW})"
echo "  2) www (${DOMAIN_WWW})"
read -r -p "Pilih [1/2] (default: 1): " DOMAIN_CANONICAL_OPTION
DOMAIN_CANONICAL_OPTION="${DOMAIN_CANONICAL_OPTION:-1}"
if [[ "${DOMAIN_CANONICAL_OPTION}" == "2" ]]; then
  DOMAIN_CANONICAL="${DOMAIN_WWW}"
else
  DOMAIN_CANONICAL="${DOMAIN_NON_WWW}"
fi

read -r -p "Ambil SSL Let's Encrypt sekarang? (Y/n): " SSL_OPTION
SSL_OPTION="${SSL_OPTION:-Y}"
if [[ "${SSL_OPTION}" =~ ^[Yy]$ ]]; then
  SSL_ENABLED=true
  read -r -p "Masukkan email untuk SSL: " SSL_EMAIL
  if [[ -z "${SSL_EMAIL}" ]]; then
    echo "Email SSL tidak boleh kosong."
    exit 1
  fi
else
  SSL_ENABLED=false
  SSL_EMAIL=""
fi

read -r -p "Masukkan password MySQL untuk user aplikasi (kosong = generate random): " DB_PASS_INPUT
if [[ -z "${DB_PASS_INPUT}" ]]; then
  MYSQL_PASSWORD="$(openssl rand -base64 32 | tr -d '=+/' | cut -c1-20)"
else
  MYSQL_PASSWORD="${DB_PASS_INPUT}"
fi

read -r -p "Masukkan JWT secret (kosong = generate random 64 chars): " JWT_SECRET_INPUT
if [[ -z "${JWT_SECRET_INPUT}" ]]; then
  JWT_SECRET="$(openssl rand -hex 32)"
else
  JWT_SECRET="${JWT_SECRET_INPUT}"
fi

RUN_SEED_ON_STARTUP=false

APP_PORT="$(find_free_tcp_port_from 8080)" || exit 1

DOMAIN_SANITIZED="$(echo "${DOMAIN_NAME}" | tr -d '.-')"
DOMAIN_CLEAN="${DOMAIN_SANITIZED}"
DOMAIN_HASH="$(echo -n "${DOMAIN_NAME}" | md5sum | cut -c1-8)"
APP_SLUG="${DOMAIN_SANITIZED}"
DB_NAME="db_${DOMAIN_SANITIZED}"
DB_USER="u_$(echo "${DOMAIN_CLEAN}" | cut -c1-20)_${DOMAIN_HASH}"
if [[ ${#DB_USER} -gt 32 ]]; then
  DB_USER="u_$(echo "${DOMAIN_CLEAN}" | cut -c1-15)_${DOMAIN_HASH}"
fi

APP_USER="${APP_SLUG}"
APP_GROUP="${APP_USER}"
APP_HOME="/opt/${APP_SLUG}"
APP_ENV_FILE="${APP_HOME}/shared/.env.production"
APP_BIN_LINK="${APP_HOME}/bin/${APP_SLUG}"
APP_RELEASES_DIR="${APP_HOME}/releases"
APP_SERVICE_NAME="${APP_SLUG}.service"
APP_SYSTEMD_FILE="/etc/systemd/system/${APP_SERVICE_NAME}"
NGINX_SITE_FILE="/etc/nginx/sites-available/${APP_SLUG}-${DOMAIN_NAME}"
NGINX_SITE_LINK="/etc/nginx/sites-enabled/${APP_SLUG}-${DOMAIN_NAME}"

echo ""
echo "Ringkasan setup:"
echo "  Domain Base  : ${DOMAIN_BASE}"
echo "  Domain WWW   : ${DOMAIN_WWW}"
echo "  Canonical    : ${DOMAIN_CANONICAL}"
echo "  SSL          : ${SSL_ENABLED}"
echo "  App Slug     : ${APP_SLUG}"
echo "  App Port     : ${APP_PORT} (otomatis: 8080 ke atas jika sibuk)"
echo "  Seed Startup : ${RUN_SEED_ON_STARTUP}"
echo "  DB Name      : ${DB_NAME}"
echo "  DB User      : ${DB_USER}"
echo "  App User     : ${APP_USER}"
echo "  Install Path : ${APP_HOME}"
echo "  Registry     : ${BEBII_REGISTRY_URL%/}/api/registry/${REGISTRY_ARTIFACT}/latest"
echo "  Artifact     : ${REGISTRY_ARTIFACT}"
echo ""

RELEASE_ID="$(date +%Y%m%d%H%M%S)"
RELEASE_DIR="${APP_RELEASES_DIR}/${RELEASE_ID}"

log "1) Validasi dependency dari init-server.sh..."
if ! systemctl is-enabled nginx >/dev/null 2>&1; then
  echo "Nginx belum siap. Jalankan ./init-server.sh terlebih dahulu."
  exit 1
fi

if ! systemctl is-enabled mysql >/dev/null 2>&1; then
  echo "MySQL belum siap. Jalankan ./init-server.sh terlebih dahulu."
  exit 1
fi

if [[ "${SSL_ENABLED}" == "true" ]]; then
  require_cmd certbot
fi

if [[ -n "${MYSQL_ROOT_PASSWORD}" ]]; then
  if ! mysql_admin -N -e "SELECT 1" >/dev/null 2>&1; then
    echo "Login root MariaDB/MySQL gagal. Periksa MYSQL_ROOT_PASSWORD."
    exit 1
  fi
  echo "✅ Koneksi root MariaDB/MySQL OK (dengan password)."
fi

log "2) Buat user aplikasi dan direktori..."
if ! id -u "${APP_USER}" >/dev/null 2>&1; then
  useradd --system --home "${APP_HOME}" --shell /usr/sbin/nologin "${APP_USER}"
fi

mkdir -p "${APP_HOME}/"{bin,shared,logs,releases}
mkdir -p "${RELEASE_DIR}"
chown -R "${APP_USER}:${APP_GROUP}" "${APP_HOME}"

log "3) Unduh release bundle dari Bebii Registry..."
download_registry_bundle "${RELEASE_DIR}"
chown -R "${APP_USER}:${APP_GROUP}" "${RELEASE_DIR}"

log "3b) Siapkan symlink aset public..."
ln -sfn "${RELEASE_DIR}/public" "${APP_HOME}/public"
chown -h "${APP_USER}:${APP_GROUP}" "${APP_HOME}/public"

log "4) Setup database dan user..."
mysql_admin <<EOF
CREATE DATABASE IF NOT EXISTS \`${DB_NAME}\` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
DROP USER IF EXISTS '${DB_USER}'@'127.0.0.1';
CREATE USER '${DB_USER}'@'127.0.0.1' IDENTIFIED BY '${MYSQL_PASSWORD}';
GRANT ALL PRIVILEGES ON \`${DB_NAME}\`.* TO '${DB_USER}'@'127.0.0.1';
FLUSH PRIVILEGES;
EOF

log "5) Tulis env production..."
cat > "${APP_ENV_FILE}" <<EOF
APP_ENV=production
GIN_MODE=release
RUN_SEED_ON_STARTUP=${RUN_SEED_ON_STARTUP}
HTTP_PORT=${APP_PORT}
REGISTRY_ARTIFACT=${REGISTRY_ARTIFACT}
REGISTRY_BINARY=${REGISTRY_BINARY}

DB_HOST=127.0.0.1
DB_PORT=3306
DB_USER=${DB_USER}
DB_PASSWORD=${MYSQL_PASSWORD}
DB_NAME=${DB_NAME}

JWT_SECRET=${JWT_SECRET}
JWT_EXPIRES_IN_HOURS=24

ADMIN_LOG_RETENTION_HOURS=24

REDIS_ADDR=127.0.0.1:6379
REDIS_PASSWORD=
REDIS_DB=0
REDIS_CACHE_TTL_SECONDS=180
REDIS_CACHE_NAMESPACE=${APP_SLUG}
REDIS_CACHE_ADMIN_AUTHORIZED=false
EOF
chown "${APP_USER}:${APP_GROUP}" "${APP_ENV_FILE}"
chmod 600 "${APP_ENV_FILE}"

log "6) Aktifkan release binary..."
ln -sfn "${RELEASE_DIR}/${BUNDLE_BINARY_NAME}" "${APP_BIN_LINK}"
chown -h "${APP_USER}:${APP_GROUP}" "${APP_BIN_LINK}"

log "7) Buat/Update systemd service..."
cat > "${APP_SYSTEMD_FILE}" <<EOF
[Unit]
Description=Bebii Blog Service
After=network.target mysql.service redis-server.service
Wants=network-online.target

[Service]
Type=simple
User=${APP_USER}
Group=${APP_GROUP}
WorkingDirectory=${APP_HOME}
ExecStart=${APP_BIN_LINK}
EnvironmentFile=${APP_ENV_FILE}
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
ReadWritePaths=${APP_HOME}
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable "${APP_SERVICE_NAME}"
systemctl restart "${APP_SERVICE_NAME}"

log "7a) Tunggu app & migrasi (endpoint /health di localhost)..."
HEALTH_OK=false
for _ in $(seq 1 120); do
  if curl -sf --connect-timeout 2 "http://127.0.0.1:${APP_PORT}/health" >/dev/null 2>&1; then
    HEALTH_OK=true
    echo "✅ App merespons http://127.0.0.1:${APP_PORT}/health"
    break
  fi
  sleep 1
done
if [[ "${HEALTH_OK}" != "true" ]]; then
  echo "⚠️  /health belum OK setelah 120s. Cek: sudo journalctl -u ${APP_SERVICE_NAME} -n 80 --no-pager"
fi

log "7b) Kompatibilitas DB: kolom posts.view (setelah migrasi pertama)..."
for _ in $(seq 1 120); do
  if [[ "$(mysql_admin -N -e "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA='${DB_NAME}' AND TABLE_NAME='posts'")" == "1" ]]; then
    break
  fi
  sleep 1
done
if [[ "$(mysql_admin -N -e "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA='${DB_NAME}' AND TABLE_NAME='posts'")" == "1" ]]; then
  if [[ "$(mysql_admin -N -e "SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA='${DB_NAME}' AND TABLE_NAME='posts' AND COLUMN_NAME='view'")" == "0" ]]; then
    mysql_admin -e "USE \`${DB_NAME}\`; ALTER TABLE posts ADD COLUMN \`view\` BIGINT NOT NULL DEFAULT 0;"
    echo "✅ Kolom posts.view ditambahkan (kompatibilitas tanpa ubah post_repository.go)."
  fi
else
  echo "⚠️  Tabel posts belum ada setelah 120s. Setelah app jalan, jalankan manual:"
  echo "    mysql -e \"USE \\\`${DB_NAME}\\\`; ALTER TABLE posts ADD COLUMN \\\`view\\\` BIGINT NOT NULL DEFAULT 0;\""
fi

log "7c) Jalankan seed sekali (mode -seed-only, env sama seperti systemd)..."
SEED_RAN=false
if systemd-run \
  --uid="${APP_USER}" \
  --gid="${APP_GROUP}" \
  --working-directory="${APP_HOME}" \
  -p "EnvironmentFile=${APP_ENV_FILE}" \
  --wait --collect \
  "${APP_BIN_LINK}" -seed-only; then
  SEED_RAN=true
  echo "✅ Seed selesai (-seed-only)."
elif sudo -u "${APP_USER}" bash -lc "set -a; source '${APP_ENV_FILE}'; set +a; exec '${APP_BIN_LINK}' -seed-only"; then
  SEED_RAN=true
  echo "✅ Seed selesai (-seed-only, fallback bash)."
else
  echo "⚠️  Seed (-seed-only) gagal. Lanjutkan deploy tanpa mematikan service."
  echo "    Coba manual saat dibutuhkan:"
  echo "    sudo systemd-run --uid=${APP_USER} --gid=${APP_GROUP} -p EnvironmentFile=${APP_ENV_FILE} --wait --collect ${APP_BIN_LINK} -seed-only"
fi

sed -i 's/^RUN_SEED_ON_STARTUP=.*/RUN_SEED_ON_STARTUP=false/' "${APP_ENV_FILE}"
echo "✅ RUN_SEED_ON_STARTUP dipaksa false di ${APP_ENV_FILE}"

log "8) Setup Nginx domain..."
LE_SSL_DIR="/etc/letsencrypt/live/${DOMAIN_BASE}"
LE_SSL_CERT="${LE_SSL_DIR}/fullchain.pem"
LE_SSL_KEY="${LE_SSL_DIR}/privkey.pem"
LE_SSL_OPTIONS="/etc/letsencrypt/options-ssl-nginx.conf"
LE_SSL_DHPARAM="/etc/letsencrypt/ssl-dhparams.pem"

if [[ "${SSL_ENABLED}" == "true" && -f "${LE_SSL_CERT}" && -f "${LE_SSL_KEY}" && -f "${LE_SSL_OPTIONS}" && -f "${LE_SSL_DHPARAM}" ]]; then
  cat > "${NGINX_SITE_FILE}" <<EOF
server {
    listen 80;
    listen [::]:80;
    server_name ${DOMAIN_NON_WWW} ${DOMAIN_WWW};
    return 301 https://${DOMAIN_CANONICAL}\$request_uri;
}

server {
    listen 443 ssl;
    listen [::]:443 ssl;
    http2 on;
    server_name ${DOMAIN_NON_WWW} ${DOMAIN_WWW};

    ssl_certificate ${LE_SSL_CERT};
    ssl_certificate_key ${LE_SSL_KEY};
    include ${LE_SSL_OPTIONS};
    ssl_dhparam ${LE_SSL_DHPARAM};

    if (\$host != ${DOMAIN_CANONICAL}) {
        return 301 https://${DOMAIN_CANONICAL}\$request_uri;
    }

    client_max_body_size 100m;

    location /static/ {
        proxy_pass http://127.0.0.1:${APP_PORT};
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        expires 7d;
        add_header Cache-Control "public, max-age=604800, immutable";
    }

    location / {
        proxy_pass http://127.0.0.1:${APP_PORT};
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        proxy_read_timeout 60s;
        proxy_connect_timeout 10s;
    }
}
EOF
else
  cat > "${NGINX_SITE_FILE}" <<EOF
server {
    listen 80;
    listen [::]:80;
    server_name ${DOMAIN_NON_WWW} ${DOMAIN_WWW};

    client_max_body_size 100m;

    location /static/ {
        proxy_pass http://127.0.0.1:${APP_PORT};
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        expires 7d;
        add_header Cache-Control "public, max-age=604800, immutable";
    }

    location / {
        if (\$host != ${DOMAIN_CANONICAL}) {
            return 301 http://${DOMAIN_CANONICAL}\$request_uri;
        }
        proxy_pass http://127.0.0.1:${APP_PORT};
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        proxy_read_timeout 60s;
        proxy_connect_timeout 10s;
    }
}
EOF
fi

ln -sfn "${NGINX_SITE_FILE}" "${NGINX_SITE_LINK}"
nginx -t
systemctl enable nginx
systemctl reload nginx

log "9) Firewall baseline..."
ufw allow OpenSSH || true
ufw allow 'Nginx Full' || true
ufw --force enable || true

if [[ "${SSL_ENABLED}" == "true" ]]; then
  log "10) Ambil SSL certificate (Let's Encrypt)..."
  if certbot --nginx --non-interactive --agree-tos --email "${SSL_EMAIL}" -d "${DOMAIN_BASE}" -d "${DOMAIN_WWW}" --redirect; then
    cat > "${NGINX_SITE_FILE}" <<EOF
server {
    listen 80;
    listen [::]:80;
    server_name ${DOMAIN_NON_WWW} ${DOMAIN_WWW};
    return 301 https://${DOMAIN_CANONICAL}\$request_uri;
}

server {
    listen 443 ssl;
    listen [::]:443 ssl;
    http2 on;
    server_name ${DOMAIN_NON_WWW} ${DOMAIN_WWW};

    ssl_certificate ${LE_SSL_CERT};
    ssl_certificate_key ${LE_SSL_KEY};
    include ${LE_SSL_OPTIONS};
    ssl_dhparam ${LE_SSL_DHPARAM};

    if (\$host != ${DOMAIN_CANONICAL}) {
        return 301 https://${DOMAIN_CANONICAL}\$request_uri;
    }

    client_max_body_size 100m;

    location /static/ {
        proxy_pass http://127.0.0.1:${APP_PORT};
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        expires 7d;
        add_header Cache-Control "public, max-age=604800, immutable";
    }

    location / {
        proxy_pass http://127.0.0.1:${APP_PORT};
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        proxy_read_timeout 60s;
        proxy_connect_timeout 10s;
    }
}
EOF
    nginx -t
    systemctl reload nginx
    echo "✅ SSL berhasil: https://${DOMAIN_CANONICAL}/"
  else
    echo ""
    echo "⚠️  SSL gagal (HTTP tetap jalan). Cek DNS domain mengarah ke IP server ini, port 80 terbuka, lalu jalankan:"
    echo "    sudo certbot --nginx -d ${DOMAIN_BASE} -d ${DOMAIN_WWW}"
    echo ""
  fi
fi

echo ""
echo "==============================================="
echo "Setup selesai"
echo "==============================================="
echo "Domain Base    : ${DOMAIN_BASE}"
echo "Domain WWW     : ${DOMAIN_WWW}"
echo "Canonical      : ${DOMAIN_CANONICAL}"
echo "SSL enabled    : ${SSL_ENABLED}"
echo "Service        : ${APP_SERVICE_NAME}"
echo "Systemd file   : ${APP_SYSTEMD_FILE}"
echo "Nginx conf     : ${NGINX_SITE_FILE}"
echo "Env file       : ${APP_ENV_FILE}"
echo "✅ DB name      : ${DB_NAME}"
echo "✅ DB user      : ${DB_USER}"
echo "✅ DB password  : ${MYSQL_PASSWORD}"
if grep -q '^RUN_SEED_ON_STARTUP=true' "${APP_ENV_FILE}" 2>/dev/null; then
  echo "⚠️  RUN_SEED_ON_STARTUP masih true — set ke false manual jika seed sudah OK: ${APP_ENV_FILE}"
fi
echo ""
echo "Perintah cek:"
echo "  sudo systemctl status ${APP_SERVICE_NAME}"
echo "  sudo journalctl -u ${APP_SERVICE_NAME} -f"
if [[ "${SSL_ENABLED}" == "true" ]]; then
  echo "  https://${DOMAIN_CANONICAL}/health"
else
  echo "  http://${DOMAIN_CANONICAL}/health"
fi
