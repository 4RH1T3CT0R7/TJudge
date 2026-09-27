#!/usr/bin/env lua
-- Перетягивание каната: стратегия «равная трата».
-- Остаток энергии делится поровну на оставшиеся итерации, к концу тратится весь.
-- Протокол: ← строка от судьи (stdin), → ваша строка (stdout).
-- Отладку печатайте в io.stderr, stdout читает судья.

local energy = tonumber(io.read("l"))  -- ← начальная энергия
local n = tonumber(io.read("l"))       -- ← число итераций
for i = 0, n - 1 do
  local spend = energy // (n - i)      -- поровну на оставшиеся итерации
  energy = energy - spend
  io.write(spend, "\n")                -- → трата: от 0 до остатка
  io.stdout:flush()                    -- сброс вывода обязателен
  local opponent = tonumber(io.read("l"))  -- ← трата соперника
end
