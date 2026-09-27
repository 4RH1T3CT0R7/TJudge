#!/usr/bin/env python3
# Дилемма заключённого: стратегия «око за око» (tit for tat).
# Первый ход - сотрудничество, дальше повтор прошлого хода соперника.
# Протокол: ← строка от судьи (stdin), → ваша строка (stdout).
# Отладку печатайте в stderr: print(..., file=sys.stderr), stdout читает судья.

n = int(input())                  # ← число итераций
move = "COOPERATE"
for _ in range(n):
    print(move, flush=True)       # → ваш ход; flush=True обязателен
    move = input()                # ← ход соперника
