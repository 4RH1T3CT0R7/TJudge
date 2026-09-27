#!/usr/bin/env lua
-- Общественное благо: стратегия «условный вклад».
-- Сначала вложить всё, дальше столько же, сколько соперник вложил в прошлый раз.
-- Протокол: ← строка от судьи (stdin), → ваша строка (stdout).
-- Отладку печатайте в io.stderr, stdout читает судья.

local capital = tonumber(io.read("l"))     -- ← капитал на итерацию E
local multiplier = tonumber(io.read("l"))  -- ← множитель пула m, дробное число
local n = tonumber(io.read("l"))           -- ← число итераций
local give = capital                       -- первый вклад - весь капитал
for _ = 1, n do
  io.write(give, "\n")                     -- → вклад: целое число от 0 до E
  io.stdout:flush()                        -- сброс вывода обязателен
  give = tonumber(io.read("l"))            -- ← вклад соперника
end
