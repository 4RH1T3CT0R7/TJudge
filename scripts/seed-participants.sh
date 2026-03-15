#!/bin/bash
set -euo pipefail

# Usage: ./scripts/seed-participants.sh <tournament_id>
# Run on server: sudo docker compose -f docker-compose.selfhosted.yml exec postgres psql -U tjudge -d tjudge -f /tmp/seed.sql
# Or pipe: cat scripts/seed-participants.sql | sudo docker compose -f docker-compose.selfhosted.yml exec -T postgres psql -U tjudge -d tjudge

TOURNAMENT_ID="${1:?Usage: $0 <tournament_id>}"
DEFAULT_PASSWORD='$2a$10$LJJxZJ0Q5YKq0Q5YKq0Q5OuZJ0Q5YKq0Q5YKq0Q5YKq0Q5YKq0Q5O'

# Generate bcrypt hash for default password "GameTheory2026"
# On server: htpasswd -nbBC 10 "" "GameTheory2026" | tr -d ':\n' | sed 's/$2y/$2a/'
# Or use python3: python3 -c "import bcrypt; print(bcrypt.hashpw(b'GameTheory2026', bcrypt.gensalt(10)).decode())"

cat << EOSQL
-- Seed participants from CSV for tournament: $TOURNAMENT_ID
-- Default password: GameTheory2026
-- Generated: $(date)

BEGIN;

-- Password hash for "GameTheory2026" (bcrypt)
-- Generate on server: python3 -c "import bcrypt; print(bcrypt.hashpw(b'GameTheory2026', bcrypt.gensalt(10)).decode())"
-- Then replace the hash below

DO \$\$
DECLARE
  default_hash TEXT := '\$2a\$10\$rKxO5V5V5V5V5V5V5V5V5OuKxO5V5V5V5V5V5V5V5V5V5V5V5V5O'; -- REPLACE WITH REAL HASH
  t_id UUID := '$TOURNAMENT_ID';
  team_uuid UUID;
  user_uuid UUID;
BEGIN

-- ==========================================
-- Team 1: M.O.S.C.O.W.
-- ==========================================
team_uuid := gen_random_uuid();
-- Create captain
INSERT INTO users (id, username, email, password_hash, role) VALUES
  (gen_random_uuid(), 'Kirill_Kudryashov', 'kskudryashov_1@edu.hse.ru', default_hash, 'user')
  ON CONFLICT (username) DO NOTHING;
SELECT id INTO user_uuid FROM users WHERE username = 'Kirill_Kudryashov';
INSERT INTO teams (id, tournament_id, name, code, leader_id) VALUES
  (team_uuid, t_id, 'M.O.S.C.O.W.', 'MOSCW1', user_uuid)
  ON CONFLICT DO NOTHING;
-- Members
INSERT INTO users (id, username, email, password_hash, role) VALUES
  (gen_random_uuid(), 'teraqqq', 'SRBabin@yandex.ru', default_hash, 'user') ON CONFLICT (username) DO NOTHING;
INSERT INTO team_members (team_id, user_id) SELECT team_uuid, id FROM users WHERE username = 'teraqqq' ON CONFLICT DO NOTHING;
INSERT INTO users (id, username, email, password_hash, role) VALUES
  (gen_random_uuid(), 'jurijo', 'y.feddorov@gmail.com', default_hash, 'user') ON CONFLICT (username) DO NOTHING;
INSERT INTO team_members (team_id, user_id) SELECT team_uuid, id FROM users WHERE username = 'jurijo' ON CONFLICT DO NOTHING;
INSERT INTO team_members (team_id, user_id) SELECT team_uuid, id FROM users WHERE username = 'Kirill_Kudryashov' ON CONFLICT DO NOTHING;

EOSQL

# Parse CSV and generate SQL for remaining teams
python3 << 'PYEOF'
import csv
import sys

tournament_id = sys.argv[1] if len(sys.argv) > 1 else "REPLACE_ME"

with open("Таблица-data-2026-03-15 10_11_26.csv", "r") as f:
    reader = csv.DictReader(f)
    teams = {}
    for row in reader:
        tid = row["team_id"]
        if tid not in teams:
            teams[tid] = {"name": row["team_name"], "members": [], "captain": None}
        member = {"username": row["username"], "email": row["email"], "is_captain": row["is_captain"] == "true"}
        teams[tid]["members"].append(member)
        if member["is_captain"]:
            teams[tid]["captain"] = member

    for i, (tid, team) in enumerate(teams.items(), 1):
        captain = team["captain"]
        if not captain:
            captain = team["members"][0]

        team_code = f"T{i:02d}{tid[:4].upper()}"
        safe_name = team["name"].replace("'", "''")

        print(f"""
-- ==========================================
-- Team {i}: {team['name']}
-- ==========================================
team_uuid := gen_random_uuid();
INSERT INTO users (id, username, email, password_hash, role) VALUES
  (gen_random_uuid(), '{captain["username"]}', '{captain["email"]}', default_hash, 'user') ON CONFLICT (username) DO NOTHING;
SELECT id INTO user_uuid FROM users WHERE username = '{captain["username"]}';
INSERT INTO teams (id, tournament_id, name, code, leader_id) VALUES
  (team_uuid, t_id, '{safe_name}', '{team_code}', user_uuid) ON CONFLICT DO NOTHING;""")

        for m in team["members"]:
            if m["username"] != captain["username"]:
                print(f"""INSERT INTO users (id, username, email, password_hash, role) VALUES
  (gen_random_uuid(), '{m["username"]}', '{m["email"]}', default_hash, 'user') ON CONFLICT (username) DO NOTHING;""")
            print(f"""INSERT INTO team_members (team_id, user_id) SELECT team_uuid, id FROM users WHERE username = '{m["username"]}' ON CONFLICT DO NOTHING;""")

PYEOF

cat << 'EOSQL'

END;
$$ LANGUAGE plpgsql;

COMMIT;
EOSQL
