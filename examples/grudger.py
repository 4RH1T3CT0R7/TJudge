#!/usr/bin/env python3
"""
Grudger Strategy (Grim Trigger) - Prisoner's Dilemma

A strict punishment-based strategy:
  1. Start by cooperating
  2. Continue cooperating as long as opponent cooperates
  3. If opponent EVER defects, defect forever (hold a grudge)

This strategy is:
  - Nice (never defects first)
  - Unforgiving (one defection = permanent retaliation)
  - Effective against exploiters
  - Vulnerable to noise/mistakes (one error = mutual destruction)

Best against: Always-Defect, random strategies
Weak against: Tit-for-Tat with occasional mistakes
"""
import sys


def main():
    betrayed = False

    while True:
        try:
            line = input().strip()

            # Check if opponent ever defected
            if line.upper() == "DEFECT":
                betrayed = True

            # Cooperate until betrayed, then defect forever
            if betrayed:
                my_move = "DEFECT"
            else:
                my_move = "COOPERATE"

            print(my_move)
            sys.stdout.flush()

        except EOFError:
            break


if __name__ == "__main__":
    main()
