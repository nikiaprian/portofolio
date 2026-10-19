#!/usr/bin/env bash

set -euo pipefail

log() {
  echo ""
  echo "[INFO] $1"
}

echo "==============================================="
echo " Bebii Blog Production Setup (Ubuntu)"
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
require_cmd git
require_cmd nginx
require_cmd mysql
require_cmd ufw
require_cmd ss
require_cmd curl

# Port TCP pertama yang bebas mulai dari $1 (default 8080) — untuk beberapa instance di satu server.
# Selain socket listen (ss), juga skip port yang sudah dipesan di .env / nginx proxy_pass
# agar tidak tabrakan saat instance lama sedang down / crash-loop.
find_free_tcp_port_from() {
  local start="${1:-8080}"
  local p used

  used="$(
    {
      # Port yang sudah diklaim instance lain (meski service sedang tidak listen).
      if compgen -G "/opt/*/shared/.env.production" >/dev/null 2>&1; then
        awk -F= '$1=="HTTP_PORT" {
          gsub(/[[:space:]]/, "", $2)
          if ($2 ~ /^[0-9]+$/) print $2
        }' /opt/*/shared/.env.production 2>/dev/null || true
      fi
      # Port yang sudah di-proxy nginx (termasuk app non-standard).
      if [[ -d /etc/nginx/sites-enabled ]]; then
        grep -hREo '127\.0\.0\.1:[0-9]+' /etc/nginx/sites-enabled 2>/dev/null | cut -d: -f2 || true
      fi
      # Port yang sedang LISTEN sekarang.
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

has_node_toolchain() {
  command -v node >/dev/null 2>&1 && command -v npm >/dev/null 2>&1
}

# Generate public/static/js/*.min.js (file tidak di-commit ke Git).
run_repo_minify() {
  local repo_dir="$1" app_user="$2"
  if [[ ! -f "${repo_dir}/minify.js" || ! -f "${repo_dir}/package.json" ]]; then
    echo "minify.js / package.json tidak ada di repo — lewati minify JS."
    return 0
  fi
  if ! has_node_toolchain; then
    echo "Node.js/npm diperlukan untuk generate .min.js. Install: apt install -y nodejs npm"
    exit 1
  fi
  sudo -u "${app_user}" bash -lc "cd '${repo_dir}' && npm ci && npm run dev && rm -rf node_modules"
}

GITHUB_TOKEN_FILE="${GITHUB_TOKEN_FILE:-/root/.github_token}"
GITHUB_USERNAME="${GITHUB_USERNAME:-}"
GITHUB_TOKEN="${GITHUB_TOKEN:-}"
GIT_URL="${GIT_URL:-https://github.com/nikiaprian/portofolio.git}"
GITHUB_AUTH_HEADER=""

# Password root MariaDB/MySQL (opsional): sudo MYSQL_ROOT_PASSWORD='rahasia' ./setup-production.sh
# Kosong = mysql tanpa -p (perilaku default).
MYSQL_ROOT_PASSWORD="${MYSQL_ROOT_PASSWORD:-}"

mysql_admin() {
  if [[ -n "${MYSQL_ROOT_PASSWORD:-}" ]]; then
    mysql -u root -p"${MYSQL_ROOT_PASSWORD}" "$@"
  else
    mysql "$@"
  fi
}

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

load_github_credentials "${GITHUB_TOKEN_FILE}"

if [[ -n "${GITHUB_TOKEN}" ]]; then
  GITHUB_AUTH_HEADER="AUTHORIZATION: basic $(printf "%s:%s" "${GITHUB_USERNAME}" "${GITHUB_TOKEN}" | base64 | tr -d '\n')"
fi

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

# Seed hanya dijalankan eksplisit via -seed-only pada bootstrap ini.
# Jangan aktifkan seed otomatis saat service start agar tidak crash loop jika seed gagal.
RUN_SEED_ON_STARTUP=false

APP_PORT="$(find_free_tcp_port_from 8080)" || exit 1

# Pola pembuatan DB menyesuaikan wp.sh
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
APP_REPO_DIR="${APP_HOME}/repo"
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
echo ""

log "1) Validasi dependency dari init-server.sh..."
GO_BIN=""
if command -v go >/dev/null 2>&1; then
  GO_BIN="$(command -v go)"
elif [[ -x /usr/local/go/bin/go ]]; then
  GO_BIN="/usr/local/go/bin/go"
fi

if [[ -z "${GO_BIN}" ]]; then
  echo "Go belum terpasang atau PATH belum memuat Go."
  echo "Coba:"
  echo "  - pastikan init-server.sh sudah sukses"
  echo "  - buka shell baru, atau jalankan: source /etc/profile.d/golang.sh"
  echo "  - atau install Go manual di /usr/local/go"
  exit 1
fi

if ! systemctl is-enabled nginx >/dev/null 2>&1; then
  echo "Nginx belum siap. Jalankan ./init-server.sh terlebih dahulu."
  exit 1
fi

if ! systemctl is-enabled mysql >/dev/null 2>&1; then
  echo "MySQL belum siap. Jalankan ./init-server.sh terlebih dahulu."
  exit 1
fi

GO_VERSION="$("${GO_BIN}" version | awk '{print $3}' | sed 's/go//')"
if [[ "${GO_VERSION}" != 1.25* ]]; then
  echo "Peringatan: versi Go saat ini ${GO_VERSION}, disarankan 1.25.x sesuai go.mod."
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

mkdir -p "${APP_HOME}/"{bin,shared,logs,releases,repo}
chown -R "${APP_USER}:${APP_GROUP}" "${APP_HOME}"

log "3) Clone/update repository..."
if [[ -z "${GITHUB_TOKEN}" ]]; then
  echo "GITHUB_TOKEN belum diisi."
  echo "Gunakan salah satu:"
  echo "  1) sudo GITHUB_TOKEN='ghp_xxx' ./setup-production.sh"
  echo "  2) simpan token di ${GITHUB_TOKEN_FILE}, lalu: sudo ./setup-production.sh"
  exit 1
fi

if [[ ! -d "${APP_REPO_DIR}/.git" ]]; then
  sudo -u "${APP_USER}" git -c credential.username="${GITHUB_USERNAME}" -c http.extraHeader="${GITHUB_AUTH_HEADER}" clone -b dev "${GIT_URL}" "${APP_REPO_DIR}"
else
  sudo -u "${APP_USER}" git -C "${APP_REPO_DIR}" remote set-url origin "${GIT_URL}"
  sudo -u "${APP_USER}" git -c credential.username="${GITHUB_USERNAME}" -c http.extraHeader="${GITHUB_AUTH_HEADER}" -C "${APP_REPO_DIR}" fetch --all
  sudo -u "${APP_USER}" git -C "${APP_REPO_DIR}" checkout dev
  sudo -u "${APP_USER}" git -C "${APP_REPO_DIR}" reset --hard origin/dev
fi

log "3a) Minify asset JS (generate public/static/js/*.min.js)..."
run_repo_minify "${APP_REPO_DIR}" "${APP_USER}"

log "3b) Siapkan symlink aset public..."
ln -sfn "${APP_REPO_DIR}/public" "${APP_HOME}/public"
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

log "6) Build binary dan aktifkan release..."
RELEASE_ID="$(date +%Y%m%d%H%M%S)"
RELEASE_DIR="${APP_RELEASES_DIR}/${RELEASE_ID}"
mkdir -p "${RELEASE_DIR}"
chown -R "${APP_USER}:${APP_GROUP}" "${RELEASE_DIR}"

sudo -u "${APP_USER}" bash -lc "cd '${APP_REPO_DIR}' && '${GO_BIN}' mod download && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 '${GO_BIN}' build -ldflags='-s -w' -o '${RELEASE_DIR}/${APP_SLUG}' ./cmd"
chmod +x "${RELEASE_DIR}/${APP_SLUG}"
chown "${APP_USER}:${APP_GROUP}" "${RELEASE_DIR}/${APP_SLUG}"

ln -sfn "${RELEASE_DIR}/${APP_SLUG}" "${APP_BIN_LINK}"
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

# Tanpa patch repository: GORM tetap SELECT posts.view jika field View ada di model.
# Migrasi (AutoMigrate) jalan saat service start; tunggu tabel posts lalu tambah kolom jika perlu.
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

# Seed lewat binary -seed-only (tidak bergantung pada RunSeedOnStartup di binary lama).
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

# Hardening: paksa nonaktifkan seed startup meski seed -seed-only gagal.
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
  # First deploy SSL path: keep nginx valid before Certbot creates LE files.
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
