#!/usr/bin/env python3
"""Simple defector - always outputs DEFECT"""
import sys

while True:
    try:
        line = input()  # Read opponent's move (or empty on first round)
        print("DEFECT")
        sys.stdout.flush()
    except EOFError:
        break
