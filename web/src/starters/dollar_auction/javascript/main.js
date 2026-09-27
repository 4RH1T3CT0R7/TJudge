#!/usr/bin/env node
// Аукцион двойной цены: стратегия «пас после порога».
// Перебивать соперника на 1, пока ставка не выше половины приза, дальше пас.
// Протокол: ← строка от судьи (stdin), → ваша строка (stdout).
// Ходы поочерёдные: сначала ставка соперника, потом ваш ответ. О пасе соперника
// судья не сообщает: ввод просто заканчивается.
// Отладку печатайте через console.error, stdout читает судья.
const readline = require("readline");

const rl = readline.createInterface({ input: process.stdin });
const init = [];                  // P и максимум итераций

rl.on("line", (line) => {
  if (init.length < 2) {
    init.push(Number(line));      // ← приз P, затем максимум итераций
    return;
  }
  const limit = Math.floor(init[0] / 2);  // дороже половины приза не торговаться
  // ← ставка соперника, 0 в начале торгов
  let bid = Number(line) + 1;     // перебить на 1
  if (bid > limit) bid = 0;       // 0 = пас, торги заканчиваются
  // console.log сбрасывает вывод сам
  console.log(bid);               // → ставка выше соперника или 0
});
