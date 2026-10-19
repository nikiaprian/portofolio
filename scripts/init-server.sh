#!/usr/bin/env bash

set -euo pipefail
DEBUG="${DEBUG:-false}"
if [[ "${DEBUG}" == "true" ]]; then
  set -x
fi

echo "==============================================="
echo " Ubuntu Server Init for Bebii Blog"
echo "==============================================="
echo ""

if [[ "${EUID}" -ne 0 ]]; then
  echo "Jalankan script ini sebagai root (sudo)."
  exit 1
fi

require_cmd() {
  local cmd="$1"
  if ! command -v "${cmd}" >/dev/null 2>&1; then
    echo "Command wajib tidak ditemukan: ${cmd}"
    exit 1
  fi
}

require_cmd apt
require_cmd curl
require_cmd systemctl
require_cmd lsb_release
require_cmd openssl

log() {
  echo ""
  echo "[INFO] $1"
}

detect_php_fpm_version() {
  local ver="" unit=""
  if command -v php >/dev/null 2>&1; then
    ver="$(php -r 'echo PHP_MAJOR_VERSION.".".PHP_MINOR_VERSION;' 2>/dev/null || true)"
  fi
  if [[ -n "${ver}" ]] && systemctl cat "php${ver}-fpm.service" >/dev/null 2>&1; then
    printf '%s\n' "${ver}"
    return 0
  fi
  unit="$(systemctl list-unit-files --type=service --no-legend 'php*-fpm.service' --no-pager 2>/dev/null | awk 'NR==1 { print $1 }')"
  if [[ "${unit}" =~ ^php(.+)-fpm\.service$ ]]; then
    printf '%s\n' "${BASH_REMATCH[1]}"
    return 0
  fi
  return 1
}

GO_VERSION="${GO_VERSION:-1.25.0}"
INSTALL_PMA="${INSTALL_PMA:-true}"
PMA_PORT="${PMA_PORT:-51623}"
PMA_USER="${PMA_USER:-pma_web}"
PMA_PASSWORD="${PMA_PASSWORD:-}"
MYSQL_PMA_USER="${MYSQL_PMA_USER:-pma_admin}"
MYSQL_PMA_PASSWORD="${MYSQL_PMA_PASSWORD:-}"

if [[ "${INSTALL_PMA}" != "true" && "${INSTALL_PMA}" != "false" ]]; then
  echo "Nilai INSTALL_PMA harus true/false."
  exit 1
fi

if ! [[ "${PMA_PORT}" =~ ^[0-9]+$ ]]; then
  echo "PMA_PORT harus angka."
  exit 1
fi

if [[ "${INSTALL_PMA}" == "true" && -z "${PMA_PASSWORD}" ]]; then
  read -r -p "Masukkan password phpMyAdmin basic auth: " PMA_PASSWORD
  echo ""
  if [[ -z "${PMA_PASSWORD}" ]]; then
    echo "Password phpMyAdmin tidak boleh kosong."
    exit 1
  fi
fi

if [[ "${INSTALL_PMA}" == "true" && -z "${MYSQL_PMA_PASSWORD}" ]]; then
  MYSQL_PMA_PASSWORD="$(openssl rand -base64 32 | tr -d '=+/' | cut -c1-20)"
fi

echo ""
echo "Mode: non-interaktif (tanpa konfirmasi)."
echo "Paket yang akan di-install:"
echo "- nginx (+ catch-all default vhost untuk domain belum terinstall)"
echo "- mysql-server"
echo "- redis-server"
echo "- php-fpm + ekstensi mysql/xml/mbstring/zip/curl/gd"
echo "- certbot + python3-certbot-nginx"
echo "- ufw + fail2ban"
echo "- git + unzip + build-essential + jq + htop + apache2-utils"
echo "- nodejs + npm (minify JS untuk add.sh / update.sh)"
echo "- golang ${GO_VERSION} (official tarball)"
if [[ "${INSTALL_PMA}" == "true" ]]; then
  echo "- phpMyAdmin (manual install, Nginx listen port ${PMA_PORT})"
fi
echo ""

log "1) Update sistem..."
apt update
DEBIAN_FRONTEND=noninteractive apt upgrade -y

log "2) Install paket dasar production..."
DEBIAN_FRONTEND=noninteractive apt install -y \
  ca-certificates \
  curl \
  wget \
  gnupg \
  lsb-release \
  software-properties-common \
  apt-transport-https \
  git \
  unzip \
  tar \
  build-essential \
  apache2-utils \
  jq \
  htop \
  nodejs \
  npm \
  ufw \
  fail2ban \
  nginx \
  mysql-server \
  redis-server \
  php-fpm \
  php-cli \
  php-mysql \
  php-xml \
  php-mbstring \
  php-zip \
  php-curl \
  php-gd \
  certbot \
  python3-certbot-nginx

log "3) Enable service utama..."
systemctl enable --now nginx
systemctl enable --now mysql
systemctl enable --now redis-server

log "3a) Nonaktifkan default Nginx site (hindari bentrok vhost)..."
if [[ -f /etc/nginx/sites-available/default ]]; then
  cp /etc/nginx/sites-available/default "/etc/nginx/sites-available/default.bak.init.$(date +%Y%m%d%H%M%S)"
fi
rm -f /etc/nginx/sites-enabled/default || true

log "3b) Pasang catch-all default vhost (domain tanpa vhost → 503, bukan site lain)..."
NGINX_CATCHALL_SITE="/etc/nginx/sites-available/000-default-catchall.conf"
NGINX_CATCHALL_LINK="/etc/nginx/sites-enabled/000-default-catchall.conf"
NGINX_CATCHALL_SSL_DIR="/etc/nginx/ssl"
NGINX_CATCHALL_CERT="${NGINX_CATCHALL_SSL_DIR}/catchall-selfsigned.crt"
NGINX_CATCHALL_KEY="${NGINX_CATCHALL_SSL_DIR}/catchall-selfsigned.key"

nginx_version_ge() {
  local need="$1" cur
  cur="$(nginx -v 2>&1 | sed -n 's/.*nginx\/\([0-9.]*\).*/\1/p')"
  [[ -n "${cur}" ]] || return 1
  dpkg --compare-versions "${cur}" ge "${need}" 2>/dev/null
}

mkdir -p "${NGINX_CATCHALL_SSL_DIR}"
if [[ ! -f "${NGINX_CATCHALL_CERT}" || ! -f "${NGINX_CATCHALL_KEY}" ]]; then
  openssl req -x509 -nodes -days 3650 -newkey rsa:2048 \
    -keyout "${NGINX_CATCHALL_KEY}" \
    -out "${NGINX_CATCHALL_CERT}" \
    -subj "/CN=invalid.local/O=CatchAll/C=XX"
  chmod 600 "${NGINX_CATCHALL_KEY}"
  chmod 644 "${NGINX_CATCHALL_CERT}"
fi

if nginx_version_ge "1.19.4"; then
  cat >"${NGINX_CATCHALL_SITE}" <<'EOF'
# Domain yang belum punya vhost aplikasi — jangan proxy ke instance lain (hindari indeks SEO salah).
server {
    listen 80 default_server;
    listen [::]:80 default_server;
    server_name _;

    default_type text/plain;
    add_header X-Robots-Tag "noindex, nofollow, noarchive" always;
    add_header Retry-After "3600" always;
    return 503 "Service unavailable\n";
}

server {
    listen 443 ssl default_server;
    listen [::]:443 ssl default_server;
    server_name _;

    ssl_reject_handshake on;
}
EOF
else
  cat >"${NGINX_CATCHALL_SITE}" <<EOF
# Domain yang belum punya vhost aplikasi — jangan proxy ke instance lain (hindari indeks SEO salah).
server {
    listen 80 default_server;
    listen [::]:80 default_server;
    server_name _;

    default_type text/plain;
    add_header X-Robots-Tag "noindex, nofollow, noarchive" always;
    add_header Retry-After "3600" always;
    return 503 "Service unavailable\n";
}

server {
    listen 443 ssl default_server;
    listen [::]:443 ssl default_server;
    server_name _;

    ssl_certificate ${NGINX_CATCHALL_CERT};
    ssl_certificate_key ${NGINX_CATCHALL_KEY};

    default_type text/plain;
    add_header X-Robots-Tag "noindex, nofollow, noarchive" always;
    add_header Retry-After "3600" always;
    return 503 "Service unavailable\n";
}
EOF
fi

ln -sfn "${NGINX_CATCHALL_SITE}" "${NGINX_CATCHALL_LINK}"
nginx -t
systemctl reload nginx

log "4) Install Go ${GO_VERSION}..."
ARCH="$(dpkg --print-architecture)"
case "${ARCH}" in
  amd64) GO_ARCH="amd64" ;;
  arm64) GO_ARCH="arm64" ;;
  *)
    echo "Arsitektur ${ARCH} belum didukung otomatis untuk install Go."
    echo "Install Go manual: https://go.dev/dl/"
    exit 1
    ;;
esac

GO_TARBALL="go${GO_VERSION}.linux-${GO_ARCH}.tar.gz"
GO_URL="https://go.dev/dl/${GO_TARBALL}"
TMP_GO="/tmp/${GO_TARBALL}"

curl -fsSL "${GO_URL}" -o "${TMP_GO}"
rm -rf /usr/local/go
tar -C /usr/local -xzf "${TMP_GO}"
rm -f "${TMP_GO}"

cat >/etc/profile.d/golang.sh <<'EOF'
export PATH=$PATH:/usr/local/go/bin
EOF
chmod 644 /etc/profile.d/golang.sh
export PATH="$PATH:/usr/local/go/bin"

log "5) Setup firewall baseline..."
ufw allow OpenSSH || true
ufw allow 'Nginx Full' || true
if [[ "${INSTALL_PMA}" == "true" ]]; then
  ufw allow "${PMA_PORT}/tcp" || true
fi
ufw --force enable || true

log "6) Setup fail2ban..."
if [[ ! -f /etc/fail2ban/jail.local ]]; then
  cat >/etc/fail2ban/jail.local <<'EOF'
[DEFAULT]
bantime = 10m
findtime = 10m
maxretry = 5
backend = systemd
destemail = root@localhost
sender = fail2ban@localhost

[sshd]
enabled = true
EOF
fi
systemctl enable --now fail2ban

if [[ "${INSTALL_PMA}" == "true" ]]; then
  log "7) Install phpMyAdmin (manual + Nginx config)..."
  PMA_DIR="/usr/share/phpmyadmin"
  PMA_TMP="/tmp/phpmyadmin.tar.gz"

  rm -rf "${PMA_DIR}"
  mkdir -p "${PMA_DIR}"

  log "7.1) Ambil versi terbaru phpMyAdmin..."
  PMA_URL="$(curl --connect-timeout 10 --max-time 60 --retry 3 --retry-delay 2 -fsSL https://www.phpmyadmin.net/home_page/version.txt | awk 'NR==1{print "https://files.phpmyadmin.net/phpMyAdmin/"$1"/phpMyAdmin-"$1"-all-languages.tar.gz"}')"

  if [[ -z "${PMA_URL}" ]]; then
    echo "[ERROR] Gagal mendapatkan URL phpMyAdmin terbaru."
    exit 1
  fi

  log "7.2) Download phpMyAdmin dari: ${PMA_URL}"
  curl --connect-timeout 10 --max-time 300 --retry 3 --retry-delay 3 -fSL "${PMA_URL}" -o "${PMA_TMP}"
  log "7.3) Ekstrak archive phpMyAdmin..."
  tar -xzf "${PMA_TMP}" -C /tmp
  # Hindari exit karena SIGPIPE saat set -o pipefail aktif.
  mapfile -t PMA_ARCHIVE_LIST < <(tar -tzf "${PMA_TMP}")
  if [[ ${#PMA_ARCHIVE_LIST[@]} -eq 0 ]]; then
    echo "[ERROR] Archive phpMyAdmin kosong/tidak valid: ${PMA_TMP}"
    exit 1
  fi
  PMA_EXTRACTED_DIR="${PMA_ARCHIVE_LIST[0]%%/*}"
  if [[ -z "${PMA_EXTRACTED_DIR}" || ! -d "/tmp/${PMA_EXTRACTED_DIR}" ]]; then
    echo "[ERROR] Gagal mendeteksi folder hasil ekstrak phpMyAdmin."
    exit 1
  fi
  log "7.4) Salin file phpMyAdmin ke ${PMA_DIR}..."
  cp -r "/tmp/${PMA_EXTRACTED_DIR}/." "${PMA_DIR}/"
  rm -rf "${PMA_TMP}" "/tmp/${PMA_EXTRACTED_DIR}"

  mkdir -p /var/lib/phpmyadmin/tmp
  chown -R www-data:www-data /var/lib/phpmyadmin

  log "7.5) Generate config phpMyAdmin..."
  if [[ ! -f "${PMA_DIR}/config.inc.php" ]]; then
    BLOWFISH_SECRET="$(openssl rand -hex 16)"
    cat >"${PMA_DIR}/config.inc.php" <<EOF
<?php
\$cfg['blowfish_secret'] = '${BLOWFISH_SECRET}';
\$i = 0;
\$i++;
\$cfg['Servers'][\$i]['auth_type'] = 'cookie';
\$cfg['Servers'][\$i]['host'] = 'localhost';
\$cfg['Servers'][\$i]['compress'] = false;
\$cfg['Servers'][\$i]['AllowNoPassword'] = false;
\$cfg['TempDir'] = '/var/lib/phpmyadmin/tmp';
EOF
  fi

  PMA_HTPASSWD="/etc/nginx/.htpasswd-phpmyadmin"
  log "7.6) Setup basic auth phpMyAdmin..."
  htpasswd -bc "${PMA_HTPASSWD}" "${PMA_USER}" "${PMA_PASSWORD}"
  chmod 640 "${PMA_HTPASSWD}"
  chown root:www-data "${PMA_HTPASSWD}"

  if ! PHP_VERSION="$(detect_php_fpm_version)"; then
    echo "[ERROR] Versi PHP-FPM tidak terdeteksi. Cek dengan: systemctl list-unit-files | grep fpm"
    exit 1
  fi
  systemctl enable --now "php${PHP_VERSION}-fpm"
  log "7.6b) Naikkan batas upload PHP untuk import phpMyAdmin..."
  cat >"/etc/php/${PHP_VERSION}/fpm/conf.d/99-phpmyadmin-upload.ini" <<'EOF'
upload_max_filesize = 128M
post_max_size = 128M
memory_limit = 512M
max_execution_time = 300
max_input_time = 300
EOF
  PMA_NGINX_SITE="/etc/nginx/sites-available/phpmyadmin-${PMA_PORT}.conf"
  log "7.7) Tulis konfigurasi Nginx untuk phpMyAdmin port ${PMA_PORT}..."
  cat >"${PMA_NGINX_SITE}" <<EOF
server {
    listen ${PMA_PORT};
    listen [::]:${PMA_PORT};
    server_name _;

    auth_basic "Restricted phpMyAdmin";
    auth_basic_user_file ${PMA_HTPASSWD};

    root /usr/share/phpmyadmin;
    index index.php index.html index.htm;
    client_max_body_size 128m;

    location / {
        try_files \$uri \$uri/ /index.php?\$args;
    }

    location ~ \.php$ {
        include snippets/fastcgi-php.conf;
        fastcgi_pass unix:/run/php/php${PHP_VERSION}-fpm.sock;
        fastcgi_param SCRIPT_FILENAME \$document_root\$fastcgi_script_name;
        include fastcgi_params;
    }

    location ~* \.(jpg|jpeg|gif|css|png|js|ico|html|xml|txt|svg|woff|woff2)$ {
        expires 1d;
        access_log off;
    }
}
EOF
  ln -sfn "${PMA_NGINX_SITE}" "/etc/nginx/sites-enabled/phpmyadmin-${PMA_PORT}.conf"
  # Vhost lama tanpa nomor port ikut listen di port yang sama dan menang
  # karena namanya lebih dulu secara alfabet (`phpmyadmin` sebelum `phpmyadmin-51623.conf`).
  if [[ -e /etc/nginx/sites-enabled/phpmyadmin ]]; then
    log "7.7b) Nonaktifkan vhost phpMyAdmin lama supaya tidak bentrok di port ${PMA_PORT}..."
    rm -f /etc/nginx/sites-enabled/phpmyadmin
  fi

  log "7.8) Buat user MySQL untuk login phpMyAdmin..."
  mysql <<EOF
CREATE USER IF NOT EXISTS '${MYSQL_PMA_USER}'@'localhost' IDENTIFIED BY '${MYSQL_PMA_PASSWORD}';
GRANT ALL PRIVILEGES ON *.* TO '${MYSQL_PMA_USER}'@'localhost' WITH GRANT OPTION;
FLUSH PRIVILEGES;
EOF
fi

log "8) Validasi dan reload service..."
if PHP_FPM_VERSION="$(detect_php_fpm_version)"; then
  systemctl enable --now "php${PHP_FPM_VERSION}-fpm"
  systemctl restart "php${PHP_FPM_VERSION}-fpm"
else
  echo "[WARN] Service PHP-FPM tidak ditemukan. Cek unit dengan: systemctl list-unit-files | grep fpm"
fi
nginx -t
systemctl reload nginx

echo ""
echo "==============================================="
echo "Init server selesai (bootstrap only, belum menjalankan aplikasi bebii-blog)"
echo "==============================================="
echo "✅ Versi Go       : $(/usr/local/go/bin/go version || true)"
echo "✅ Nginx          : $(nginx -v 2>&1)"
echo "✅ Nginx catch-all: ${NGINX_CATCHALL_SITE} (domain tanpa vhost → 503 + noindex)"
echo "✅ MySQL          : $(mysql --version)"
echo "✅ Redis          : $(redis-server --version | head -n 1)"
if command -v node >/dev/null 2>&1 && command -v npm >/dev/null 2>&1; then
  echo "✅ Node.js        : $(node -v) / npm $(npm -v)"
else
  echo "⚠️  Node.js/npm    : tidak terdeteksi setelah install"
fi
if [[ "${INSTALL_PMA}" == "true" ]]; then
  echo ""
  echo "✅ phpMyAdmin (akses web)"
  echo "✅ - URL  : http://<ip-server>:${PMA_PORT}"
  echo "✅ - User : ${PMA_USER}"
  echo "✅ - Pass : ${PMA_PASSWORD}"
  echo ""
  echo "✅ phpMyAdmin (login MySQL di form login)"
  echo "✅ - User : ${MYSQL_PMA_USER}"
  echo "✅ - Pass : ${MYSQL_PMA_PASSWORD}"
fi
echo ""
