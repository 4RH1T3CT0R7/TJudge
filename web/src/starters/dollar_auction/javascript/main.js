#!/usr/bin/env node
// Аукцион двойной цены: стратегия «пас после порога».
// Перебивать ставку соперника на 1, пока своя ставка не выше половины приза, дальше пас.
// Протокол: ← строка от судьи (stdin), → ваша строка (stdout).
// Здесь ходы поочерёдные: сначала приходит ставка соперника, потом ваш ответ.
// Отладку печатайте через console.error, stdout читает судья.
const readline = require("readline");

const rl = readline.createInterface({ input: process.stdin });
const init = [];                  // P и максимум раундов

rl.on("line", (line) => {
  if (init.length < 2) {
    init.push(Number(line));      // ← стоимость приза P, затем максимум раундов торгов
    return;
  }
  const limit = Math.floor(init[0] / 2);  // дороже половины приза не торговаться
  let bid = Number(line) + 1;     // ← ставка соперника (0 в начале торгов); перебить на 1
  if (bid > limit) bid = 0;       // 0 = пас, торги заканчиваются
  console.log(bid);               // → ставка: больше ставки соперника или 0; console.log сбрасывает вывод сам
});
