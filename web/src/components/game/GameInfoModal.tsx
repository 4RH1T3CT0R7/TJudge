import { gameDetails } from '../../utils/gameDatabase';
import { Modal } from '../ui/Modal';

// Modal component
export function GameInfoModal({
  isOpen,
  onClose,
  gameId,
  gameName,
  gameIcon
}: {
  isOpen: boolean;
  onClose: () => void;
  gameId: string;
  gameName: string;
  gameIcon: string;
}) {
  const details = gameDetails[gameId];
  if (!details) return null;

  return (
    <Modal
      open={isOpen}
      onClose={onClose}
      maxWidth="max-w-2xl"
      title={
        <span className="flex items-center gap-3">
          <span className="text-3xl" aria-hidden="true">{gameIcon}</span>
          {gameName}
        </span>
      }
    >
      {/* tabIndex: блок прокрутки входит в цикл Tab модалки и листается стрелками */}
      <div
        role="region"
        tabIndex={0}
        aria-label="Описание игры"
        className="overflow-y-auto max-h-[calc(85vh-80px)] space-y-6"
      >
        <div>
          <h3 className="text-sm font-bold text-primary-400 uppercase tracking-wide mb-3">
            История
          </h3>
          <div className="text-gray-300 text-sm leading-relaxed whitespace-pre-line">
            {details.history}
          </div>
        </div>

        <div>
          <h3 className="text-sm font-bold text-amber-400 uppercase tracking-wide mb-3">
            Интересные факты
          </h3>
          <ul className="space-y-2">
            {details.facts.map((fact, i) => (
              <li key={i} className="flex gap-2 text-sm text-gray-300">
                <span className="text-amber-500 mt-1">•</span>
                <span>{fact}</span>
              </li>
            ))}
          </ul>
        </div>

        <div>
          <h3 className="text-sm font-bold text-blue-400 uppercase tracking-wide mb-3">
            Где применяется
          </h3>
          <div className="flex flex-wrap gap-2">
            {details.applications.map((app, i) => (
              <span key={i} className="px-3 py-1 bg-blue-900/30 text-blue-300 text-xs rounded-full">
                {app}
              </span>
            ))}
          </div>
        </div>
      </div>
    </Modal>
  );
}
