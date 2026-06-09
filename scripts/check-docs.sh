#!/bin/bash
set -euo pipefail

# TJudge: проверка заземляемых фактов документации.
#
# Документация - основной инструмент рычага solo-разработчика и AI-агентов,
# и она имеет свойство тихо отставать от кода. Скрипт проверяет факты,
# которые можно проверить автоматически. Запускается в CI (job lint).

cd "$(dirname "$0")/.."

FAIL=0
err() { echo "::error::DOC-CHECK: $1"; FAIL=1; }
ok()  { echo "  OK: $1"; }

# 1. Диапазон миграций в CLAUDE.md соответствует migrations/.
max_migration=$(ls migrations/ | grep -oE '^[0-9]{6}' | sort -u | tail -1)
if grep -q "000001\`-\`${max_migration}" CLAUDE.md; then
    ok "CLAUDE.md: диапазон миграций до ${max_migration}"
else
    err "CLAUDE.md заявляет не тот диапазон миграций (фактический максимум: ${max_migration})"
fi

# 2. Версия Go в README не отстаёт от go.mod.
gomod_minor=$(grep -E '^go [0-9]+\.[0-9]+' go.mod | grep -oE '[0-9]+\.[0-9]+' | head -1)
if grep -qE "Go-${gomod_minor%%.*}\.[0-9]+" README.md || grep -q "Go ${gomod_minor}" README.md || grep -q "Go-${gomod_minor}" README.md; then
    ok "README.md: версия Go согласована с go.mod (${gomod_minor})"
else
    err "README.md упоминает не ту версию Go (go.mod: ${gomod_minor})"
fi

# 3. Файлы, на которые ссылаются доки, существуют.
for f in docs/ROADMAP.md deployments/prometheus/prometheus.yml deployments/prometheus/alerts deployments/alertmanager/alertmanager.yml docs/openapi.yaml; do
    if [ -e "$f" ]; then
        ok "существует: $f"
    else
        err "файл/каталог отсутствует, но на него ссылаются доки или compose: $f"
    fi
done

# 4. docker-compose монтирует конфиги из deployments/ - они обязаны быть в репо.
#    Runtime-каталоги (./data, ./backups) и секреты (telegram_token,
#    в .gitignore) создаются при деплое и не проверяются.
for compose in docker-compose.yml docker-compose.selfhosted.yml; do
    [ -f "$compose" ] || continue
    compose_ok=1
    while IFS= read -r path; do
        case "$path" in
            *telegram_token*) continue ;;
        esac
        if [ ! -e "$path" ]; then
            err "$compose монтирует несуществующий конфиг: $path"
            compose_ok=0
        fi
    done < <(grep -oE '\./deployments/[^:]+' "$compose" | sort -u)
    [ "$compose_ok" -eq 1 ] && ok "$compose: все mounts из deployments/ существуют"
done

if [ "$FAIL" -ne 0 ]; then
    echo ""
    echo "Документация разошлась с кодом - поправьте доки или скрипт."
    exit 1
fi

echo ""
echo "Все проверки документации прошли."
