#!/bin/bash
# Generates a temporary self-signed certificate so nginx can start
# and serve ACME challenges on port 80 before real certs are obtained.
#
# Usage: ./scripts/init-ssl.sh <domain>
# Then:  docker compose -f docker-compose.selfhosted.yml restart nginx
# Then:  docker run --rm -v tjudge_certbot_webroot:/var/www/certbot -v tjudge_certbot_certs:/etc/letsencrypt certbot/certbot certonly --webroot -w /var/www/certbot -d <domain> --agree-tos --register-unsafely-without-email --non-interactive
# Then:  docker compose -f docker-compose.selfhosted.yml restart nginx

set -e

DOMAIN="${1:?Usage: $0 <domain>}"

docker volume create tjudge_certbot_certs 2>/dev/null || true

docker run --rm \
  -v tjudge_certbot_certs:/etc/letsencrypt \
  alpine/openssl req -x509 -nodes -days 1 \
  -newkey rsa:2048 \
  -keyout "/etc/letsencrypt/live/${DOMAIN}/privkey.pem" \
  -out "/etc/letsencrypt/live/${DOMAIN}/fullchain.pem" \
  -subj "/CN=${DOMAIN}"

echo "Temporary self-signed certificate created for ${DOMAIN}"
echo "Now restart nginx and run certbot to get a real certificate."
