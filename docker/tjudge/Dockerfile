# Multi-stage build для tjudge-cli
# Stage 1: Сборка Rust приложения
FROM rust:1.85-alpine AS builder

# Устанавливаем необходимые зависимости для сборки
RUN apk add --no-cache musl-dev git

# Клонируем tjudge-cli
WORKDIR /build
RUN git clone --depth 1 https://github.com/bmstu-itstech/tjudge-cli.git .

# Собираем в release режиме (для native архитектуры)
RUN cargo build --release

# Stage 2: Runtime образ с поддержкой всех языков
FROM alpine:3.19

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
COPY --from=builder /build/target/release/tjudge-cli /usr/local/bin/tjudge-cli

# Делаем бинарник исполняемым
RUN chmod +x /usr/local/bin/tjudge-cli

# Создаём рабочую директорию для программ
WORKDIR /programs

# Переключаемся на непривилегированного пользователя
USER tjudge

# Точка входа
ENTRYPOINT ["tjudge-cli"]
