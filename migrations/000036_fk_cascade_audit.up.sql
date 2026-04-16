-- P2.9: приводим недостающие ON DELETE политики к явным значениям.
--
-- Было (implicit default NO ACTION == RESTRICT, но неявно):
--   matches.program1_id, matches.program2_id       → programs(id)
--   teams.leader_id                                → users(id)
--   tournaments.creator_id                         → users(id)
--   programs.game_id                               → games(id)
--
-- Политика:
--   - matches.program*_id     → CASCADE (матч теряет смысл без программы)
--   - teams.leader_id         → RESTRICT (удаление user'а forbidden пока он leader;
--                                         нужно сначала передать leadership)
--   - tournaments.creator_id  → SET NULL (турнир остаётся)
--   - programs.game_id        → RESTRICT (нельзя удалить игру, на которой есть программы)

-- matches.program1_id
DO $$
DECLARE
    cname text;
BEGIN
    SELECT conname INTO cname
      FROM pg_constraint
     WHERE conrelid = 'matches'::regclass
       AND conkey = ARRAY[(
           SELECT attnum FROM pg_attribute
            WHERE attrelid = 'matches'::regclass AND attname = 'program1_id'
       )];
    IF cname IS NOT NULL THEN
        EXECUTE format('ALTER TABLE matches DROP CONSTRAINT %I', cname);
    END IF;
    ALTER TABLE matches
        ADD CONSTRAINT matches_program1_id_fkey
        FOREIGN KEY (program1_id) REFERENCES programs(id) ON DELETE CASCADE;
END $$;

-- matches.program2_id
DO $$
DECLARE
    cname text;
BEGIN
    SELECT conname INTO cname
      FROM pg_constraint
     WHERE conrelid = 'matches'::regclass
       AND conkey = ARRAY[(
           SELECT attnum FROM pg_attribute
            WHERE attrelid = 'matches'::regclass AND attname = 'program2_id'
       )];
    IF cname IS NOT NULL THEN
        EXECUTE format('ALTER TABLE matches DROP CONSTRAINT %I', cname);
    END IF;
    ALTER TABLE matches
        ADD CONSTRAINT matches_program2_id_fkey
        FOREIGN KEY (program2_id) REFERENCES programs(id) ON DELETE CASCADE;
END $$;

-- teams.leader_id
DO $$
DECLARE
    cname text;
BEGIN
    SELECT conname INTO cname
      FROM pg_constraint
     WHERE conrelid = 'teams'::regclass
       AND conkey = ARRAY[(
           SELECT attnum FROM pg_attribute
            WHERE attrelid = 'teams'::regclass AND attname = 'leader_id'
       )];
    IF cname IS NOT NULL THEN
        EXECUTE format('ALTER TABLE teams DROP CONSTRAINT %I', cname);
    END IF;
    ALTER TABLE teams
        ADD CONSTRAINT teams_leader_id_fkey
        FOREIGN KEY (leader_id) REFERENCES users(id) ON DELETE RESTRICT;
END $$;

-- tournaments.creator_id
DO $$
DECLARE
    cname text;
BEGIN
    SELECT conname INTO cname
      FROM pg_constraint
     WHERE conrelid = 'tournaments'::regclass
       AND conkey = ARRAY[(
           SELECT attnum FROM pg_attribute
            WHERE attrelid = 'tournaments'::regclass AND attname = 'creator_id'
       )];
    IF cname IS NOT NULL THEN
        EXECUTE format('ALTER TABLE tournaments DROP CONSTRAINT %I', cname);
    END IF;
    ALTER TABLE tournaments
        ADD CONSTRAINT tournaments_creator_id_fkey
        FOREIGN KEY (creator_id) REFERENCES users(id) ON DELETE SET NULL;
END $$;

-- programs.game_id
DO $$
DECLARE
    cname text;
BEGIN
    SELECT conname INTO cname
      FROM pg_constraint
     WHERE conrelid = 'programs'::regclass
       AND conkey = ARRAY[(
           SELECT attnum FROM pg_attribute
            WHERE attrelid = 'programs'::regclass AND attname = 'game_id'
       )];
    IF cname IS NOT NULL THEN
        EXECUTE format('ALTER TABLE programs DROP CONSTRAINT %I', cname);
    END IF;
    ALTER TABLE programs
        ADD CONSTRAINT programs_game_id_fkey
        FOREIGN KEY (game_id) REFERENCES games(id) ON DELETE RESTRICT;
END $$;
