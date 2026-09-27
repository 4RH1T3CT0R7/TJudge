#!/usr/bin/env lua
-- Дилемма путешественника: стратегия «заявка 90».
-- Каждую итерацию одна и та же высокая заявка, но внутри границ [L, U].
-- Протокол: ← строка от судьи (stdin), → ваша строка (stdout).
-- Отладку печатайте в io.stderr, stdout читает судья.

local low = tonumber(io.read("l"))     -- ← нижняя граница L
local high = tonumber(io.read("l"))    -- ← верхняя граница U
local bonus = tonumber(io.read("l"))   -- ← бонус и штраф R
local n = tonumber(io.read("l"))       -- ← число итераций
local claim = math.min(math.max(90, low), high)  -- 90, если оно внутри [L, U]
for _ = 1, n do
  io.write(claim, "\n")                -- → заявка: целое число от L до U
  io.stdout:flush()                    -- сброс вывода обязателен
  local opponent = tonumber(io.read("l"))  -- ← заявка соперника
end
