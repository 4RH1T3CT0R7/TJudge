#!/bin/bash
# Script to upload tug_of_war programs for all teams in a tournament
# Usage: ./upload_tug_of_war.sh <tournament_id>

set -e

API_BASE="http://localhost:8080/api/v1"
TOURNAMENT_ID="${1:-9e0baeb4-060a-4ffa-8382-2b8d18fa3b69}"

echo "Uploading tug_of_war programs for tournament: $TOURNAMENT_ID"

# Get tug_of_war game ID
TUG_OF_WAR_GAME_ID=$(curl -s "$API_BASE/games/name/tug_of_war" | jq -r '.id')
if [ -z "$TUG_OF_WAR_GAME_ID" ] || [ "$TUG_OF_WAR_GAME_ID" == "null" ]; then
    echo "Error: tug_of_war game not found"
    exit 1
fi
echo "Tug of War game ID: $TUG_OF_WAR_GAME_ID"

# Get teams in tournament
TEAMS=$(curl -s "$API_BASE/tournaments/$TOURNAMENT_ID/teams")
TEAM_COUNT=$(echo "$TEAMS" | jq 'length')
echo "Found $TEAM_COUNT teams"

# Define strategies (Python programs for tug_of_war)
declare -a STRATEGIES
STRATEGIES[0]='#!/usr/bin/python3
m = int(input())
remaining = 100
for i in range(m):
    rounds_left = m - i
    spend = min(remaining, remaining // rounds_left) if rounds_left > 0 else remaining
    remaining -= spend
    print(max(0, spend), flush=True)
    opp = int(input())
    if opp < 0:
        break
'

STRATEGIES[1]='#!/usr/bin/python3
m = int(input())
remaining = 100
for i in range(m):
    spend = min(remaining, 20)  # Conservative: spend max 20 per round
    remaining -= spend
    print(spend, flush=True)
    opp = int(input())
    if opp < 0:
        break
'

STRATEGIES[2]='#!/usr/bin/python3
import random
m = int(input())
remaining = 100
for i in range(m):
    if remaining <= 0:
        print(0, flush=True)
    else:
        rounds_left = m - i
        avg = remaining // max(1, rounds_left)
        spend = min(remaining, random.randint(max(0, avg - 5), avg + 5))
        remaining -= spend
        print(spend, flush=True)
    opp = int(input())
    if opp < 0:
        break
'

STRATEGIES[3]='#!/usr/bin/python3
m = int(input())
remaining = 100
# Front-loaded: spend more early
weights = [3, 2.5, 2, 1.5, 1, 0.5, 0.3, 0.2, 0.1, 0.1]
for i in range(m):
    if remaining <= 0:
        print(0, flush=True)
    else:
        w = weights[i] if i < len(weights) else 0.1
        total_weight = sum(weights[j] if j < len(weights) else 0.1 for j in range(i, m))
        spend = int(remaining * w / max(0.1, total_weight))
        spend = max(0, min(remaining, spend))
        remaining -= spend
        print(spend, flush=True)
    opp = int(input())
    if opp < 0:
        break
'

echo "Defined ${#STRATEGIES[@]} strategies"

# For each team, we need to login and upload
# Since we don't have credentials, we'll use admin account
# First, let's check if admin is available

echo ""
echo "Note: This script requires manual execution in the performance test framework"
echo "or admin credentials to upload on behalf of teams."
echo ""
echo "To upload programs, run the Go test instead:"
echo "  go test -v -tags=performance ./tests/performance/... -run TestUploadTugOfWar -timeout 5m -count=1"
