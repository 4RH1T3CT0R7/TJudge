#!/usr/bin/env python3
# Дилемма путешественника: стратегия «заявка 90».
# Каждую итерацию одна и та же высокая заявка, но внутри границ [L, U].
# Протокол: ← строка от судьи (stdin), → ваша строка (stdout).
# Отладку печатайте в stderr: print(..., file=sys.stderr), stdout читает судья.

low = int(input())                # ← нижняя граница L
high = int(input())               # ← верхняя граница U
bonus = int(input())              # ← бонус и штраф R
n = int(input())                  # ← число итераций
claim = min(max(90, low), high)   # 90, если оно внутри [L, U]
for _ in range(n):
    print(claim, flush=True)      # → заявка: целое число от L до U; flush=True обязателен
    opponent = int(input())       # ← заявка соперника
