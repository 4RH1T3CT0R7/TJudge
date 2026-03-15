-- Create test tournament with 2 users, 2 teams, 3 games
-- Users: user2 / Password123!  and  user3 / Password123!

BEGIN;

-- Users
INSERT INTO users (id, username, email, password_hash, role) VALUES
  ('aaaaaaaa-0000-0000-0000-000000000002', 'user2', 'user2@test.local', '$2a$10$t3XJu1C20DzJ.gcD6krB6OICBRR8HlgEk8R2o8lWGDWDp3WDAtFmy', 'user'),
  ('aaaaaaaa-0000-0000-0000-000000000003', 'user3', 'user3@test.local', '$2a$10$t3XJu1C20DzJ.gcD6krB6OICBRR8HlgEk8R2o8lWGDWDp3WDAtFmy', 'user')
ON CONFLICT (username) DO NOTHING;

-- Tournament
INSERT INTO tournaments (id, name, description, game_type, status, max_participants, max_team_size, code, creator_id)
VALUES (
  'bbbbbbbb-0000-0000-0000-000000000001',
  'Тестовый турнир',
  'Турнир для тестирования. 3 игры: Дилемма заключённого, Перетягивание каната, Дилемма путешественника.',
  'dilemma',
  'pending',
  100,
  3,
  'TEST01',
  'aaaaaaaa-0000-0000-0000-000000000002'
) ON CONFLICT DO NOTHING;

-- Add 3 games to tournament
INSERT INTO tournament_games (tournament_id, game_id, is_active) VALUES
  ('bbbbbbbb-0000-0000-0000-000000000001', '6d60eca1-1a58-4c1c-8c37-60afa39c69d1', true),
  ('bbbbbbbb-0000-0000-0000-000000000001', '79d53fba-6490-4dd4-888a-4d28fb7a77ea', false),
  ('bbbbbbbb-0000-0000-0000-000000000001', 'c320a53f-8a0a-40c8-9d94-66c9cf55a9d7', false)
ON CONFLICT DO NOTHING;

-- Team 1 (user2 captain)
INSERT INTO teams (id, tournament_id, name, code, leader_id) VALUES
  ('cccccccc-0000-0000-0000-000000000001', 'bbbbbbbb-0000-0000-0000-000000000001', 'Тестовая Альфа', 'TALPHA', 'aaaaaaaa-0000-0000-0000-000000000002')
ON CONFLICT DO NOTHING;
INSERT INTO team_members (team_id, user_id) VALUES
  ('cccccccc-0000-0000-0000-000000000001', 'aaaaaaaa-0000-0000-0000-000000000002')
ON CONFLICT DO NOTHING;

-- Team 2 (user3 captain)
INSERT INTO teams (id, tournament_id, name, code, leader_id) VALUES
  ('cccccccc-0000-0000-0000-000000000002', 'bbbbbbbb-0000-0000-0000-000000000001', 'Тестовая Бета', 'TBETAA', 'aaaaaaaa-0000-0000-0000-000000000003')
ON CONFLICT DO NOTHING;
INSERT INTO team_members (team_id, user_id) VALUES
  ('cccccccc-0000-0000-0000-000000000002', 'aaaaaaaa-0000-0000-0000-000000000003')
ON CONFLICT DO NOTHING;

COMMIT;

-- Verify
SELECT 'Users:' AS info, COUNT(*) FROM users WHERE username IN ('user2', 'user3');
SELECT 'Tournament:' AS info, id, name, status FROM tournaments WHERE id = 'bbbbbbbb-0000-0000-0000-000000000001';
SELECT 'Teams:' AS info, COUNT(*) FROM teams WHERE tournament_id = 'bbbbbbbb-0000-0000-0000-000000000001';
SELECT 'Games:' AS info, COUNT(*) FROM tournament_games WHERE tournament_id = 'bbbbbbbb-0000-0000-0000-000000000001';
