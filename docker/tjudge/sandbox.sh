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
#
# Пачка: sandbox --batch <лимит_с> [опции] -- <игра> <п1> <п2> [<игра> <п1> <п2>]...
# играет матчи по очереди в одном контейнере: создание и удаление контейнера
# стоит дороже самого матча. Каждый матч идёт как одиночный: свежие копии
# программ, своя пара uid. После матча процессы и файлы его ботов убираются
# (cleanup), иначе бот мог бы оставить следующему матчу состояние; SysV IPC и
# POSIX-очереди закрыты sysctl-ами контейнера. Итог матча - запись на stdout:
#   @@begin <номер> <статус> <код>
#   stdout судьи в base64
#   @@stderr
#   stderr матча в base64 (как у одиночного: судья и хвост stderr бота)
#   @@end
# Статус done - матч сыгран, код как у одиночного; timeout - матч не уложился в
# свой лимит; skipped - не сыгран (кончилось время пачки или не удалась
# уборка), его повторят. Время на всю пачку - лимит одного матча: первый матч
# получает его целиком, следующие - остаток. Ни один процесс от uid бота не
# получает stdout контейнера, а base64 не содержит @, так что подделать запись
# нельзя.
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

# prepare_bots <база uid> <программа1> <программа2> - копии программ под uid
# базы и базы+1, лаунчеры /programs/.bot0 и .bot1 в массив bots
prepare_bots() {
    local base=$1 i prog src name uid launcher drop
    local -a progs=("$2" "$3")
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
        # без дескрипторов: процесс от uid бота не получает ни вывода матча, ни stdout контейнера
        "${drop[@]}" test -x "$prog" </dev/null >/dev/null 2>&1 || fail "program $name is not executable as uid $uid"
        launcher=/programs/.bot$i
        # >>: при игре с собой обе стороны пишут в файл первой.
        # ruby без RubyGems: с ним запуск занимает 50-400 мс и съедает 200 мс
        # судьи на первый ход; гемов в образе нет, стандартная библиотека работает
        printf '#!/bin/sh\numask 077\nulimit -f %d\ntrap "" XFSZ\nexport RUBYOPT=--disable-gems\nexec %s %s 2>>%s\n' \
            $((err_limit / 512)) "${drop[*]}" "$prog" "$launcher.err" >"$launcher"
        chmod 0700 "$launcher"
        bots+=("$launcher")
    done
}

# play [команда-обёртка...] -- <аргументы tjudge-cli>: матч с ботами из bots.
# без exec: после матча нужен stderr проигравшей стороны. сбой в дописывании
# хвоста не должен превращать итог матча в сбой песочницы.
# stderr tjudge-cli копит tail и отдаёт одним куском: с -v там строка на
# каждый ход, и построчная запись в лог докера замедляла матч на 1000
# итерациях на 15-20%. tail дочитывает поток до конца, так что судья не
# упрётся в закрытый pipe, а оставляет конец с итоговой строкой. fd 3 -
# stdout для счёта, ботам он не достаётся
play() {
    local wrap=() code err
    while [[ $1 != -- ]]; do
        wrap+=("$1")
        shift
    done
    shift
    # код - судьи (или timeout над ним), а не tail: сбой записи хвоста не
    # должен превращаться в проигрыш программы
    trap - ERR
    set +e
    { "${wrap[@]}" tjudge-cli "$@" "${bots[@]}" 2>&1 >&3 3>&- | tail -c 512K >&2; code=${PIPESTATUS[0]}; } 3>&1
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
    return "$code"
}

# cleanup - убирает всё, что оставили боты матча. Работает root из основного
# шелла: встроенный kill -1 (без exec, бот не сорвёт его своим RLIMIT_NPROC)
# достаёт любой процесс контейнера, кроме init и самого шелла, в том числе
# отвязанный и неотслеживаемый (его /proc принадлежит root). Файлы в /tmp,
# /dev/shm и /programs root удаляет с DAC_OVERRIDE и FOWNER: каталоги бота без
# прав и его sticky-каталоги тоже. Ни одной команды от uid ботов: она
# унаследовала бы stdout контейнера, и выживший бот подделал бы через неё
# запись. 1 - если что-то осталось
cleanup() {
    local p try alive left
    for ((try = 0; try < 20; try++)); do
        kill -KILL -1 2>/dev/null
        alive=0
        # убитые исчезают из /proc, когда их подберёт init контейнера (этот шелл)
        for p in /proc/[0-9]*; do
            p=${p#/proc/}
            ((p == 1 || p == BASHPID)) || alive=1
        done
        ((alive == 0)) && break
        sleep 0.05
    done
    ((alive == 0)) || return 1
    shopt -s nullglob dotglob
    rm -rf /tmp/* /dev/shm/* /programs/*
    left=(/tmp/* /dev/shm/* /programs/*)
    shopt -u nullglob dotglob
    ((${#left[@]} == 0))
}

# итоги матча пачки: отдельный tmpfs только для root (см. batchHostConfig).
# на /programs и /tmp боты могут писать и забить их до отказа записи
results=/mnt/results

# emit <номер> <статус> <код> - запись итога матча пачки на stdout
emit() {
    echo "@@begin $1 $2 $3"
    [[ ! -f $results/out ]] || base64 "$results/out"
    echo "@@stderr"
    [[ ! -f $results/err ]] || base64 "$results/err"
    echo "@@end"
    rm -f "$results/out" "$results/err"
}

now_ms() {
    local t=${EPOCHREALTIME/[.,]/}
    echo $((t / 1000))
}

batch() {
    ((EUID == 0)) || fail "batch: needs root"
    local limit=$1 opts=() k n base deadline left lim code status t0 u0
    shift
    [[ $limit =~ ^[1-9][0-9]*$ ]] || fail "batch: bad limit: $limit"
    while (($#)) && [[ $1 != -- ]]; do
        opts+=("$1")
        shift
    done
    (($#)) || fail "batch: no -- before matches"
    shift
    (($# > 0 && $# % 3 == 0)) || fail "batch: matches must be <game> <program1> <program2>"
    n=$(($# / 3))
    local m=("$@")
    [[ -d $results ]] || fail "batch: $results not mounted"
    umask 077
    # своя пара uid на каждый матч пачки, все в диапазоне 20000-59999
    base=$((20000 + SRANDOM % (20000 - n) * 2))
    deadline=$(($(now_ms) + limit * 1000))
    for ((k = 0; k < n; k++)); do
        u0=$((base + 2 * k))
        left=$((deadline - $(now_ms)))
        if ((k > 0 && left < 1000)); then
            emit "$k" skipped 0
            continue
        fi
        lim=$((limit * 1000))
        ((k == 0 || left >= lim)) || lim=$left
        t0=$(now_ms)
        # матч в подшелле: exit из fail и copy_failed заканчивает только его.
        # снаружи ERR и -e сняты, иначе ненулевой код матча оборвал бы пачку
        trap - ERR
        set +e
        (
            trap 'fail "line $LINENO: command failed"' ERR
            set -e
            prepare_bots "$u0" "${m[3 * k + 1]}" "${m[3 * k + 2]}"
            # || exit: код программы (1, 2) - итог матча, а не повод для ERR
            play timeout -s KILL "$((lim / 1000)).$(printf %03d $((lim % 1000)))" -- "${m[3 * k]}" "${opts[@]}" || exit $?
        ) >"$results/out" 2>"$results/err"
        code=$?
        set -e
        trap 'fail "line $LINENO: command failed"' ERR
        status="done"
        # судью убил лимит: timeout с -s KILL добивает и себя, код 137 (124 -
        # без KILL). полный лимит - вина программы, урезанный остатком пачки -
        # нет, матч повторят
        if ((code == 124 || code == 137)) && (($(now_ms) - t0 >= lim - 100)); then
            status=timeout
            ((lim == limit * 1000)) || status=skipped
        fi
        if ! cleanup; then
            # уборка не удалась: этот матч сыгран, остальные в этот
            # контейнер не пускаются
            emit "$k" "$status" "$code"
            local j
            for ((j = k + 1; j < n; j++)); do
                emit "$j" skipped 0
            done
            exit 0
        fi
        emit "$k" "$status" "$code"
    done
    exit 0
}

if [[ ${1:-} == --batch ]]; then
    (($# >= 2)) || fail "usage: sandbox --batch <limit> [options] -- <game> <program1> <program2>..."
    batch "${@:2}"
fi

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
prepare_bots $((20000 + SRANDOM % 20000 * 2)) "${progs[@]}"
code=0
play -- "${args[@]}" || code=$?
exit "$code"
