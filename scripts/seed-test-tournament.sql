-- Create test tournament with 2 users, 2 teams, 3 games
-- Users: user2 / Password123!  and  user3 / Password123!

BEGIN;

-- Users (upsert password if exist)
INSERT INTO users (username, email, password_hash, role) VALUES
  ('user2', 'user2@test.local', '$2a$10$t3XJu1C20DzJ.gcD6krB6OICBRR8HlgEk8R2o8lWGDWDp3WDAtFmy', 'user'),
  ('user3', 'user3@test.local', '$2a$10$t3XJu1C20DzJ.gcD6krB6OICBRR8HlgEk8R2o8lWGDWDp3WDAtFmy', 'user')
ON CONFLICT (username) DO UPDATE SET password_hash = EXCLUDED.password_hash;

-- Tournament
DO $$ DECLARE u2id UUID; t_id UUID;
BEGIN
  SELECT id INTO u2id FROM users WHERE username = 'user2';
  INSERT INTO tournaments (id, name, description, game_type, status, max_participants, max_team_size, code, creator_id)
  VALUES (
    'bbbbbbbb-0000-0000-0000-000000000001',
    'Тестовый турнир',
    'Турнир для тестирования. 3 игры.',
    'dilemma', 'pending', 100, 3, 'TEST01', u2id
  ) ON CONFLICT DO NOTHING;
  t_id := 'bbbbbbbb-0000-0000-0000-000000000001';

  -- Games
  INSERT INTO tournament_games (tournament_id, game_id, is_active) VALUES
    (t_id, '6d60eca1-1a58-4c1c-8c37-60afa39c69d1', true),
    (t_id, '79d53fba-6490-4dd4-888a-4d28fb7a77ea', false),
    (t_id, 'c320a53f-8a0a-40c8-9d94-66c9cf55a9d7', false)
  ON CONFLICT DO NOTHING;

  -- Team 1
  INSERT INTO teams (tournament_id, name, code, leader_id)
  VALUES (t_id, 'Тестовая Альфа', 'TALPHA', u2id)
  ON CONFLICT DO NOTHING;
  INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u2id FROM teams t WHERE t.name = 'Тестовая Альфа' AND t.tournament_id = t_id
  ON CONFLICT DO NOTHING;

  -- Team 2
  INSERT INTO teams (tournament_id, name, code, leader_id)
  VALUES (t_id, 'Тестовая Бета', 'TBETAA', (SELECT id FROM users WHERE username = 'user3'))
  ON CONFLICT DO NOTHING;
  INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u WHERE t.name = 'Тестовая Бета' AND t.tournament_id = t_id AND u.username = 'user3'
  ON CONFLICT DO NOTHING;
END $$;

COMMIT;

-- Verify
SELECT 'Users:' AS info, COUNT(*) FROM users WHERE username IN ('user2', 'user3');
SELECT 'Tournament:' AS info, id, name, status FROM tournaments WHERE id = 'bbbbbbbb-0000-0000-0000-000000000001';
SELECT 'Teams:' AS info, COUNT(*) FROM teams WHERE tournament_id = 'bbbbbbbb-0000-0000-0000-000000000001';
SELECT 'Games:' AS info, COUNT(*) FROM tournament_games WHERE tournament_id = 'bbbbbbbb-0000-0000-0000-000000000001';
