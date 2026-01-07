# Управление секретами

## Обзор

TJudge поддерживает несколько способов управления секретами:

| Способ | Среда | Безопасность | Сложность |
|--------|-------|--------------|-----------|
| Переменные окружения | Development | Низкая | Простая |
| Docker Secrets | Production | Средняя | Средняя |

## Development (Переменные окружения)

```bash
# .env файл (НЕ коммитить в Git!)
DB_PASSWORD=your-password
JWT_SECRET=your-jwt-secret-minimum-32-characters
REDIS_PASSWORD=your-redis-password
```

```bash
# Запуск с .env
docker-compose up -d
```

## Production (Docker Secrets)

### 1. Создание секретов

```bash
# Создание файлов секретов
mkdir -p secrets
echo "your-db-password" > secrets/db_password.txt
echo "your-jwt-secret-min-32-chars" > secrets/jwt_secret.txt
echo "your-redis-password" > secrets/redis_password.txt

# Защита файлов
chmod 600 secrets/*.txt
```

### 2. Конфигурация docker-compose.prod.yml

```yaml
services:
  api:
    secrets:
      - db_password
      - jwt_secret
      - redis_password
    environment:
      - DB_PASSWORD_FILE=/run/secrets/db_password
      - JWT_SECRET_FILE=/run/secrets/jwt_secret
      - REDIS_PASSWORD_FILE=/run/secrets/redis_password

secrets:
  db_password:
    file: ./secrets/db_password.txt
  jwt_secret:
    file: ./secrets/jwt_secret.txt
  redis_password:
    file: ./secrets/redis_password.txt
```

### 3. Чтение секретов в приложении

Приложение автоматически определяет источник секретов:
- Если установлена переменная `*_FILE` - читает из файла
- Иначе использует значение переменной напрямую

## Ротация секретов

### JWT Secret

1. Добавьте новый ключ в конфигурацию
2. Дождитесь истечения старых токенов (15 мин по умолчанию)
3. Удалите старый ключ

### База данных

1. Создайте нового пользователя БД
2. Обновите файл секрета
3. Перезапустите контейнеры: `docker-compose restart api worker`
4. Удалите старого пользователя

### Redis

1. Установите новый пароль: `CONFIG SET requirepass newpassword`
2. Обновите файл секрета
3. Перезапустите контейнеры

## Рекомендации

1. **Никогда** не коммитьте секреты в Git
2. Добавьте `secrets/` в `.gitignore`
3. Используйте Docker Secrets в production
4. Регулярно ротируйте секреты
5. Ограничьте доступ к файлам секретов (chmod 600)
