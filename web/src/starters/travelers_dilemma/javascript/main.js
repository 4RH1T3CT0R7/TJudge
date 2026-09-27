#!/usr/bin/env node
// Дилемма путешественника: стратегия «заявка 90».
// Каждую итерацию одна и та же высокая заявка, но внутри границ [L, U].
// Протокол: ← строка от судьи (stdin), → ваша строка (stdout).
// Отладку печатайте через console.error, stdout читает судья.
const readline = require("readline");

const rl = readline.createInterface({ input: process.stdin });
const init = [];                  // L, U, R и число итераций
let claim = 0;
let played = 0;

rl.on("line", (line) => {
  if (init.length < 4) {
    init.push(Number(line));      // ← границы L и U, бонус R, затем число итераций
    if (init.length === 4) {
      const [low, high] = init;
      claim = Math.min(Math.max(90, low), high);  // 90, если оно внутри [L, U]
      console.log(claim);         // → заявка; console.log сбрасывает вывод сам
    }
    return;
  }
  played++;                       // ← заявка соперника
  if (played < init[3]) console.log(claim);  // → та же заявка
});
