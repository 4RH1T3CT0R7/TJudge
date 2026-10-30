import { useState } from 'react';
import axios from 'axios';
import { useQueries, useQueryClient } from '@tanstack/react-query';
import api from '../../api/client';
import { queryKeys } from '../../api/queryKeys';
import { useTournamentTeams } from '../../hooks/queries';
import { confirmDialog } from '../../store/confirmStore';
import { useToastStore } from '../../store/toastStore';
import { Field } from '../ui/Field';
import { Spinner } from '../ui/Spinner';
import { statusLabels } from './types';
import type { Team, Tournament, User } from '../../types';

// Текст ошибки переноса: сервер отвечает по-английски
function moveErrorMessage(err: unknown): string {
  const raw = axios.isAxiosError(err) ? String(err.response?.data?.error ?? '') : '';
  // «cannot delete team of active or completed tournament» тоже содержит completed
  if (raw.includes('cannot delete team')) return 'Это последний участник команды идущего турнира: команда осталась бы пустой';
  if (raw.includes('completed')) return 'Турнир завершён: составы команд не меняются';
  return raw || 'Не удалось изменить состав';
}

// Составы команд турнира: админ переносит участника в другую команду или
// убирает из команды. Уход капитана передаёт капитанство другому участнику,
// опустевшая команда турнира в регистрации удаляется - как при выходе.
export function TeamsAdminTab({ tournaments }: { tournaments: Tournament[] }) {
  const [tournamentId, setTournamentId] = useState('');
  const [busy, setBusy] = useState(false);
  const queryClient = useQueryClient();
  const tournament = tournaments.find((t) => t.id === tournamentId);
  const teamsQuery = useTournamentTeams(tournamentId);
  const teams = teamsQuery.data ?? [];
  const teamQueries = useQueries({
    queries: teams.map((t) => ({ queryKey: queryKeys.team(t.id), queryFn: () => api.getTeam(t.id) })),
  });
  const locked = tournament?.status === 'completed';

  // touched - команды, чей состав поменялся
  const change = async (action: () => Promise<void>, done: string, touched: string[]) => {
    setBusy(true);
    try {
      await action();
      useToastStore.getState().addToast(done, 'success');
    } catch (err) {
      useToastStore.getState().addToast(moveErrorMessage(err), 'error');
    } finally {
      // сначала список: опустевшая команда удалена, её состав перезапрос дал бы 404
      await queryClient.invalidateQueries({ queryKey: queryKeys.tournamentTeams(tournamentId) });
      const left = new Set((queryClient.getQueryData<Team[]>(queryKeys.tournamentTeams(tournamentId)) ?? []).map((t) => t.id));
      for (const id of touched) {
        if (left.has(id)) void queryClient.invalidateQueries({ queryKey: queryKeys.team(id) });
        else queryClient.removeQueries({ queryKey: queryKeys.team(id) });
      }
      setBusy(false);
    }
  };

  // последний участник команды в регистрации уводит её с собой: это подтверждается
  const confirmEmptied = (team: Team, members: User[], message: string) =>
    members.length > 1 || tournament?.status !== 'pending'
      ? confirmDialog({ message })
      : confirmDialog({ message, details: [`команда «${team.name}» останется пустой и будет удалена`], danger: true });

  const move = async (from: Team, members: User[], user: User, toId: string) => {
    const to = teams.find((t) => t.id === toId);
    if (!to) return;
    if (!(await confirmEmptied(from, members, `Перенести ${user.username} из «${from.name}» в «${to.name}»?`))) return;
    await change(() => api.moveTeamMember(to.id, user.id), `${user.username} теперь в «${to.name}»`, [from.id, to.id]);
  };

  const remove = async (from: Team, members: User[], user: User) => {
    if (!(await confirmEmptied(from, members, `Убрать ${user.username} из «${from.name}»?`))) return;
    await change(() => api.removeMember(from.id, user.id, true), `${user.username} убран из «${from.name}»`, [from.id]);
  };

  return (
    <div>
      <h2 className="mb-4 text-lg font-semibold text-gray-100">Составы команд</h2>
      <Field label="Выберите турнир" className="mb-6">
        {(control) => (
          <select {...control} value={tournamentId} onChange={(e) => setTournamentId(e.target.value)} className="input max-w-md">
            <option value="">-- Выберите турнир --</option>
            {tournaments.map((t) => (
              <option key={t.id} value={t.id}>
                {t.name} ({statusLabels[t.status]})
              </option>
            ))}
          </select>
        )}
      </Field>

      {tournamentId && teamsQuery.isLoading && <Spinner className="text-sm text-gray-400">загрузка команд</Spinner>}
      {tournamentId && !teamsQuery.isLoading && teams.length === 0 && <p className="text-sm text-gray-400">В турнире нет команд</p>}
      {locked && <p className="mb-4 text-sm text-gray-400">Турнир завершён: составы только для просмотра.</p>}

      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        {teams.map((team, i) => {
          const members = teamQueries[i]?.data?.members ?? [];
          return (
            <section key={team.id} aria-label={`Команда ${team.name}`} className={`card ${team.is_disqualified ? 'border-red-600' : ''}`}>
              <h3 className="flex flex-wrap items-center gap-2 font-semibold text-gray-100">
                <span className="min-w-0 truncate">{team.name}</span>
                {team.is_disqualified && <span className="badge badge-red">дисквалифицирована</span>}
                <span className="text-sm font-normal text-gray-400">участников {team.member_count ?? members.length}</span>
              </h3>
              {teamQueries[i]?.isLoading ? (
                <Spinner className="mt-3 text-xs text-gray-500">загрузка</Spinner>
              ) : (
                <ul className="mt-3 space-y-2">
                  {members.map((m) => (
                    <li key={m.id} className="flex flex-wrap items-center gap-2 text-sm">
                      <span className="min-w-0 flex-1">
                        <span className="text-gray-100">{m.username}</span>
                        {m.id === team.leader_id && <span className="badge badge-blue ml-2">капитан</span>}
                        <span className="ml-2 text-xs text-gray-500">{m.email}</span>
                      </span>
                      {!locked && (
                        <>
                          {teams.length > 1 && (
                            <select
                              aria-label={`Перенести ${m.username} в другую команду`}
                              value=""
                              disabled={busy}
                              onChange={(e) => void move(team, members, m, e.target.value)}
                              className="input w-auto py-1 text-sm"
                            >
                              <option value="">Перенести в…</option>
                              {teams
                                .filter((t) => t.id !== team.id)
                                .map((t) => (
                                  <option key={t.id} value={t.id}>
                                    {t.name}
                                  </option>
                                ))}
                            </select>
                          )}
                          <button type="button" onClick={() => void remove(team, members, m)} disabled={busy} className="btn btn-sm btn-danger">
                            Убрать
                          </button>
                        </>
                      )}
                    </li>
                  ))}
                </ul>
              )}
            </section>
          );
        })}
      </div>
    </div>
  );
}
