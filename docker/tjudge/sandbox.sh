#!/bin/bash
# Точка входа матч-контейнера: разводит две программы по разным uid и
# запускает tjudge-cli.
#
# Аргументы как у tjudge-cli: <игра> [опции] <программа1> <программа2>, где
# программа - путь /programs/<имя>. Файлы программ (у java ещё <имя>_classes)
# примонтированы в /mnt/programs, куда пускают только root. Скрипт стартует
# от root с CHOWN, DAC_OVERRIDE, SETUID, SETGID и KILL (см.
# buildMatchHostConfig в internal/executor), копирует каждую программу в tmpfs
# /programs с владельцем - своим uid и правами 0500 и отдаёт tjudge-cli вместо
# неё лаунчер, который сбрасывает root до этого uid. Бот остаётся без
# capabilities и не может ни прочитать соперника, ни послать сигнал или
# ptrace ему и tjudge-cli. KILL нужен tjudge-cli, чтобы в конце матча добить
# ботов под чужими uid.
#
# Код выхода 125 - сбой подготовки песочницы, executor считает его
# инфраструктурной ошибкой, а не ошибкой программы.
set -eEuo pipefail

fail() {
    echo "sandbox: $*" >&2
    exit 125
}
trap 'fail "line $LINENO: command failed"' ERR

(($# >= 3)) || fail "usage: sandbox <game> [options] <program1> <program2>"
args=("${@:1:$#-2}")
progs=("${@: -2}")

# старый executor (до разведения ботов по uid) запускает образ от tjudge без
# capabilities и монтирует программы прямо в /programs. матч тогда идёт как
# раньше, под одним uid: образ, обновлённый раньше воркера, не должен
# проваливать матчи. от root этой ветки нет
if ((EUID != 0)) && [[ -e ${progs[0]} ]]; then
    exec tjudge-cli "$@"
fi

# пара uid своя на каждый матч: RLIMIT_NPROC считается на uid по всему хосту,
# и с общим uid параллельные матчи отнимали бы друг у друга процессы
base=$((20000 + SRANDOM % 20000 * 2))

bots=()
for i in 0 1; do
    prog=${progs[i]}
    # одна программа с обеих сторон играет сама с собой под одним uid
    if ((i == 1)) && [[ $prog == "${progs[0]}" ]]; then
        bots+=("${bots[0]}")
        continue
    fi

    name=${prog#/programs/}
    [[ $prog == /programs/$name && $name =~ ^[A-Za-z0-9_][A-Za-z0-9._-]*$ ]] || fail "bad program path: $prog"
    [[ -f /mnt/programs/$name ]] || fail "program not mounted: $name"

    uid=$((base + i))
    cp /mnt/programs/"$name" "$prog"
    chmod 0500 "$prog"
    if [[ -d /mnt/programs/${name}_classes ]]; then
        cp -R /mnt/programs/"${name}_classes" "${prog}_classes"
        chmod -R u=rX,go= "${prog}_classes"
        chown -R "$uid:$uid" "${prog}_classes"
    fi
    chown "$uid:$uid" "$prog"

    # после смены uid с root ядро сбрасывает capabilities, --inh-caps
    # добивает наследуемые, no-new-privileges не даёт получить их обратно
    drop=(setpriv --reuid="$uid" --regid="$uid" --clear-groups --inh-caps=-all)
    "${drop[@]}" test -x "$prog" || fail "program $name is not executable as uid $uid"
    launcher=/programs/.bot$i
    printf '#!/bin/sh\nexec %s %s\n' "${drop[*]}" "$prog" >"$launcher"
    chmod 0700 "$launcher"
    bots+=("$launcher")
done

exec tjudge-cli "${args[@]}" "${bots[@]}"
