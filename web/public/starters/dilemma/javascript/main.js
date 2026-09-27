#!/usr/bin/env node
// Дилемма заключённого: стратегия «око за око» (tit for tat).
// Первый ход - сотрудничество, дальше повтор прошлого хода соперника.
// Протокол: ← строка от судьи (stdin), → ваша строка (stdout).
// Отладку печатайте через console.error, stdout читает судья.
const readline = require("readline");

const rl = readline.createInterface({ input: process.stdin });
let n = null;
let played = 0;

rl.on("line", (line) => {
  if (n === null) {
    n = Number(line);             // ← число итераций
    console.log("COOPERATE");     // → первый ход; console.log сбрасывает вывод сам
    return;
  }
  played++;                       // ← ход соперника
  if (played < n) {
    console.log(line.trim());     // → ответить тем же
  }
});
