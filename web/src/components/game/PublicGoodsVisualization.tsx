import { useState } from 'react';

// Public Goods Visualization
export function PublicGoodsVisualization() {
  const [contribA, setContribA] = useState(10);
  const [contribB, setContribB] = useState(10);
  const ENDOWMENT = 20;
  const MULTIPLIER = 1.5;

  const pool = (contribA + contribB) * MULTIPLIER;
  const share = pool / 2;
  const payoffA = (ENDOWMENT - contribA) + share;
  const payoffB = (ENDOWMENT - contribB) + share;

  const adjustContrib = (player: 'A' | 'B', delta: number) => {
    if (player === 'A') {
      setContribA((prev) => Math.max(0, Math.min(ENDOWMENT, prev + delta)));
    } else {
      setContribB((prev) => Math.max(0, Math.min(ENDOWMENT, prev + delta)));
    }
  };

  const isFreeRiderA = contribA === 0 && contribB > 0;
  const isFreeRiderB = contribB === 0 && contribA > 0;
  const isFullCoop = contribA === ENDOWMENT && contribB === ENDOWMENT;
  const isNash = contribA === 0 && contribB === 0;

  return (
    <div className="flex flex-col justify-center space-y-4">
      {/* Contributions */}
      <div className="space-y-3">
        <div className="space-y-1">
          <div className="flex items-center justify-between text-xs">
            <span className="font-medium text-blue-400">Игрок A</span>
            <span className="text-gray-400">
              Вклад: <span className="font-bold text-blue-400">{contribA}</span> / {ENDOWMENT}
            </span>
          </div>
          <div className="flex items-center gap-2">
            <button onClick={() => adjustContrib('A', -2)} className="w-7 h-7 rounded bg-blue-900/50 text-blue-400 text-sm font-bold hover:bg-blue-900/80 transition-colors">-</button>
            <div className="flex-1 h-3 bg-gray-700 rounded-full overflow-hidden">
              <div className="h-full bg-blue-500 rounded-full transition-[width] duration-200" style={{ width: `${(contribA / ENDOWMENT) * 100}%` }} />
            </div>
            <button onClick={() => adjustContrib('A', 2)} className="w-7 h-7 rounded bg-blue-900/50 text-blue-400 text-sm font-bold hover:bg-blue-900/80 transition-colors">+</button>
          </div>
        </div>
        <div className="space-y-1">
          <div className="flex items-center justify-between text-xs">
            <span className="font-medium text-orange-400">Игрок B</span>
            <span className="text-gray-400">
              Вклад: <span className="font-bold text-orange-400">{contribB}</span> / {ENDOWMENT}
            </span>
          </div>
          <div className="flex items-center gap-2">
            <button onClick={() => adjustContrib('B', -2)} className="w-7 h-7 rounded bg-orange-900/50 text-orange-400 text-sm font-bold hover:bg-orange-900/80 transition-colors">-</button>
            <div className="flex-1 h-3 bg-gray-700 rounded-full overflow-hidden">
              <div className="h-full bg-orange-500 rounded-full transition-[width] duration-200" style={{ width: `${(contribB / ENDOWMENT) * 100}%` }} />
            </div>
            <button onClick={() => adjustContrib('B', 2)} className="w-7 h-7 rounded bg-orange-900/50 text-orange-400 text-sm font-bold hover:bg-orange-900/80 transition-colors">+</button>
          </div>
        </div>
      </div>

      {/* Pool calculation */}
      <div className="bg-gray-800 rounded-xl p-3 border border-gray-700">
        <div className="flex items-center justify-center gap-3 text-sm mb-2">
          <span className="text-gray-400">Пул:</span>
          <span className="text-blue-400 font-bold">{contribA}</span>
          <span className="text-gray-500">+</span>
          <span className="text-orange-400 font-bold">{contribB}</span>
          <span className="text-gray-500">=</span>
          <span className="text-gray-300 font-bold">{contribA + contribB}</span>
          <span className="text-green-400 font-bold">x{MULTIPLIER}</span>
          <span className="text-gray-500">=</span>
          <span className="text-green-400 font-bold text-lg">{pool.toFixed(0)}</span>
        </div>
        <div className="w-full h-2 bg-gray-700 rounded-full overflow-hidden">
          <div
            className="h-full bg-gradient-to-r from-blue-500 via-green-500 to-orange-500 rounded-full transition-[width] duration-200"
            style={{ width: `${(pool / (ENDOWMENT * 2 * MULTIPLIER)) * 100}%` }}
          />
        </div>
      </div>

      {/* Payoffs */}
      <div className="grid grid-cols-2 gap-3">
        <div className={`rounded-xl p-3 border text-center ${
          isFreeRiderA ? 'bg-red-900/20 border-red-700/50' : 'bg-gray-800 border-gray-700'
        }`}>
          <div className="text-xs text-gray-400 mb-1">Выигрыш A</div>
          <div className="text-2xl font-bold text-blue-400">{payoffA.toFixed(1)}</div>
          <div className="text-xs text-gray-500 mt-1">
            {ENDOWMENT - contribA} + {share.toFixed(1)}
          </div>
        </div>
        <div className={`rounded-xl p-3 border text-center ${
          isFreeRiderB ? 'bg-red-900/20 border-red-700/50' : 'bg-gray-800 border-gray-700'
        }`}>
          <div className="text-xs text-gray-400 mb-1">Выигрыш B</div>
          <div className="text-2xl font-bold text-orange-400">{payoffB.toFixed(1)}</div>
          <div className="text-xs text-gray-500 mt-1">
            {ENDOWMENT - contribB} + {share.toFixed(1)}
          </div>
        </div>
      </div>

      {/* Status */}
      <div className="flex justify-center gap-2 flex-wrap">
        {isNash && (
          <span className="px-2 py-1 bg-cyan-900/40 text-cyan-400 text-xs rounded-full border border-cyan-700/50">
            Равновесие Нэша: оба по 20
          </span>
        )}
        {isFullCoop && (
          <span className="px-2 py-1 bg-green-900/40 text-green-400 text-xs rounded-full border border-green-700/50">
            Полная кооперация: оба по 30!
          </span>
        )}
        {(isFreeRiderA || isFreeRiderB) && (
          <span className="px-2 py-1 bg-red-900/40 text-red-400 text-xs rounded-full border border-red-700/50">
            Безбилетник {isFreeRiderA ? 'A' : 'B'} выигрывает больше!
          </span>
        )}
        {!isNash && !isFullCoop && !isFreeRiderA && !isFreeRiderB && (
          <span className="text-xs text-gray-500">
            Нэш: (0, 0) = по 20 | Кооперация: (20, 20) = по 30
          </span>
        )}
      </div>
    </div>
  );
}
