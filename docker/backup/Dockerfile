# P1.14: контейнер для периодического pg_dump.
# Запускает cron внутри alpine с установленным postgresql-client.
FROM alpine:3.20

RUN apk --no-cache add postgresql16-client bash ca-certificates gzip tzdata

# Создаём папку для backup-ов (маппится volume-ом из compose).
RUN mkdir -p /backups && chmod 755 /backups

COPY entrypoint.sh /entrypoint.sh
COPY backup.sh    /usr/local/bin/tjudge-backup.sh
RUN chmod +x /entrypoint.sh /usr/local/bin/tjudge-backup.sh

# Non-root для минимизации blast-radius. UID 1000 совпадает с appuser
# в других контейнерах проекта.
RUN addgroup -g 1000 backup && adduser -D -u 1000 -G backup backup && \
    chown -R backup:backup /backups
USER backup

ENTRYPOINT ["/entrypoint.sh"]
