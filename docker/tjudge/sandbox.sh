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
# инфраструктурной ошибкой, а не ошибкой программы. Программа, которую не
# удалось скопировать в tmpfs, проигрывает с кодом своей стороны (1 или 2,
# как у tjudge-cli): иначе её матчи вечно возвращались бы в очередь.
#
# stderr бота tjudge-cli открывает как pipe и не читает: бот, написавший в
# него больше буфера pipe (64 КБ), зависал и проигрывал по таймауту. Поэтому
# лаунчер ещё от root направляет stderr бота в свой файл /programs/.bot<N>.err
# (0600, по пути боту недоступен, бот пишет в унаследованный дескриптор).
# Файлы бота ограничены err_limit: сверх него запись завершается ошибкой, а не
# убивает бота сигналом. Проигравшая по ошибке сторона (код 1 или 2) получает
# хвост своего stderr после сообщения tjudge-cli: он уходит в текст ошибки
# матча, а его видят только админ и команда этой программы.
#
# Программа /refbots/<игра> - эталонный бот игры из образа, против него
# executor гоняет самопроверку программы после сборки.
set -eEuo pipefail

fail() {
    echo "sandbox: $*" >&2
    exit 125
}
trap 'fail "line $LINENO: command failed"' ERR

copy_failed() {
    echo "sandbox: program $name: copy failed" >&2
    exit $((i + 1))
}

# две копии программ (до 32 МБ) и два файла по err_limit помещаются в tmpfs
# /programs (80 МБ, см. buildMatchHostConfig)
err_limit=$((4 << 20))

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

    src=/mnt/programs
    if [[ $prog == /refbots/* ]]; then
        src=/usr/local/lib/tjudge/refbots
        prog=/programs/${prog#/refbots/}
    fi
    name=${prog#/programs/}
    [[ $prog == /programs/$name && $name =~ ^[A-Za-z0-9_][A-Za-z0-9._-]*$ ]] || fail "bad program path: $prog"
    [[ -f $src/$name ]] || fail "program not mounted: $name"

    uid=$((base + i))
    cp "$src/$name" "$prog" || copy_failed
    chmod 0500 "$prog"
    if [[ -d $src/${name}_classes ]]; then
        cp -R "$src/${name}_classes" "${prog}_classes" || copy_failed
        chmod -R u=rX,go= "${prog}_classes"
        chown -R "$uid:$uid" "${prog}_classes"
    fi
    chown "$uid:$uid" "$prog"

    # после смены uid с root ядро сбрасывает capabilities, --inh-caps
    # добивает наследуемые, no-new-privileges не даёт получить их обратно
    drop=(setpriv --reuid="$uid" --regid="$uid" --clear-groups --inh-caps=-all)
    "${drop[@]}" test -x "$prog" || fail "program $name is not executable as uid $uid"
    launcher=/programs/.bot$i
    # >>: при игре с собой обе стороны пишут в файл первой.
    # ruby без RubyGems: с ним запуск занимает 50-400 мс и съедает 200 мс
    # судьи на первый ход; гемов в образе нет, стандартная библиотека работает
    printf '#!/bin/sh\numask 077\nulimit -f %d\ntrap "" XFSZ\nexport RUBYOPT=--disable-gems\nexec %s %s 2>>%s\n' \
        $((err_limit / 512)) "${drop[*]}" "$prog" "$launcher.err" >"$launcher"
    chmod 0700 "$launcher"
    bots+=("$launcher")
done

# без exec: после матча нужен stderr проигравшей стороны. сбой в дописывании
# хвоста не должен превращать итог матча в сбой песочницы.
# stderr tjudge-cli копит tail и отдаёт в лог контейнера одним куском: с -v
# там строка на каждый ход, и построчная запись в лог докера замедляла матч
# на 1000 итерациях на 15-20%. tail дочитывает поток до конца, так что судья
# не упрётся в закрытый pipe, а оставляет конец с итоговой строкой. fd 3 -
# stdout контейнера для счёта, ботам он не достаётся
code=0
{ tjudge-cli "${args[@]}" "${bots[@]}" 2>&1 >&3 3>&- | tail -c 512K >&2; } 3>&1 || code=$?
trap - ERR
set +e
if ((code == 1 || code == 2)); then
    err=/programs/.bot$((code - 1)).err
    [[ -e $err ]] || err=/programs/.bot0.err
    if [[ -s $err ]]; then
        echo "--- stderr программы (последние 2 КБ) ---" >&2
        tail -c 2048 "$err" >&2
        echo >&2
        if (($(stat -c %s "$err") >= err_limit)); then
            echo "--- stderr больше 4 МБ: запись сверх лимита завершалась ошибкой ---" >&2
        fi
    fi
fi
exit "$code"
