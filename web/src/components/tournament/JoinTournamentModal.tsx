import { Field } from '../ui/Field';
import { Modal } from '../ui/Modal';

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
}

// Модалка «Участие в турнире»: создать команду или вступить по коду приглашения.
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

        <div className="relative">
          <div className="absolute inset-0 flex items-center">
            <div className="w-full border-t border-gray-700" />
          </div>
          <div className="relative flex justify-center text-sm">
            <span className="px-4 bg-gray-900 text-gray-400">или</span>
          </div>
        </div>

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
