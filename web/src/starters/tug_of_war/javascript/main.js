#!/usr/bin/env node
// Перетягивание каната: стратегия «равная трата».
// Остаток энергии делится поровну на оставшиеся итерации, к концу тратится весь.
// Протокол: ← строка от судьи (stdin), → ваша строка (stdout).
// Отладку печатайте через console.error, stdout читает судья.
const readline = require("readline");

const rl = readline.createInterface({ input: process.stdin });
const init = [];                  // энергия и число итераций
let energy = 0;
let n = 0;
let played = 0;

// ход: остаток поровну на оставшиеся итерации
function move() {
  const spend = Math.floor(energy / (n - played));
  energy -= spend;
  console.log(spend);             // → трата; console.log сбрасывает вывод сам
}

rl.on("line", (line) => {
  if (init.length < 2) {
    init.push(Number(line));      // ← начальная энергия, затем число итераций
    if (init.length === 2) {
      [energy, n] = init;
      move();
    }
    return;
  }
  played++;                       // ← трата соперника
  if (played < n) move();
});
