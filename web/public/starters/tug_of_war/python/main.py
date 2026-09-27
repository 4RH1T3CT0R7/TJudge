#!/usr/bin/env python3
# Перетягивание каната: стратегия «равная трата».
# Остаток энергии делится поровну на оставшиеся итерации, к концу тратится весь.
# Протокол: ← строка от судьи (stdin), → ваша строка (stdout).
# Отладку печатайте в stderr: print(..., file=sys.stderr), stdout читает судья.

energy = int(input())             # ← начальная энергия
n = int(input())                  # ← число итераций
for i in range(n):
    spend = energy // (n - i)     # поровну на оставшиеся итерации
    energy -= spend
    print(spend, flush=True)      # → сколько потратить: от 0 до остатка; flush=True обязателен
    opponent = int(input())       # ← сколько потратил соперник
