import { Field } from '../ui/Field';
import { Modal } from '../ui/Modal';
import type { Team } from '../../types';

interface JoinTournamentModalProps {
  open: boolean;
  onClose: () => void;
  teamName: string;
  setTeamName: (name: string) => void;
  joinCode: string;
  setJoinCode: (code: string) => void;
  joinError: string;
  setJoinError: (e: string) => void;
  isJoining: boolean;
  onCreateTeam: () => void;
  onJoinTeam: () => void;
  /** Команды, в которые можно вступить из списка. */
  teams: Team[];
  maxTeamSize: number;
  onJoinTeamById: (teamId: string) => void;
}

function Or() {
  return (
    <div className="relative">
      <div className="absolute inset-0 flex items-center">
        <div className="w-full border-t border-gray-700" />
      </div>
      <div className="relative flex justify-center text-sm">
        <span className="px-4 bg-gray-900 text-gray-400">или</span>
      </div>
    </div>
  );
}

// Модалка «Участие в турнире»: создать команду, вступить в команду из списка
// или по коду приглашения.
export function JoinTournamentModal({
  open,
  onClose,
  teamName,
  setTeamName,
  joinCode,
  setJoinCode,
  joinError,
  setJoinError,
  isJoining,
  onCreateTeam,
  onJoinTeam,
  teams,
  maxTeamSize,
  onJoinTeamById,
}: JoinTournamentModalProps) {
  return (
    <Modal open={open} onClose={onClose} title="Участие в турнире">
      <div className="space-y-6">
        <Field label="Название новой команды">
          {(control) => (
            <div className="flex gap-2">
              <input
                {...control}
                type="text"
                name="teamName"
                autoComplete="off"
                value={teamName}
                onChange={(e) => setTeamName(e.target.value)}
                className="input flex-1"
              />
              <button
                onClick={onCreateTeam}
                disabled={isJoining || !teamName.trim()}
                className="btn btn-primary"
              >
                Создать
              </button>
            </div>
          )}
        </Field>

        {teams.length > 0 && (
          <>
            <Or />
            <section aria-labelledby="join-list-title">
              <h3 id="join-list-title" className="mb-2 text-sm font-medium text-gray-300">Вступить в команду</h3>
              <ul className="max-h-64 space-y-2 overflow-y-auto">
                {teams.map((t) => (
                  <li key={t.id} className="flex items-center gap-2">
                    <span className="min-w-0 flex-1 truncate text-gray-100">{t.name}</span>
                    <span className="shrink-0 text-xs tabular-nums text-gray-400">
                      <span className="sr-only">участников </span>
                      {t.member_count ?? 0}{maxTeamSize > 0 && `/${maxTeamSize}`}
                    </span>
                    <button
                      type="button"
                      onClick={() => onJoinTeamById(t.id)}
                      disabled={isJoining}
                      aria-label={`Вступить в команду ${t.name}`}
                      className="btn btn-sm btn-secondary shrink-0"
                    >
                      Вступить
                    </button>
                  </li>
                ))}
              </ul>
            </section>
          </>
        )}

        <Or />

        <Field label="Код приглашения в команду" error={joinError || undefined}>
          {(control) => (
            <div className="flex gap-2">
              <input
                {...control}
                type="text"
                name="joinCode"
                autoComplete="off"
                value={joinCode}
                onChange={(e) => { setJoinCode(e.target.value); setJoinError(''); }}
                className="input flex-1 font-mono"
              />
              <button
                onClick={onJoinTeam}
                disabled={isJoining || !joinCode.trim()}
                className="btn btn-secondary"
              >
                Вступить
              </button>
            </div>
          )}
        </Field>
      </div>
    </Modal>
  );
}
