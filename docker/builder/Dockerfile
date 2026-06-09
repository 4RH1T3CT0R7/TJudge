# tjudge-builder: песочница компиляции загруженных программ.
#
# Worker запускает этот образ для каждой задачи компиляции:
#   - network none, CapDrop ALL, readonly rootfs, tmpfs /tmp;
#   - монтируется ТОЛЬКО каталог сборки конкретной программы (/build, rw);
#   - команда формируется в internal/infrastructure/executor/compiler.go
#     (buildCompilePlan) - компиляторы и syntax-check интерпретаторов.
#
# Сборка: make docker-build-builder
# Конфиг: EXECUTOR_BUILDER_IMAGE (по умолчанию tjudge-builder:latest).

FROM alpine:3.20

# Компиляторы: gcc/g++ (c, cpp), go, rust, openjdk (java).
# Интерпретаторы для проверки синтаксиса: python3, nodejs, ruby, php, lua.
RUN apk --no-cache add \
    gcc g++ musl-dev \
    go \
    rust \
    openjdk17-jdk \
    python3 \
    nodejs \
    ruby \
    php83 \
    lua5.4 \
    && ln -sf /usr/bin/php83 /usr/local/bin/php \
    && ln -sf /usr/bin/luac5.4 /usr/local/bin/luac \
    && ln -sf /usr/bin/lua5.4 /usr/local/bin/lua

# Непривилегированный пользователь: компиляция не должна идти от root
# даже внутри песочницы.
RUN addgroup -g 1000 builder && \
    adduser -D -u 1000 -G builder builder

USER builder

# Команду задаёт worker; образ без entrypoint-логики.
CMD ["true"]
