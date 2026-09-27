#!/usr/bin/env node
// Общественное благо: стратегия «условный вклад».
// Сначала вложить всё, дальше столько же, сколько соперник вложил в прошлый раз.
// Протокол: ← строка от судьи (stdin), → ваша строка (stdout).
// Отладку печатайте через console.error, stdout читает судья.
const readline = require("readline");

const rl = readline.createInterface({ input: process.stdin });
const init = [];                  // E, m и число итераций
let played = 0;

rl.on("line", (line) => {
  if (init.length < 3) {
    init.push(Number(line));      // ← капитал E, множитель m (дробный), число итераций
    if (init.length === 3) {
      console.log(init[0]);       // → весь капитал; console.log сбрасывает вывод сам
    }
    return;
  }
  played++;                       // ← вклад соперника
  if (played < init[2]) {
    console.log(Number(line));    // → вклад: столько же, сколько соперник
  }
});
