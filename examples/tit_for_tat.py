#!/usr/bin/env python3
"""
Tit-for-Tat Strategy (Prisoner's Dilemma)

The most successful strategy in Axelrod's famous tournaments.
Simple rules:
  1. First move: COOPERATE
  2. After that: copy opponent's previous move

This strategy is:
  - Nice (never defects first)
  - Retaliatory (punishes defection immediately)
  - Forgiving (returns to cooperation if opponent cooperates)
  - Clear (easy to understand for opponents)
"""
import sys


def main():
    opponent_last_move = None

    while True:
        try:
            line = input().strip()

            # Parse opponent's move (empty on first round)
            if line:
                opponent_last_move = line.upper()

            # First move: always cooperate
            # After that: copy opponent's last move
            if opponent_last_move is None:
                my_move = "COOPERATE"
            elif opponent_last_move == "DEFECT":
                my_move = "DEFECT"
            else:
                my_move = "COOPERATE"

            print(my_move)
            sys.stdout.flush()

        except EOFError:
            break


if __name__ == "__main__":
    main()
