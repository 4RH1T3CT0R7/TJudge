# Периодический pg_dump и архив каталога программ (entrypoint.sh).
# Клиент той же мажорной версии, что сервер (postgres:15-alpine в compose):
# дамп более нового pg_dump в старый сервер не восстанавливается (17-й пишет
# SET transaction_timeout, которого нет в 15). Поднимается вместе с сервером.
FROM postgres:15-alpine

RUN apk --no-cache add ca-certificates curl

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# uid 1000 как у api и worker: читает файлы программ и пишет в /backups
RUN addgroup -g 1000 backup && adduser -D -u 1000 -G backup backup && \
    mkdir -p /backups && chown backup:backup /backups
USER backup

STOPSIGNAL SIGTERM
ENTRYPOINT ["/entrypoint.sh"]
CMD []
