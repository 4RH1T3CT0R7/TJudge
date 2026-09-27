#!/usr/bin/env lua
-- Дилемма заключённого: стратегия «око за око» (tit for tat).
-- Первый ход - сотрудничество, дальше повтор прошлого хода соперника.
-- Протокол: ← строка от судьи (stdin), → ваша строка (stdout).
-- Отладку печатайте в io.stderr, stdout читает судья.

local n = tonumber(io.read("l"))  -- ← число итераций
local move = "COOPERATE"
for _ = 1, n do
  io.write(move, "\n")            -- → ваш ход
  io.stdout:flush()               -- сброс вывода обязателен
  move = io.read("l")             -- ← ход соперника
end
