import { useState } from 'react';

// Traveler's Dilemma Visualization
export function TravelersDilemmaVisualization() {
  const [claimA, setClaimA] = useState(80);
  const [claimB, setClaimB] = useState(80);
  const R = 2;

  const computePayoffs = () => {
    if (claimA === claimB) {
      return { payoffA: claimA, payoffB: claimB };
    }
    const minClaim = Math.min(claimA, claimB);
    if (claimA < claimB) {
      return { payoffA: minClaim + R, payoffB: minClaim - R };
    }
    return { payoffA: minClaim - R, payoffB: minClaim + R };
  };

  const { payoffA, payoffB } = computePayoffs();
  const isEqual = claimA === claimB;
  const isNash = claimA === 2 && claimB === 2;
  const isCooperative = claimA === 100 && claimB === 100;

  const adjustClaim = (player: 'A' | 'B', delta: number) => {
    if (player === 'A') {
      setClaimA((prev) => Math.max(2, Math.min(100, prev + delta)));
    } else {
      setClaimB((prev) => Math.max(2, Math.min(100, prev + delta)));
    }
  };

  return (
    <div className="flex flex-col justify-center space-y-4">
      {/* Claims */}
      <div className="space-y-3">
        <div className="space-y-1">
          <div className="flex items-center justify-between text-xs">
            <span className="font-medium text-blue-400">Игрок A</span>
            <span className="text-gray-400">Заявка: <span className="font-bold text-blue-400">{claimA}</span></span>
          </div>
          <div className="flex items-center gap-2">
            <button onClick={() => adjustClaim('A', -10)} className="w-7 h-7 rounded bg-blue-900/50 text-blue-400 text-sm font-bold hover:bg-blue-900/80 transition-colors">-</button>
            <div className="flex-1 h-3 bg-gray-700 rounded-full overflow-hidden relative">
              <div className="h-full bg-blue-500 rounded-full transition-[width] duration-200" style={{ width: `${((claimA - 2) / 98) * 100}%` }} />
            </div>
            <button onClick={() => adjustClaim('A', 10)} className="w-7 h-7 rounded bg-blue-900/50 text-blue-400 text-sm font-bold hover:bg-blue-900/80 transition-colors">+</button>
          </div>
        </div>
        <div className="space-y-1">
          <div className="flex items-center justify-between text-xs">
            <span className="font-medium text-purple-400">Игрок B</span>
            <span className="text-gray-400">Заявка: <span className="font-bold text-purple-400">{claimB}</span></span>
          </div>
          <div className="flex items-center gap-2">
            <button onClick={() => adjustClaim('B', -10)} className="w-7 h-7 rounded bg-purple-900/50 text-purple-400 text-sm font-bold hover:bg-purple-900/80 transition-colors">-</button>
            <div className="flex-1 h-3 bg-gray-700 rounded-full overflow-hidden relative">
              <div className="h-full bg-purple-500 rounded-full transition-[width] duration-200" style={{ width: `${((claimB - 2) / 98) * 100}%` }} />
            </div>
            <button onClick={() => adjustClaim('B', 10)} className="w-7 h-7 rounded bg-purple-900/50 text-purple-400 text-sm font-bold hover:bg-purple-900/80 transition-colors">+</button>
          </div>
        </div>
      </div>

      {/* Payoff display */}
      <div className={`rounded-xl p-3 border transition-colors ${
        isEqual ? 'bg-green-900/20 border-green-700/50' : 'bg-gray-800 border-gray-700'
      }`}>
        <div className="flex justify-around items-center">
          <div className="text-center">
            <div className="text-xs text-gray-400 mb-1">Выигрыш A</div>
            <div className={`text-2xl font-bold ${
              payoffA > payoffB ? 'text-green-400' : payoffA < payoffB ? 'text-red-400' : 'text-blue-400'
            }`}>{payoffA}</div>
          </div>
          <div className="text-gray-600 text-lg">vs</div>
          <div className="text-center">
            <div className="text-xs text-gray-400 mb-1">Выигрыш B</div>
            <div className={`text-2xl font-bold ${
              payoffB > payoffA ? 'text-green-400' : payoffB < payoffA ? 'text-red-400' : 'text-purple-400'
            }`}>{payoffB}</div>
          </div>
        </div>
        {isEqual && (
          <div className="text-center text-xs text-green-400 mt-2">Одинаковые заявки: оба получают {claimA}</div>
        )}
        {!isEqual && (
          <div className="text-center text-xs text-gray-400 mt-2">
            Скромный получает +{R}, жадный получает -{R} от минимума ({Math.min(claimA, claimB)})
          </div>
        )}
      </div>

      {/* Status badges */}
      <div className="flex justify-center gap-2 flex-wrap">
        {isNash && (
          <span className="px-2 py-1 bg-cyan-900/40 text-cyan-400 text-xs rounded-full border border-cyan-700/50">
            Равновесие Нэша (2, 2)
          </span>
        )}
        {isCooperative && (
          <span className="px-2 py-1 bg-green-900/40 text-green-400 text-xs rounded-full border border-green-700/50">
            Кооперативный оптимум (100, 100)
          </span>
        )}
        {!isNash && !isCooperative && (
          <span className="text-xs text-gray-500">
            Нэш: (2, 2) = по 2 | Кооперация: (100, 100) = по 100
          </span>
        )}
      </div>
    </div>
  );
}
