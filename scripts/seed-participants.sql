-- Seed tournament participants
-- Generated from CSV data
-- Usage: replace TOURNAMENT_ID and PASSWORD_HASH, then run:
--   cat scripts/seed-participants.sql | sudo docker compose -f docker-compose.selfhosted.yml exec -T postgres psql -U tjudge -d tjudge
--
-- To generate password hash on server:
--   sudo docker compose -f docker-compose.selfhosted.yml exec api python3 -c "import bcrypt; print(bcrypt.hashpw(b'GameTheory2026', bcrypt.gensalt(10)).decode())"

BEGIN;

-- !!! REPLACE THESE !!!
\set tournament_id '00000000-0000-0000-0000-000000000000'
\set pwd_hash '$2a$10$REPLACE_WITH_REAL_BCRYPT_HASH'

-- ==========================================
-- Insert 89 users
-- ==========================================
INSERT INTO users (username, email, password_hash, role) VALUES
  ('teraqqq', 'SRBabin@yandex.ru', :'pwd_hash', 'user'),
  ('jurijo', 'y.feddorov@gmail.com', :'pwd_hash', 'user'),
  ('Kirill_Kudryashov', 'kskudryashov_1@edu.hse.ru', :'pwd_hash', 'user'),
  ('mottitov', 'matvey.titov.2017@mail.ru', :'pwd_hash', 'user'),
  ('kotik594', 'maxss.kzn@gmail.com', :'pwd_hash', 'user'),
  ('VovaBobatop', 'v-gashev@list.ru', :'pwd_hash', 'user'),
  ('Standuuser', 'maksimakimovd33@gmail.com', :'pwd_hash', 'user'),
  ('bgejetjet', 'savkarush@gmail.com', :'pwd_hash', 'user'),
  ('deshevie_aviabileti', 'uprtkotleta17@gmail.com', :'pwd_hash', 'user'),
  ('Alexdat2000', 'alexdat@list.ru', :'pwd_hash', 'user'),
  ('mr_kolya228228227', 'mr.nikola2004@bk.ru', :'pwd_hash', 'user'),
  ('mbolgov', 'misha20045000@gmail.com', :'pwd_hash', 'user'),
  ('valerikkkkk', 'rod_valer@mail.ru', :'pwd_hash', 'user'),
  ('robivirt', 'robivirt@mail.ru', :'pwd_hash', 'user'),
  ('Vladimir_N0vikov', 'vladimirnovikov2004@gmail.com', :'pwd_hash', 'user'),
  ('Timomeg', 'timomeg7@gmail.com', :'pwd_hash', 'user'),
  ('errare_humanum_est_7', 'pleschcoff.artym@yandex.ru', :'pwd_hash', 'user'),
  ('refresh_reality', 'ars89036698220potapov@yandex.ru', :'pwd_hash', 'user'),
  ('watlok', 'rewatlok@gmail.com', :'pwd_hash', 'user'),
  ('dreamenddischarger', 'linearspace45@gmail.com', :'pwd_hash', 'user'),
  ('yan_drozh', 'yan.drozd28@gmail.com', :'pwd_hash', 'user'),
  ('dibbruh', 'brutskiy.robog@mail.ru', :'pwd_hash', 'user'),
  ('kukaaanchik', 'a26849724@gmail.com', :'pwd_hash', 'user'),
  ('maks_matiupatenko', 'maksmtu@gmail.com', :'pwd_hash', 'user'),
  ('systy257', 'georgpv@mail.ru', :'pwd_hash', 'user'),
  ('GrifkaTop', 'Grifkatop@gmail.com', :'pwd_hash', 'user'),
  ('VitaliyK', 'VitalyKo2007@yandex.ru', :'pwd_hash', 'user'),
  ('mikhan_go', 'abulfat.misha@yandex.ru', :'pwd_hash', 'user'),
  ('Zakh_Vlad', '', :'pwd_hash', 'user'),
  ('Pojinatel', 'ruslan22334@mail.ru', :'pwd_hash', 'user'),
  ('ululu123', 'asgathusnullin80859@gmail.com', :'pwd_hash', 'user'),
  ('mihleonid', 'mihleonid@gmail.com', :'pwd_hash', 'user'),
  ('C4eboksar', 'denis.tugow2014@gmail.com', :'pwd_hash', 'user'),
  ('cvbnqq', 'vdmshbn@yandex.ru', :'pwd_hash', 'user'),
  ('gachimansemen', 's.anikin1309@gmail.com', :'pwd_hash', 'user'),
  ('catfloppahrt', 'dlavsego230606@mail.ru', :'pwd_hash', 'user'),
  ('misha_pro851', 'm1shutka.p@yandex.ru', :'pwd_hash', 'user'),
  ('Hrono5', 'veldyaskin2005@gmail.com', :'pwd_hash', 'user'),
  ('maaaruch', 'paveel.mixajlov@bk.ru', :'pwd_hash', 'user'),
  ('kamenyyy', 'ksmolin06@gmail.com', :'pwd_hash', 'user'),
  ('AfeelU', 'klokova_maria06@mail.ru', :'pwd_hash', 'user'),
  ('l_loret', 'gordeewandrey90@gmail.com', :'pwd_hash', 'user'),
  ('Tixdim', 'tixdim.on@gmail.com', :'pwd_hash', 'user'),
  ('kurygas', 'ikuryga@yandex.ru', :'pwd_hash', 'user'),
  ('randomrandoms', 'daniil.savinov.05@gmail.com', :'pwd_hash', 'user'),
  ('h_eldar', 'eldar9376445@gmail.com', :'pwd_hash', 'user'),
  ('NickMish', 'nickmishukov@gmail.com', :'pwd_hash', 'user'),
  ('DonSemyonio', 'semenchaykin2005@yandex.ru', :'pwd_hash', 'user'),
  ('maxim11111111', 'shepelev.maks.2005@mail.ru', :'pwd_hash', 'user'),
  ('Omsify', 'badzan00@mail.ru', :'pwd_hash', 'user'),
  ('timtim2379', 'ilyasovtimur2001@gmail.com', :'pwd_hash', 'user'),
  ('rmagg', 'ruslanmagaramov@inbox.ru', :'pwd_hash', 'user'),
  ('biba2231', 'giantufo0@gmail.com', :'pwd_hash', 'user'),
  ('negroid666', 'peter.eric2007@gmail.com', :'pwd_hash', 'user'),
  ('Dash2222', 'hpxhqls@mail.ru', :'pwd_hash', 'user'),
  ('buldakov_a', '20096ninja@gmail.com', :'pwd_hash', 'user'),
  ('extremepeacee', 'kirillgerasimov06@yandex.ry', :'pwd_hash', 'user'),
  ('morsxdd', 'derictor228@gmail.com', :'pwd_hash', 'user'),
  ('Demidyglas', 'roman091707@gmail.com', :'pwd_hash', 'user'),
  ('AndreyShalimov', 'fylhtq.2003@mail.ru', :'pwd_hash', 'user'),
  ('idm4x1', 'i.d.maximov@mail.ru', :'pwd_hash', 'user'),
  ('levshinartem', 'chimpoda@gmail.com', :'pwd_hash', 'user'),
  ('Cookie137', 'k.egor2007@gmail.com', :'pwd_hash', 'user'),
  ('BunnyQwQ', 'i9850072345@gmail.com', :'pwd_hash', 'user'),
  ('vnftkllsm', 'sfgserr@gmail.com', :'pwd_hash', 'user'),
  ('GerMaNfraer', 'flisd4282@gmail.com', :'pwd_hash', 'user'),
  ('mtvzv', 'matveyz07@mail.ru', :'pwd_hash', 'user'),
  ('bezpredel_nepredel', 's.tyannikov@yandex.ru', :'pwd_hash', 'user'),
  ('gusinyi_pashtet', 'krop2002@gmail.com', :'pwd_hash', 'user'),
  ('svenkapp', 'yuriy129@mail.ru', :'pwd_hash', 'user'),
  ('FrankKauperwooDd', 'nrakushin@internet.ru', :'pwd_hash', 'user'),
  ('sntkm', 'snetkovms@mail.ru', :'pwd_hash', 'user'),
  ('Frexto', 'f.maksim.2005@gmail.com', :'pwd_hash', 'user'),
  ('Vinogradov_dima', 'vinogradovdima123456789@gmail.com', :'pwd_hash', 'user'),
  ('io_mashtak', 'igorek.mashtak@gmail.com', :'pwd_hash', 'user'),
  ('marymigi', 'm2510450@edu.misis.ru', :'pwd_hash', 'user'),
  ('armenianorthodox', 'armina2407@mail.ru', :'pwd_hash', 'user'),
  ('boimelmaksim', 'ipazzxyz@gmail.com', :'pwd_hash', 'user'),
  ('secondhint', 'Gar89ipov@gmail.com', :'pwd_hash', 'user'),
  ('aalex_kuznecov', 'alexkuznetsov35@yandex.ru', :'pwd_hash', 'user'),
  ('Shutkarazavr', 'degtiarev32713@gmail.com', :'pwd_hash', 'user'),
  ('ArsenyKrasilnikov', 'arceniy.krasilnikov@yandex.ru', :'pwd_hash', 'user'),
  ('FIRE_present', 'misha.boris07@gmail.com', :'pwd_hash', 'user'),
  ('nnvxd0', 'daniil.zarechniy@gmail.com', :'pwd_hash', 'user'),
  ('antiplib', 'otvaginvlad@mail.ru', :'pwd_hash', 'user'),
  ('PotapovSerg', 'potserstan@mail.ru', :'pwd_hash', 'user'),
  ('Docknell', 'taraskirka07@gmail.com', :'pwd_hash', 'user'),
  ('Vadimka10', 'vadim.shilyaev8@gmail.com', :'pwd_hash', 'user'),
  ('soulj57rus', 'varnjuk79@gmail.com', :'pwd_hash', 'user')
ON CONFLICT (username) DO NOTHING;

-- Team 1: M.O.S.C.O.W.
INSERT INTO teams (tournament_id, name, code, leader_id)
  SELECT :'tournament_id'::uuid, 'M.O.S.C.O.W.', 'GERR53',
    (SELECT id FROM users WHERE username = 'Kirill_Kudryashov')
  WHERE NOT EXISTS (SELECT 1 FROM teams WHERE name = 'M.O.S.C.O.W.' AND tournament_id = :'tournament_id'::uuid);

INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'M.O.S.C.O.W.' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'teraqqq'
  ON CONFLICT DO NOTHING;
INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'M.O.S.C.O.W.' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'jurijo'
  ON CONFLICT DO NOTHING;
INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'M.O.S.C.O.W.' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'Kirill_Kudryashov'
  ON CONFLICT DO NOTHING;

-- Team 2: Аннигиляторы
INSERT INTO teams (tournament_id, name, code, leader_id)
  SELECT :'tournament_id'::uuid, 'Аннигиляторы', 'YJ52U7',
    (SELECT id FROM users WHERE username = 'mottitov')
  WHERE NOT EXISTS (SELECT 1 FROM teams WHERE name = 'Аннигиляторы' AND tournament_id = :'tournament_id'::uuid);

INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'Аннигиляторы' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'mottitov'
  ON CONFLICT DO NOTHING;
INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'Аннигиляторы' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'kotik594'
  ON CONFLICT DO NOTHING;
INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'Аннигиляторы' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'VovaBobatop'
  ON CONFLICT DO NOTHING;

-- Team 3: Луковый угар
INSERT INTO teams (tournament_id, name, code, leader_id)
  SELECT :'tournament_id'::uuid, 'Луковый угар', 'Z4ZMMX',
    (SELECT id FROM users WHERE username = 'deshevie_aviabileti')
  WHERE NOT EXISTS (SELECT 1 FROM teams WHERE name = 'Луковый угар' AND tournament_id = :'tournament_id'::uuid);

INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'Луковый угар' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'Standuuser'
  ON CONFLICT DO NOTHING;
INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'Луковый угар' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'bgejetjet'
  ON CONFLICT DO NOTHING;
INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'Луковый угар' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'deshevie_aviabileti'
  ON CONFLICT DO NOTHING;

-- Team 4: Capybara plushie
INSERT INTO teams (tournament_id, name, code, leader_id)
  SELECT :'tournament_id'::uuid, 'Capybara plushie', 'V1HXS6',
    (SELECT id FROM users WHERE username = 'Alexdat2000')
  WHERE NOT EXISTS (SELECT 1 FROM teams WHERE name = 'Capybara plushie' AND tournament_id = :'tournament_id'::uuid);

INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'Capybara plushie' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'Alexdat2000'
  ON CONFLICT DO NOTHING;
INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'Capybara plushie' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'mr_kolya228228227'
  ON CONFLICT DO NOTHING;
INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'Capybara plushie' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'mbolgov'
  ON CONFLICT DO NOTHING;

-- Team 5: brilliant blunder
INSERT INTO teams (tournament_id, name, code, leader_id)
  SELECT :'tournament_id'::uuid, 'brilliant blunder', 'R9I2ID',
    (SELECT id FROM users WHERE username = 'Vladimir_N0vikov')
  WHERE NOT EXISTS (SELECT 1 FROM teams WHERE name = 'brilliant blunder' AND tournament_id = :'tournament_id'::uuid);

INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'brilliant blunder' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'valerikkkkk'
  ON CONFLICT DO NOTHING;
INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'brilliant blunder' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'robivirt'
  ON CONFLICT DO NOTHING;
INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'brilliant blunder' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'Vladimir_N0vikov'
  ON CONFLICT DO NOTHING;

-- Team 6: NOIQONLYKVAS
INSERT INTO teams (tournament_id, name, code, leader_id)
  SELECT :'tournament_id'::uuid, 'NOIQONLYKVAS', 'NA32KL',
    (SELECT id FROM users WHERE username = 'refresh_reality')
  WHERE NOT EXISTS (SELECT 1 FROM teams WHERE name = 'NOIQONLYKVAS' AND tournament_id = :'tournament_id'::uuid);

INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'NOIQONLYKVAS' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'Timomeg'
  ON CONFLICT DO NOTHING;
INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'NOIQONLYKVAS' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'errare_humanum_est_7'
  ON CONFLICT DO NOTHING;
INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'NOIQONLYKVAS' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'refresh_reality'
  ON CONFLICT DO NOTHING;

-- Team 7: kosya fan club
INSERT INTO teams (tournament_id, name, code, leader_id)
  SELECT :'tournament_id'::uuid, 'kosya fan club', '32HLPU',
    (SELECT id FROM users WHERE username = 'dreamenddischarger')
  WHERE NOT EXISTS (SELECT 1 FROM teams WHERE name = 'kosya fan club' AND tournament_id = :'tournament_id'::uuid);

INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'kosya fan club' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'watlok'
  ON CONFLICT DO NOTHING;
INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'kosya fan club' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'dreamenddischarger'
  ON CONFLICT DO NOTHING;

-- Team 8: Дети дедлайна
INSERT INTO teams (tournament_id, name, code, leader_id)
  SELECT :'tournament_id'::uuid, 'Дети дедлайна', 'KBR7M9',
    (SELECT id FROM users WHERE username = 'kukaaanchik')
  WHERE NOT EXISTS (SELECT 1 FROM teams WHERE name = 'Дети дедлайна' AND tournament_id = :'tournament_id'::uuid);

INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'Дети дедлайна' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'yan_drozh'
  ON CONFLICT DO NOTHING;
INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'Дети дедлайна' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'dibbruh'
  ON CONFLICT DO NOTHING;
INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'Дети дедлайна' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'kukaaanchik'
  ON CONFLICT DO NOTHING;

-- Team 9: Грифаки
INSERT INTO teams (tournament_id, name, code, leader_id)
  SELECT :'tournament_id'::uuid, 'Грифаки', 'TYUTQB',
    (SELECT id FROM users WHERE username = 'GrifkaTop')
  WHERE NOT EXISTS (SELECT 1 FROM teams WHERE name = 'Грифаки' AND tournament_id = :'tournament_id'::uuid);

INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'Грифаки' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'maks_matiupatenko'
  ON CONFLICT DO NOTHING;
INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'Грифаки' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'systy257'
  ON CONFLICT DO NOTHING;
INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'Грифаки' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'GrifkaTop'
  ON CONFLICT DO NOTHING;

-- Team 10: ZXCorasik
INSERT INTO teams (tournament_id, name, code, leader_id)
  SELECT :'tournament_id'::uuid, 'ZXCorasik', 'B6EK9Q',
    (SELECT id FROM users WHERE username = 'VitaliyK')
  WHERE NOT EXISTS (SELECT 1 FROM teams WHERE name = 'ZXCorasik' AND tournament_id = :'tournament_id'::uuid);

INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'ZXCorasik' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'VitaliyK'
  ON CONFLICT DO NOTHING;
INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'ZXCorasik' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'mikhan_go'
  ON CONFLICT DO NOTHING;
INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'ZXCorasik' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'Zakh_Vlad'
  ON CONFLICT DO NOTHING;

-- Team 11: Basis
INSERT INTO teams (tournament_id, name, code, leader_id)
  SELECT :'tournament_id'::uuid, 'Basis', 'YF9YKS',
    (SELECT id FROM users WHERE username = 'mihleonid')
  WHERE NOT EXISTS (SELECT 1 FROM teams WHERE name = 'Basis' AND tournament_id = :'tournament_id'::uuid);

INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'Basis' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'Pojinatel'
  ON CONFLICT DO NOTHING;
INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'Basis' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'ululu123'
  ON CONFLICT DO NOTHING;
INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'Basis' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'mihleonid'
  ON CONFLICT DO NOTHING;

-- Team 12: KISAD_TEAM
INSERT INTO teams (tournament_id, name, code, leader_id)
  SELECT :'tournament_id'::uuid, 'KISAD_TEAM', '5O4N9Z',
    (SELECT id FROM users WHERE username = 'C4eboksar')
  WHERE NOT EXISTS (SELECT 1 FROM teams WHERE name = 'KISAD_TEAM' AND tournament_id = :'tournament_id'::uuid);

INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'KISAD_TEAM' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'C4eboksar'
  ON CONFLICT DO NOTHING;
INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'KISAD_TEAM' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'cvbnqq'
  ON CONFLICT DO NOTHING;
INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'KISAD_TEAM' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'gachimansemen'
  ON CONFLICT DO NOTHING;

-- Team 13: anything
INSERT INTO teams (tournament_id, name, code, leader_id)
  SELECT :'tournament_id'::uuid, 'anything', 'PPJUEZ',
    (SELECT id FROM users WHERE username = 'catfloppahrt')
  WHERE NOT EXISTS (SELECT 1 FROM teams WHERE name = 'anything' AND tournament_id = :'tournament_id'::uuid);

INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'anything' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'catfloppahrt'
  ON CONFLICT DO NOTHING;
INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'anything' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'misha_pro851'
  ON CONFLICT DO NOTHING;
INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'anything' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'Hrono5'
  ON CONFLICT DO NOTHING;

-- Team 14: 139 хромосом на троих
INSERT INTO teams (tournament_id, name, code, leader_id)
  SELECT :'tournament_id'::uuid, '139 хромосом на троих', '2657H2',
    (SELECT id FROM users WHERE username = 'kamenyyy')
  WHERE NOT EXISTS (SELECT 1 FROM teams WHERE name = '139 хромосом на троих' AND tournament_id = :'tournament_id'::uuid);

INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = '139 хромосом на троих' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'maaaruch'
  ON CONFLICT DO NOTHING;
INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = '139 хромосом на троих' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'kamenyyy'
  ON CONFLICT DO NOTHING;
INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = '139 хромосом на троих' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'AfeelU'
  ON CONFLICT DO NOTHING;

-- Team 15: Winless gang
INSERT INTO teams (tournament_id, name, code, leader_id)
  SELECT :'tournament_id'::uuid, 'Winless gang', 'MUE9B9',
    (SELECT id FROM users WHERE username = 'l_loret')
  WHERE NOT EXISTS (SELECT 1 FROM teams WHERE name = 'Winless gang' AND tournament_id = :'tournament_id'::uuid);

INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'Winless gang' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'l_loret'
  ON CONFLICT DO NOTHING;
INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'Winless gang' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'Tixdim'
  ON CONFLICT DO NOTHING;
INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'Winless gang' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'kurygas'
  ON CONFLICT DO NOTHING;

-- Team 16: Log'N'Roll
INSERT INTO teams (tournament_id, name, code, leader_id)
  SELECT :'tournament_id'::uuid, 'Log''N''Roll', 'GZ18CS',
    (SELECT id FROM users WHERE username = 'NickMish')
  WHERE NOT EXISTS (SELECT 1 FROM teams WHERE name = 'Log''N''Roll' AND tournament_id = :'tournament_id'::uuid);

INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'Log''N''Roll' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'randomrandoms'
  ON CONFLICT DO NOTHING;
INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'Log''N''Roll' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'h_eldar'
  ON CONFLICT DO NOTHING;
INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'Log''N''Roll' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'NickMish'
  ON CONFLICT DO NOTHING;

-- Team 17: Bauman_code_mems
INSERT INTO teams (tournament_id, name, code, leader_id)
  SELECT :'tournament_id'::uuid, 'Bauman_code_mems', 'VLWONO',
    (SELECT id FROM users WHERE username = 'Omsify')
  WHERE NOT EXISTS (SELECT 1 FROM teams WHERE name = 'Bauman_code_mems' AND tournament_id = :'tournament_id'::uuid);

INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'Bauman_code_mems' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'DonSemyonio'
  ON CONFLICT DO NOTHING;
INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'Bauman_code_mems' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'maxim11111111'
  ON CONFLICT DO NOTHING;
INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'Bauman_code_mems' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'Omsify'
  ON CONFLICT DO NOTHING;

-- Team 18: Chili Nedras
INSERT INTO teams (tournament_id, name, code, leader_id)
  SELECT :'tournament_id'::uuid, 'Chili Nedras', 'S9MDRX',
    (SELECT id FROM users WHERE username = 'timtim2379')
  WHERE NOT EXISTS (SELECT 1 FROM teams WHERE name = 'Chili Nedras' AND tournament_id = :'tournament_id'::uuid);

INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'Chili Nedras' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'timtim2379'
  ON CONFLICT DO NOTHING;
INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'Chili Nedras' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'rmagg'
  ON CONFLICT DO NOTHING;
INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'Chili Nedras' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'biba2231'
  ON CONFLICT DO NOTHING;

-- Team 19: WA Enjoyers
INSERT INTO teams (tournament_id, name, code, leader_id)
  SELECT :'tournament_id'::uuid, 'WA Enjoyers', 'E805TV',
    (SELECT id FROM users WHERE username = 'negroid666')
  WHERE NOT EXISTS (SELECT 1 FROM teams WHERE name = 'WA Enjoyers' AND tournament_id = :'tournament_id'::uuid);

INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'WA Enjoyers' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'negroid666'
  ON CONFLICT DO NOTHING;
INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'WA Enjoyers' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'Dash2222'
  ON CONFLICT DO NOTHING;
INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'WA Enjoyers' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'buldakov_a'
  ON CONFLICT DO NOTHING;

-- Team 20: Polevoy fans
INSERT INTO teams (tournament_id, name, code, leader_id)
  SELECT :'tournament_id'::uuid, 'Polevoy fans', '679ALH',
    (SELECT id FROM users WHERE username = 'morsxdd')
  WHERE NOT EXISTS (SELECT 1 FROM teams WHERE name = 'Polevoy fans' AND tournament_id = :'tournament_id'::uuid);

INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'Polevoy fans' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'extremepeacee'
  ON CONFLICT DO NOTHING;
INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'Polevoy fans' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'morsxdd'
  ON CONFLICT DO NOTHING;
INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'Polevoy fans' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'Demidyglas'
  ON CONFLICT DO NOTHING;

-- Team 21: [МИСИС] В потоке
INSERT INTO teams (tournament_id, name, code, leader_id)
  SELECT :'tournament_id'::uuid, '[МИСИС] В потоке', '70M78J',
    (SELECT id FROM users WHERE username = 'AndreyShalimov')
  WHERE NOT EXISTS (SELECT 1 FROM teams WHERE name = '[МИСИС] В потоке' AND tournament_id = :'tournament_id'::uuid);

INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = '[МИСИС] В потоке' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'AndreyShalimov'
  ON CONFLICT DO NOTHING;
INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = '[МИСИС] В потоке' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'idm4x1'
  ON CONFLICT DO NOTHING;
INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = '[МИСИС] В потоке' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'levshinartem'
  ON CONFLICT DO NOTHING;

-- Team 22: DEVilS
INSERT INTO teams (tournament_id, name, code, leader_id)
  SELECT :'tournament_id'::uuid, 'DEVilS', 'TJQ3Y9',
    (SELECT id FROM users WHERE username = 'Cookie137')
  WHERE NOT EXISTS (SELECT 1 FROM teams WHERE name = 'DEVilS' AND tournament_id = :'tournament_id'::uuid);

INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'DEVilS' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'Cookie137'
  ON CONFLICT DO NOTHING;
INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'DEVilS' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'BunnyQwQ'
  ON CONFLICT DO NOTHING;
INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'DEVilS' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'vnftkllsm'
  ON CONFLICT DO NOTHING;

-- Team 23: ПТУ имени Дейкстры
INSERT INTO teams (tournament_id, name, code, leader_id)
  SELECT :'tournament_id'::uuid, 'ПТУ имени Дейкстры', 'C6TOZZ',
    (SELECT id FROM users WHERE username = 'mtvzv')
  WHERE NOT EXISTS (SELECT 1 FROM teams WHERE name = 'ПТУ имени Дейкстры' AND tournament_id = :'tournament_id'::uuid);

INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'ПТУ имени Дейкстры' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'GerMaNfraer'
  ON CONFLICT DO NOTHING;
INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'ПТУ имени Дейкстры' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'mtvzv'
  ON CONFLICT DO NOTHING;
INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'ПТУ имени Дейкстры' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'bezpredel_nepredel'
  ON CONFLICT DO NOTHING;

-- Team 24: mmm
INSERT INTO teams (tournament_id, name, code, leader_id)
  SELECT :'tournament_id'::uuid, 'mmm', 'CCQCK3',
    (SELECT id FROM users WHERE username = 'FrankKauperwooDd')
  WHERE NOT EXISTS (SELECT 1 FROM teams WHERE name = 'mmm' AND tournament_id = :'tournament_id'::uuid);

INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'mmm' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'gusinyi_pashtet'
  ON CONFLICT DO NOTHING;
INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'mmm' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'svenkapp'
  ON CONFLICT DO NOTHING;
INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'mmm' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'FrankKauperwooDd'
  ON CONFLICT DO NOTHING;

-- Team 25: Karaoke shaitan
INSERT INTO teams (tournament_id, name, code, leader_id)
  SELECT :'tournament_id'::uuid, 'Karaoke shaitan', 'N20MQF',
    (SELECT id FROM users WHERE username = 'Frexto')
  WHERE NOT EXISTS (SELECT 1 FROM teams WHERE name = 'Karaoke shaitan' AND tournament_id = :'tournament_id'::uuid);

INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'Karaoke shaitan' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'sntkm'
  ON CONFLICT DO NOTHING;
INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'Karaoke shaitan' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'Frexto'
  ON CONFLICT DO NOTHING;
INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'Karaoke shaitan' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'Vinogradov_dima'
  ON CONFLICT DO NOTHING;

-- Team 26: MISIS-25-16
INSERT INTO teams (tournament_id, name, code, leader_id)
  SELECT :'tournament_id'::uuid, 'MISIS-25-16', 'HNDS1M',
    (SELECT id FROM users WHERE username = 'armenianorthodox')
  WHERE NOT EXISTS (SELECT 1 FROM teams WHERE name = 'MISIS-25-16' AND tournament_id = :'tournament_id'::uuid);

INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'MISIS-25-16' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'io_mashtak'
  ON CONFLICT DO NOTHING;
INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'MISIS-25-16' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'marymigi'
  ON CONFLICT DO NOTHING;
INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'MISIS-25-16' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'armenianorthodox'
  ON CONFLICT DO NOTHING;

-- Team 27: cododep260
INSERT INTO teams (tournament_id, name, code, leader_id)
  SELECT :'tournament_id'::uuid, 'cododep260', 'PAKHW6',
    (SELECT id FROM users WHERE username = 'secondhint')
  WHERE NOT EXISTS (SELECT 1 FROM teams WHERE name = 'cododep260' AND tournament_id = :'tournament_id'::uuid);

INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'cododep260' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'boimelmaksim'
  ON CONFLICT DO NOTHING;
INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'cododep260' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'secondhint'
  ON CONFLICT DO NOTHING;
INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'cododep260' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'aalex_kuznecov'
  ON CONFLICT DO NOTHING;

-- Team 28: Гавайская пицца
INSERT INTO teams (tournament_id, name, code, leader_id)
  SELECT :'tournament_id'::uuid, 'Гавайская пицца', 'U784W6',
    (SELECT id FROM users WHERE username = 'FIRE_present')
  WHERE NOT EXISTS (SELECT 1 FROM teams WHERE name = 'Гавайская пицца' AND tournament_id = :'tournament_id'::uuid);

INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'Гавайская пицца' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'Shutkarazavr'
  ON CONFLICT DO NOTHING;
INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'Гавайская пицца' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'ArsenyKrasilnikov'
  ON CONFLICT DO NOTHING;
INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'Гавайская пицца' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'FIRE_present'
  ON CONFLICT DO NOTHING;

-- Team 29: Berezka
INSERT INTO teams (tournament_id, name, code, leader_id)
  SELECT :'tournament_id'::uuid, 'Berezka', '39KTDD',
    (SELECT id FROM users WHERE username = 'PotapovSerg')
  WHERE NOT EXISTS (SELECT 1 FROM teams WHERE name = 'Berezka' AND tournament_id = :'tournament_id'::uuid);

INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'Berezka' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'nnvxd0'
  ON CONFLICT DO NOTHING;
INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'Berezka' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'antiplib'
  ON CONFLICT DO NOTHING;
INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'Berezka' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'PotapovSerg'
  ON CONFLICT DO NOTHING;

-- Team 30: Пептидилтрансферазный центр
INSERT INTO teams (tournament_id, name, code, leader_id)
  SELECT :'tournament_id'::uuid, 'Пептидилтрансферазный центр', 'NGX87O',
    (SELECT id FROM users WHERE username = 'Vadimka10')
  WHERE NOT EXISTS (SELECT 1 FROM teams WHERE name = 'Пептидилтрансферазный центр' AND tournament_id = :'tournament_id'::uuid);

INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'Пептидилтрансферазный центр' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'Docknell'
  ON CONFLICT DO NOTHING;
INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'Пептидилтрансферазный центр' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'Vadimka10'
  ON CONFLICT DO NOTHING;
INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'Пептидилтрансферазный центр' AND t.tournament_id = :'tournament_id'::uuid AND u.username = 'soulj57rus'
  ON CONFLICT DO NOTHING;

COMMIT;
-- Done: 30 teams, 89 users
