# Multi-stage build для tjudge-cli
# Stage 1: Сборка Rust приложения
FROM rust:1.75-alpine AS builder

# Устанавливаем необходимые зависимости для сборки
RUN apk add --no-cache musl-dev git

# Добавляем target для статической сборки
RUN rustup target add x86_64-unknown-linux-musl

# Клонируем tjudge-cli
WORKDIR /build
RUN git clone --depth 1 https://github.com/bmstu-itstech/tjudge-cli.git .

# Собираем в release режиме со статической линковкой
RUN cargo build --release --target x86_64-unknown-linux-musl

# Stage 2: Минимальный runtime образ
FROM alpine:3.19

# Добавляем непривилегированного пользователя
RUN adduser -D -H -s /sbin/nologin tjudge

# Копируем бинарник
COPY --from=builder /build/target/x86_64-unknown-linux-musl/release/tjudge-cli /usr/local/bin/tjudge-cli

# Делаем бинарник исполняемым
RUN chmod +x /usr/local/bin/tjudge-cli

# Создаём рабочую директорию для программ
WORKDIR /programs

# Переключаемся на непривилегированного пользователя
USER tjudge

# Точка входа
ENTRYPOINT ["tjudge-cli"]
