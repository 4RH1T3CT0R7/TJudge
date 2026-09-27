#!/usr/bin/env php
<?php
// Дилемма путешественника: стратегия «заявка 90».
// Каждую итерацию одна и та же высокая заявка, но внутри границ [L, U].
// Протокол: ← строка от судьи (stdin), → ваша строка (stdout).
// Отладку печатайте в STDERR: fwrite(STDERR, ...), stdout читает судья.

$low = (int) fgets(STDIN);            // ← нижняя граница L
$high = (int) fgets(STDIN);           // ← верхняя граница U
$bonus = (int) fgets(STDIN);          // ← бонус и штраф R
$n = (int) fgets(STDIN);              // ← число итераций
$claim = min(max(90, $low), $high);   // 90, если оно внутри [L, U]
for ($i = 0; $i < $n; $i++) {
    echo $claim, "\n";                // → заявка: целое число от L до U
    flush();                          // сброс вывода
    $opponent = (int) fgets(STDIN);   // ← заявка соперника
}
