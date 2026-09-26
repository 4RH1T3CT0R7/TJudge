#!/bin/sh
# рабочий конфиг собирается из alertmanager.yml: с TELEGRAM_BOT_TOKEN и
# TELEGRAM_CHAT_ID дописывается получатель telegram и становится основным.
# в отслеживаемых файлах ничего править не нужно
set -eu

cfg=/tmp/alertmanager.yml
cp /etc/alertmanager/alertmanager.yml "$cfg"

if [ -n "${TELEGRAM_BOT_TOKEN:-}" ] && [ -n "${TELEGRAM_CHAT_ID:-}" ]; then
    printf '%s' "$TELEGRAM_BOT_TOKEN" > /tmp/telegram_token
    sed -i "s/^  receiver: 'null'$/  receiver: telegram/" "$cfg"
    cat >> "$cfg" <<CONFIG
  - name: telegram
    telegram_configs:
      - bot_token_file: /tmp/telegram_token
        chat_id: $TELEGRAM_CHAT_ID
        parse_mode: HTML
        message: |-
          {{ if eq .Status "firing" }}🔥{{ else }}✅{{ end }} <b>{{ .CommonLabels.alertname }}</b> [{{ .CommonLabels.severity }}]
          {{ range .Alerts }}{{ .Annotations.summary }}
          {{ if .Annotations.description }}{{ .Annotations.description }}{{ end }}
          {{ end }}
CONFIG
fi

exec /bin/alertmanager --config.file="$cfg" --storage.path=/alertmanager
