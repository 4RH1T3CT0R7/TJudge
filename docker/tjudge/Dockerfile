# Multi-stage build для tjudge-cli
# Stage 1: Сборка Rust приложения (Debian-based — musl/Alpine падает с SIGBUS на Apple Silicon)
FROM rust:1.85-bookworm AS builder

RUN apt-get update && apt-get install -y --no-install-recommends git musl-tools && rm -rf /var/lib/apt/lists/*

# Добавляем musl target для статической линковки (совместимость с Alpine runtime)
RUN rustup target add $(uname -m)-unknown-linux-musl

# tjudge-cli на зафиксированном коммите: в нём правила и подсчёт очков игр,
# сборка из HEAD меняла бы их между релизами. Обновляется отдельным коммитом.
ARG TJUDGE_CLI_REF=600879b5e81f86cdd06b877227e2c1ef9e6dca1c
WORKDIR /build
RUN git init -q . \
    && git remote add origin https://github.com/bmstu-itstech/tjudge-cli.git \
    && git fetch -q --depth 1 origin ${TJUDGE_CLI_REF} \
    && git checkout -q FETCH_HEAD

# Собираем статический бинарник для musl (работает на Alpine)
RUN cargo build --release --target $(uname -m)-unknown-linux-musl \
    && cp target/$(uname -m)-unknown-linux-musl/release/tjudge-cli /usr/local/bin/tjudge-cli

# Stage 2: Runtime образ с поддержкой всех языков
FROM alpine:3.23

# Устанавливаем интерпретаторы и компиляторы для всех поддерживаемых языков
RUN apk add --no-cache \
    # Python
    python3 \
    # JavaScript/TypeScript (Node.js)
    nodejs \
    # C/C++ компиляторы и runtime
    gcc \
    g++ \
    musl-dev \
    libstdc++ \
    # Java (OpenJDK runtime для Java и Kotlin)
    openjdk17-jre-headless \
    # Ruby
    ruby \
    # PHP
    php83 \
    # Lua
    lua5.4 \
    # Go runtime
    go \
    # Bash для скриптов
    bash \
    # Базовые утилиты
    coreutils

# Создаём симлинки для удобства
RUN ln -sf /usr/bin/python3 /usr/bin/python && \
    ln -sf /usr/bin/lua5.4 /usr/bin/lua && \
    ln -sf /usr/bin/php83 /usr/bin/php

# Добавляем непривилегированного пользователя
RUN adduser -D -H -s /sbin/nologin tjudge

# Копируем бинарник tjudge-cli
COPY --from=builder /usr/local/bin/tjudge-cli /usr/local/bin/tjudge-cli

# Делаем бинарник исполняемым
RUN chmod +x /usr/local/bin/tjudge-cli

# Создаём рабочую директорию для программ
WORKDIR /programs

# Переключаемся на непривилегированного пользователя
USER tjudge

# Точка входа
ENTRYPOINT ["tjudge-cli"]
