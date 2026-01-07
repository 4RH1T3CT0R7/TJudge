#!/usr/bin/env python3
"""
Стратегия: Tit-for-Tat с прощением
Начинает с сотрудничества, затем копирует предыдущий ход противника.
С вероятностью 10% прощает предательство.
"""
import sys
import random

random.seed()
first_round = True

while True:
    try:
        line = input()

        if first_round:
            # Первый раунд - сотрудничаем
            print("COOPERATE")
            first_round = False
        elif line == "COOPERATE":
            print("COOPERATE")
        else:
            # С вероятностью 10% прощаем предательство
            if random.random() < 0.10:
                print("COOPERATE")
            else:
                print("DEFECT")

        sys.stdout.flush()
    except EOFError:
        break
