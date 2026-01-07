#!/usr/bin/env python3
"""
Стратегия: Адаптивная с анализом поведения
- Отслеживает частоту предательств противника
- >= 60% сотрудничества -> Tit-for-Tat
- >= 70% предательств -> всегда предаём
- Иначе Pavlov (Win-Stay, Lose-Shift)
"""
import sys

opponent_history = []
my_history = []
first_round = True

while True:
    try:
        line = input()

        if first_round:
            # Первый ход - сотрудничаем
            my_move = "COOPERATE"
            print(my_move)
            sys.stdout.flush()
            my_history.append(my_move)
            first_round = False
            continue

        opponent_history.append(line)
        total_moves = len(opponent_history)
        defect_count = opponent_history.count("DEFECT")
        defect_rate = defect_count / total_moves if total_moves > 0 else 0

        if total_moves < 5:
            # В начале - Tit-for-Tat
            my_move = line
        elif defect_rate >= 0.70:
            # Агрессивный противник - всегда предаём
            my_move = "DEFECT"
        elif defect_rate <= 0.40:
            # Кооперативный противник - Tit-for-Tat
            my_move = line
        else:
            # Pavlov: если оба сделали одинаково - повторяем свой ход
            if my_history and line == my_history[-1]:
                my_move = my_history[-1]
            else:
                my_move = "DEFECT" if my_history and my_history[-1] == "COOPERATE" else "COOPERATE"

        print(my_move)
        sys.stdout.flush()
        my_history.append(my_move)

    except EOFError:
        break
