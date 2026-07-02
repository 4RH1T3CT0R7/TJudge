import { gameDetails } from '../../utils/gameDatabase';

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
  if (!isOpen) return null;

  const details = gameDetails[gameId];
  if (!details) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4" onClick={onClose}>
      <div className="absolute inset-0 bg-black/60 backdrop-blur-sm" />
      <div
        className="relative bg-gray-900 rounded-2xl shadow-2xl max-w-2xl w-full max-h-[85vh] overflow-hidden animate-scale-in border border-gray-800"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="sticky top-0 bg-gray-900 border-b border-gray-800 px-6 py-4 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <span className="text-3xl">{gameIcon}</span>
            <h2 className="text-xl font-bold text-gray-100">{gameName}</h2>
          </div>
          <button
            onClick={onClose}
            aria-label="Закрыть"
            className="w-8 h-8 rounded-full bg-gray-800 flex items-center justify-center hover:bg-gray-700 transition-colors"
          >
            <svg aria-hidden="true" className="w-5 h-5 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        {/* Content */}
        <div className="px-6 py-5 overflow-y-auto max-h-[calc(85vh-80px)] space-y-6">
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
      </div>
    </div>
  );
}
