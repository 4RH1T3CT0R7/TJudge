-- инвайт-коды всех команд отдавались публично в GET /tournaments/{id}/teams,
-- поэтому коды команд, куда ещё можно вступить (турнир pending), меняются.
-- gen_random_uuid берёт криптостойкий random, 8 hex-символов не пересекаются
-- со старыми 6-символьными кодами
UPDATE teams t
SET code = upper(substr(replace(gen_random_uuid()::text, '-', ''), 1, 8))
FROM tournaments tr
WHERE tr.id = t.tournament_id AND tr.status = 'pending';
