#!/bin/bash
# Генерирует secrets/*.txt для docker secrets (prod, self-hosted, мониторинг).
# Существующие файлы не перезаписываются.
# Usage: ./scripts/init-secrets.sh

set -e

cd "$(dirname "$0")/.."
SECRETS_DIR="./secrets"

mkdir -p "$SECRETS_DIR"
chmod 700 "$SECRETS_DIR"

generate_password() {
    local length=${1:-32}
    openssl rand -base64 48 | tr -d '/+=' | head -c "$length"
}

create_secret() {
    local name=$1
    local length=${2:-32}
    local file="$SECRETS_DIR/${name}.txt"

    if [ -f "$file" ]; then
        echo "Secret '$name' already exists, skipping..."
    else
        echo "Generating secret '$name' (${length} chars)..."
        generate_password "$length" > "$file"
    fi
    # compose монтирует файл в контейнер как есть, а сервисы в контейнерах
    # работают не от владельца файла. от чужих закрыт каталог (700), не файл
    chmod 644 "$file"
}

create_secret "db_password"
create_secret "jwt_secret" 48
create_secret "redis_password"
create_secret "grafana_admin_password"
