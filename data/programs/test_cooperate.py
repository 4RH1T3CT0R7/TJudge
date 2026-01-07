#!/usr/bin/env python3
"""Simple cooperator - always outputs COOPERATE"""
import sys

while True:
    try:
        line = input()  # Read opponent's move (or empty on first round)
        print("COOPERATE")
        sys.stdout.flush()
    except EOFError:
        break
