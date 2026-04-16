-- Откат: возвращаем FK-constraints без явной ON DELETE политики
-- (в PostgreSQL это эквивалентно NO ACTION).

ALTER TABLE matches DROP CONSTRAINT IF EXISTS matches_program1_id_fkey;
ALTER TABLE matches
    ADD CONSTRAINT matches_program1_id_fkey
    FOREIGN KEY (program1_id) REFERENCES programs(id);

ALTER TABLE matches DROP CONSTRAINT IF EXISTS matches_program2_id_fkey;
ALTER TABLE matches
    ADD CONSTRAINT matches_program2_id_fkey
    FOREIGN KEY (program2_id) REFERENCES programs(id);

ALTER TABLE teams DROP CONSTRAINT IF EXISTS teams_leader_id_fkey;
ALTER TABLE teams
    ADD CONSTRAINT teams_leader_id_fkey
    FOREIGN KEY (leader_id) REFERENCES users(id);

ALTER TABLE tournaments DROP CONSTRAINT IF EXISTS tournaments_creator_id_fkey;
ALTER TABLE tournaments
    ADD CONSTRAINT tournaments_creator_id_fkey
    FOREIGN KEY (creator_id) REFERENCES users(id);

ALTER TABLE programs DROP CONSTRAINT IF EXISTS programs_game_id_fkey;
ALTER TABLE programs
    ADD CONSTRAINT programs_game_id_fkey
    FOREIGN KEY (game_id) REFERENCES games(id);
