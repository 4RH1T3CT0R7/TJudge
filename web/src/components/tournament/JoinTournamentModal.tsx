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
        <div>
          <h3 className="font-semibold text-gray-100 mb-3">Создать новую команду</h3>
          <div className="flex gap-2">
            <input
              type="text"
              name="teamName"
              autoComplete="off"
              value={teamName}
              onChange={(e) => setTeamName(e.target.value)}
              placeholder="Название команды"
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
        </div>

        <div className="relative">
          <div className="absolute inset-0 flex items-center">
            <div className="w-full border-t border-gray-700" />
          </div>
          <div className="relative flex justify-center text-sm">
            <span className="px-4 bg-gray-900 text-gray-400">или</span>
          </div>
        </div>

        <div>
          <h3 className="font-semibold text-gray-100 mb-3">Присоединиться к существующей</h3>
          <div className="flex gap-2">
            <input
              type="text"
              name="joinCode"
              autoComplete="off"
              value={joinCode}
              onChange={(e) => { setJoinCode(e.target.value); setJoinError(''); }}
              placeholder="Код приглашения"
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
          {joinError && <p className="text-red-400 text-sm mt-1">{joinError}</p>}
        </div>
      </div>
    </Modal>
  );
}
