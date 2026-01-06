-- Drop teams and team_members tables
DROP TRIGGER IF EXISTS update_teams_updated_at ON teams;
DROP TABLE IF EXISTS team_members;
DROP TABLE IF EXISTS teams;
DROP FUNCTION IF EXISTS generate_unique_code;
