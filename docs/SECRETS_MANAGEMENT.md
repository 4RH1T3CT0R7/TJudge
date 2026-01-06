# Управление секретами

## Обзор

TJudge поддерживает несколько способов управления секретами:

| Способ | Среда | Безопасность | Сложность |
|--------|-------|--------------|-----------|
| Переменные окружения | Development | Низкая | Простая |
| Docker Secrets | Docker Compose | Средняя | Средняя |
| Kubernetes Secrets | K8s | Средняя | Средняя |
| Sealed Secrets | K8s + Git | Высокая | Средняя |
| Encryption at Rest | K8s | Высокая | Сложная |

## Docker Compose (Development)

### Переменные окружения

```bash
# .env файл (НЕ коммитить в Git!)
DB_PASSWORD=your-password
JWT_SECRET=your-jwt-secret-minimum-32-characters
REDIS_PASSWORD=your-redis-password
```

### Docker Secrets (Production)

```bash
# Создание файлов секретов
mkdir -p secrets
echo "your-db-password" > secrets/db_password.txt
echo "your-jwt-secret-min-32-chars" > secrets/jwt_secret.txt
echo "your-redis-password" > secrets/redis_password.txt

# Защита файлов
chmod 600 secrets/*.txt
```

В `docker-compose.prod.yml`:
```yaml
services:
  api:
    secrets:
      - db_password
      - jwt_secret
      - redis_password

secrets:
  db_password:
    file: ./secrets/db_password.txt
  jwt_secret:
    file: ./secrets/jwt_secret.txt
  redis_password:
    file: ./secrets/redis_password.txt
```

## Kubernetes

### Базовые Secrets

```bash
# Создание (НЕ сохраняйте в Git!)
kubectl create secret generic tjudge-secrets \
  --from-literal=DB_PASSWORD=your-password \
  --from-literal=JWT_SECRET=your-jwt-secret \
  --from-literal=REDIS_PASSWORD=your-redis-password \
  -n tjudge
```

### Sealed Secrets (рекомендуется)

Sealed Secrets позволяет безопасно хранить секреты в Git.

#### Установка

```bash
# Контроллер
helm repo add sealed-secrets https://bitnami-labs.github.io/sealed-secrets
helm install sealed-secrets sealed-secrets/sealed-secrets -n kube-system

# CLI
brew install kubeseal  # macOS
```

#### Использование

```bash
# Генерация sealed secret
./scripts/seal-secrets.sh

# Применение
kubectl apply -f deployments/kubernetes/sealed-secret.yaml
```

Зашифрованный файл безопасен для хранения в Git — расшифровать его может только контроллер в вашем кластере.

### Encryption at Rest

Шифрование секретов в etcd на уровне kube-apiserver.

#### Настройка

1. Сгенерируйте ключ:
```bash
head -c 32 /dev/urandom | base64
```

2. Отредактируйте `encryption-config.yaml`:
```yaml
apiVersion: apiserver.config.k8s.io/v1
kind: EncryptionConfiguration
resources:
  - resources:
      - secrets
    providers:
      - aesgcm:
          keys:
            - name: key1
              secret: <ВСТАВЬТЕ_КЛЮЧ>
      - identity: {}
```

3. Скопируйте на master-ноды:
```bash
scp encryption-config.yaml master:/etc/kubernetes/
```

4. Обновите kube-apiserver manifest:
```yaml
# /etc/kubernetes/manifests/kube-apiserver.yaml
spec:
  containers:
  - command:
    - kube-apiserver
    - --encryption-provider-config=/etc/kubernetes/encryption-config.yaml
```

5. Перешифруйте существующие секреты:
```bash
kubectl get secrets -n tjudge -o json | kubectl replace -f -
```

## Ротация секретов

### JWT Secret

1. Добавьте новый ключ (старый сохраните для валидации существующих токенов)
2. Обновите конфигурацию
3. Дождитесь истечения старых токенов (15 мин по умолчанию)
4. Удалите старый ключ

### База данных

1. Создайте нового пользователя БД
2. Обновите секрет
3. Выполните rolling restart подов
4. Удалите старого пользователя

### Redis

1. Установите новый пароль через `CONFIG SET requirepass`
2. Обновите секрет
3. Выполните rolling restart подов

## Аудит

### Проверка доступа к секретам

```bash
# Kubernetes audit logs
kubectl logs -n kube-system -l component=kube-apiserver | grep secrets
```

### Проверка шифрования

```bash
# Проверка что секреты зашифрованы в etcd
ETCDCTL_API=3 etcdctl get /registry/secrets/tjudge/tjudge-secrets \
  --endpoints=https://127.0.0.1:2379 \
  --cacert=/etc/kubernetes/pki/etcd/ca.crt \
  --cert=/etc/kubernetes/pki/etcd/server.crt \
  --key=/etc/kubernetes/pki/etcd/server.key | hexdump -C | head
```

Если видите `k8s:enc:aesgcm:v1:key1` — секрет зашифрован.

## Рекомендации

1. **Никогда** не коммитьте незашифрованные секреты в Git
2. Используйте Sealed Secrets для GitOps
3. Включите encryption at rest в production
4. Регулярно ротируйте секреты
5. Ограничьте доступ через RBAC
6. Включите audit logging
