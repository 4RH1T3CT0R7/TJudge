import { useState } from 'react';

// Dollar Auction Visualization
const AUCTION_PRIZE = 100;
const AUCTION_SCENARIO = [
  { player: 'A', bid: 10 },
  { player: 'B', bid: 15 },
  { player: 'A', bid: 25 },
  { player: 'B', bid: 40 },
  { player: 'A', bid: 55 },
  { player: 'B', bid: 70 },
  { player: 'A', bid: 85 },
  { player: 'B', bid: 100 },
  { player: 'A', bid: 115 },
  { player: 'B', bid: 130 },
] as const;

export function DollarAuctionVisualization() {
  const [step, setStep] = useState(0);

  const currentBids = AUCTION_SCENARIO.slice(0, step);
  const lastBidA = [...currentBids].reverse().find((b) => b.player === 'A');
  const lastBidB = [...currentBids].reverse().find((b) => b.player === 'B');
  const bidA = lastBidA?.bid ?? 0;
  const bidB = lastBidB?.bid ?? 0;

  const winner = bidA >= bidB ? 'A' : 'B';
  const profitA = winner === 'A' ? AUCTION_PRIZE - bidA : -bidA;
  const profitB = winner === 'B' ? AUCTION_PRIZE - bidB : -bidB;

  const nextStep = () => setStep((prev) => Math.min(prev + 1, AUCTION_SCENARIO.length));
  const reset = () => setStep(0);

  return (
    <div className="flex flex-col justify-center space-y-3">
      {/* Prize */}
      <div className="text-center">
        <span className="text-xs text-gray-400">Приз: </span>
        <span className="text-lg font-bold text-yellow-400">{AUCTION_PRIZE} очков</span>
      </div>

      {/* Bid history */}
      <div className="bg-gray-800 rounded-xl p-3 border border-gray-700 max-h-40 overflow-y-auto">
        {step === 0 ? (
          <div className="text-center text-xs text-gray-500 py-2">
            Нажмите «Следующая ставка» чтобы начать
          </div>
        ) : (
          <div className="space-y-1">
            {currentBids.map((bid, i) => {
              const isLatest = i === currentBids.length - 1;
              const isA = bid.player === 'A';
              return (
                <div
                  key={i}
                  className={`flex items-center gap-2 text-xs px-2 py-1 rounded transition-colors ${
                    isLatest ? (isA ? 'bg-blue-900/30' : 'bg-red-900/30') : ''
                  }`}
                >
                  <span className={`w-5 h-5 rounded-full flex items-center justify-center text-white text-xs font-bold ${
                    isA ? 'bg-blue-500' : 'bg-red-500'
                  }`}>
                    {bid.player}
                  </span>
                  <span className="text-gray-400">ставит</span>
                  <span className={`font-bold ${isA ? 'text-blue-400' : 'text-red-400'}`}>{bid.bid}</span>
                  {bid.bid > AUCTION_PRIZE && (
                    <span className="text-red-400 ml-1">(&gt; приза!)</span>
                  )}
                </div>
              );
            })}
          </div>
        )}
      </div>

      {/* Current standings */}
      {step > 0 && (
        <div className="grid grid-cols-2 gap-3">
          <div className={`rounded-xl p-2 border text-center ${
            profitA >= 0 ? 'bg-green-900/20 border-green-700/50' : 'bg-red-900/20 border-red-700/50'
          }`}>
            <div className="text-xs text-gray-400">Игрок A</div>
            <div className="text-xs text-gray-500">ставка: {bidA}</div>
            <div className={`text-lg font-bold ${profitA >= 0 ? 'text-green-400' : 'text-red-400'}`}>
              {profitA > 0 ? '+' : ''}{profitA}
            </div>
          </div>
          <div className={`rounded-xl p-2 border text-center ${
            profitB >= 0 ? 'bg-green-900/20 border-green-700/50' : 'bg-red-900/20 border-red-700/50'
          }`}>
            <div className="text-xs text-gray-400">Игрок B</div>
            <div className="text-xs text-gray-500">ставка: {bidB}</div>
            <div className={`text-lg font-bold ${profitB >= 0 ? 'text-green-400' : 'text-red-400'}`}>
              {profitB > 0 ? '+' : ''}{profitB}
            </div>
          </div>
        </div>
      )}

      {/* Escalation warning */}
      {step > 0 && bidA + bidB > AUCTION_PRIZE && (
        <div className="text-center text-xs text-red-400 bg-red-900/20 rounded-lg py-1 border border-red-700/50">
          Суммарные ставки ({bidA + bidB}) превысили приз ({AUCTION_PRIZE})!
        </div>
      )}

      {/* Controls */}
      <div className="flex justify-center gap-3">
        {step < AUCTION_SCENARIO.length ? (
          <button
            onClick={nextStep}
            className="px-5 py-2 rounded-xl bg-gradient-to-r from-yellow-500 to-amber-500 text-white text-sm font-bold shadow-lg hover:scale-105 transition-transform"
          >
            Следующая ставка
          </button>
        ) : (
          <div className="text-center space-y-1">
            <div className="text-xs text-red-400 font-bold">
              Оба в убытке! A: {profitA}, B: {profitB}
            </div>
          </div>
        )}
        {step > 0 && (
          <button
            onClick={reset}
            className="px-4 py-2 rounded-xl bg-gray-700 text-gray-300 text-sm hover:bg-gray-600 transition-colors"
          >
            Сначала
          </button>
        )}
      </div>
    </div>
  );
}
