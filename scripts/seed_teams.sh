#!/bin/bash
# Скрипт: добавляет 2 команды в турнир, user2 в одну, user3 в другую
# Использование: ./scripts/seed_teams.sh

API="https://bcgx.itsbmstu.ru/api/v1"
TOURNAMENT_ID="71bb2d79-b47e-4ee7-b68e-bbcac20aa6a5"

# Пароли пользователей (измени если другие)
USER2_USERNAME="user2"
USER2_PASSWORD="Password123!"
USER3_USERNAME="user3"
USER3_PASSWORD="Password123!"

TEAM1_NAME="Альфа"
TEAM2_NAME="Бета"

set -e

echo "=== Логин user2 ==="
TOKEN2=$(curl -s -X POST "$API/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"username\":\"$USER2_USERNAME\",\"password\":\"$USER2_PASSWORD\"}" \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['access_token'])")

if [ -z "$TOKEN2" ]; then
  echo "ОШИБКА: не удалось залогинить user2"
  exit 1
fi
echo "OK: token получен"

echo ""
echo "=== Логин user3 ==="
TOKEN3=$(curl -s -X POST "$API/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"username\":\"$USER3_USERNAME\",\"password\":\"$USER3_PASSWORD\"}" \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['access_token'])")

if [ -z "$TOKEN3" ]; then
  echo "ОШИБКА: не удалось залогинить user3"
  exit 1
fi
echo "OK: token получен"

echo ""
echo "=== Создание команды '$TEAM1_NAME' (user2) ==="
TEAM1=$(curl -s -X POST "$API/teams" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN2" \
  -d "{\"tournament_id\":\"$TOURNAMENT_ID\",\"name\":\"$TEAM1_NAME\"}")
echo "$TEAM1" | python3 -m json.tool 2>/dev/null || echo "$TEAM1"

echo ""
echo "=== Создание команды '$TEAM2_NAME' (user3) ==="
TEAM2=$(curl -s -X POST "$API/teams" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN3" \
  -d "{\"tournament_id\":\"$TOURNAMENT_ID\",\"name\":\"$TEAM2_NAME\"}")
echo "$TEAM2" | python3 -m json.tool 2>/dev/null || echo "$TEAM2"

echo ""
echo "=== Готово ==="
