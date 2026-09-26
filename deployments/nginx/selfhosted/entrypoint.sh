#!/bin/sh
# /docker-entrypoint.d образа nginx. конфиг выбирается по сертификату DOMAIN:
# без него сайт идёт по http (и certbot может пройти acme-challenge), с ним -
# редирект на https. фоновая проверка раз в 5 минут замечает выпуск и
# продление сертификата certbot'ом и перечитывает конфиг
set -eu

cert="/etc/letsencrypt/live/$DOMAIN/fullchain.pem"

render() {
    tpl=http.conf
    [ -f "$cert" ] && tpl=https.conf
    # shellcheck disable=SC2016 # подставляется только DOMAIN, $host и прочие остаются nginx
    envsubst '$DOMAIN' < "/etc/nginx/tjudge/$tpl" > /etc/nginx/conf.d/default.conf
}

# live/<домен>/fullchain.pem - ссылка на archive/, при продлении она меняется
cert_version() { readlink -f "$cert" 2>/dev/null || true; }

render
(
    seen=$(cert_version)
    while sleep 300; do
        now=$(cert_version)
        if [ "$now" != "$seen" ]; then
            render
            nginx -s reload || true
            seen=$now
        fi
    done
) &
