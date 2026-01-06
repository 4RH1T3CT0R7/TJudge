#!/bin/bash
# Скрипт для создания Sealed Secrets
# Требует установленный kubeseal и доступ к кластеру с Sealed Secrets контроллером

set -e

NAMESPACE="${NAMESPACE:-tjudge}"
SECRET_NAME="${SECRET_NAME:-tjudge-secrets}"
OUTPUT_FILE="${OUTPUT_FILE:-deployments/kubernetes/sealed-secret.yaml}"

echo "=== TJudge Sealed Secrets Generator ==="
echo "Namespace: $NAMESPACE"
echo "Secret name: $SECRET_NAME"
echo ""

# Проверка kubeseal
if ! command -v kubeseal &> /dev/null; then
    echo "Ошибка: kubeseal не установлен"
    echo "Установка:"
    echo "  macOS: brew install kubeseal"
    echo "  Linux: wget https://github.com/bitnami-labs/sealed-secrets/releases/download/v0.24.0/kubeseal-0.24.0-linux-amd64.tar.gz"
    exit 1
fi

# Запрос значений
echo "Введите значения секретов (ввод скрыт):"
echo ""

read -sp "DB_PASSWORD: " DB_PASSWORD
echo ""

read -sp "JWT_SECRET (минимум 32 символа): " JWT_SECRET
echo ""

# Проверка длины JWT_SECRET
if [ ${#JWT_SECRET} -lt 32 ]; then
    echo "Ошибка: JWT_SECRET должен быть минимум 32 символа"
    exit 1
fi

read -sp "REDIS_PASSWORD: " REDIS_PASSWORD
echo ""
echo ""

# Создание и шифрование секрета
echo "Создание sealed secret..."

kubectl create secret generic "$SECRET_NAME" \
    --namespace="$NAMESPACE" \
    --from-literal=DB_PASSWORD="$DB_PASSWORD" \
    --from-literal=JWT_SECRET="$JWT_SECRET" \
    --from-literal=REDIS_PASSWORD="$REDIS_PASSWORD" \
    --dry-run=client -o yaml | \
    kubeseal --format yaml > "$OUTPUT_FILE"

echo ""
echo "✓ Sealed secret создан: $OUTPUT_FILE"
echo ""
echo "Применение:"
echo "  kubectl apply -f $OUTPUT_FILE"
echo ""
echo "ВАЖНО: Этот файл безопасен для хранения в Git!"
