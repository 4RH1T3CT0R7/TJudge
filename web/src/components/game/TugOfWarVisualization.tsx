import { useState } from 'react';

// Tug of War Visualization
export function TugOfWarVisualization() {
  const [rounds, setRounds] = useState([35, 35, 30]);
  const [opponentRounds] = useState([40, 30, 30]);
  const [showResults, setShowResults] = useState(false);
  const [currentRound, setCurrentRound] = useState(0);

  const totalForce = 100;
  const usedForce = rounds.reduce((a, b) => a + b, 0);
  const remaining = totalForce - usedForce;

  const adjustRound = (index: number, delta: number) => {
    const newRounds = [...rounds];
    const newValue = newRounds[index] + delta;
    if (newValue >= 0 && newValue <= 100 && usedForce + delta <= totalForce) {
      newRounds[index] = newValue;
      setRounds(newRounds);
      setShowResults(false);
    }
  };

  const getResults = () => {
    let playerWins = 0;
    let opponentWins = 0;
    rounds.forEach((force, i) => {
      if (force > opponentRounds[i]) playerWins++;
      else if (force < opponentRounds[i]) opponentWins++;
    });
    return { playerWins, opponentWins, winner: playerWins > opponentWins ? 'A' : opponentWins > playerWins ? 'B' : 'draw' };
  };

  const results = getResults();

  const getRopePosition = () => {
    if (!showResults) return 50;
    const force = rounds[currentRound];
    const oppForce = opponentRounds[currentRound];
    const diff = force - oppForce;
    return Math.max(20, Math.min(80, 50 - diff * 0.5));
  };

  const ropePosition = getRopePosition();

  return (
    <div className="flex flex-col justify-center space-y-3">
      {/* Rope visualization */}
      <div className="relative h-16 mx-2">
        <div className="absolute inset-0 flex">
          <div className={`flex-1 rounded-l-xl transition-colors ${ropePosition < 45 ? 'bg-blue-900/30' : 'bg-gray-800'}`} />
          <div className={`flex-1 rounded-r-xl transition-colors ${ropePosition > 55 ? 'bg-red-900/30' : 'bg-gray-800'}`} />
        </div>

        <div className="absolute left-1/2 top-0 bottom-0 w-0.5 bg-gray-600 -translate-x-1/2" />

        <svg className="absolute inset-0 w-full h-full" viewBox="0 0 300 60" preserveAspectRatio="none">
          <path
            d={`M 10,30 Q 75,${25 + Math.sin(Date.now() / 500) * 3} 150,30 Q 225,${35 + Math.sin(Date.now() / 500) * 3} 290,30`}
            fill="none" stroke="#b45309" strokeWidth="6" strokeLinecap="round"
          />
          <path
            d={`M 10,30 Q 75,${25 + Math.sin(Date.now() / 500) * 3} 150,30 Q 225,${35 + Math.sin(Date.now() / 500) * 3} 290,30`}
            fill="none" stroke="#d97706" strokeWidth="4" strokeLinecap="round"
          />
          <circle cx={ropePosition * 3} cy="30" r="10" fill="#dc2626" className="transition-all duration-500" />
          <circle cx={ropePosition * 3} cy="30" r="6" fill="#fca5a5" />
        </svg>

        <div className="absolute left-2 top-1/2 -translate-y-1/2 w-10 h-10 rounded-full bg-blue-500 flex items-center justify-center text-white font-bold text-sm shadow-lg">
          A
        </div>
        <div className="absolute right-2 top-1/2 -translate-y-1/2 w-10 h-10 rounded-full bg-red-500 flex items-center justify-center text-white font-bold text-sm shadow-lg">
          B
        </div>
      </div>

      {showResults && (
        <div className="flex justify-center gap-2">
          {rounds.map((_, i) => (
            <button
              key={i}
              onClick={() => setCurrentRound(i)}
              className={`px-3 py-1 rounded-lg text-xs font-medium transition-colors ${
                currentRound === i
                  ? 'bg-primary-500 text-white'
                  : 'bg-gray-700 text-gray-300'
              }`}
            >
              Раунд {i + 1}
            </button>
          ))}
        </div>
      )}

      <div className="space-y-2">
        {rounds.map((force, index) => (
          <div key={index} className="flex items-center gap-2 text-xs">
            <span className="w-14 text-gray-400">Раунд {index + 1}</span>
            <button onClick={() => adjustRound(index, -5)} disabled={force <= 0 || showResults} className="w-6 h-6 rounded bg-blue-900/50 text-blue-400 disabled:opacity-30">−</button>
            <div className="w-8 text-center font-bold text-blue-400">{force}</div>
            <button onClick={() => adjustRound(index, 5)} disabled={remaining <= 0 || showResults} className="w-6 h-6 rounded bg-blue-900/50 text-blue-400 disabled:opacity-30">+</button>
            <div className="flex-1 h-2 bg-gray-700 rounded-full overflow-hidden">
              <div className="h-full bg-blue-500 transition-[width]" style={{ width: `${force}%` }} />
            </div>
            {showResults && (
              <span className={`w-8 text-center font-bold ${
                force > opponentRounds[index] ? 'text-green-400' : force < opponentRounds[index] ? 'text-red-400' : 'text-gray-500'
              }`}>
                {force > opponentRounds[index] ? '✓' : force < opponentRounds[index] ? '✗' : '–'}
              </span>
            )}
          </div>
        ))}
      </div>

      {!showResults && remaining > 0 && (
        <div className="text-center text-xs text-gray-400">
          Осталось распределить: <span className="font-bold text-blue-400">{remaining}</span>
        </div>
      )}

      <div className="text-center">
        {!showResults ? (
          <button
            onClick={() => { setShowResults(true); setCurrentRound(0); }}
            disabled={remaining > 0}
            className="px-5 py-2 rounded-xl bg-gradient-to-r from-amber-500 to-orange-500 text-white text-sm font-bold shadow-lg hover:scale-105 transition-[transform,opacity] disabled:opacity-50 disabled:hover:scale-100"
          >
            {remaining > 0 ? `Ещё ${remaining}` : 'Тянуть!'}
          </button>
        ) : (
          <div className="space-y-1">
            <div className={`text-sm font-bold ${
              results.winner === 'A' ? 'text-blue-400' : results.winner === 'B' ? 'text-red-400' : 'text-gray-400'
            }`}>
              {results.winner === 'A' ? 'Победа!' : results.winner === 'B' ? 'Поражение' : 'Ничья'}
              <span className="text-gray-500 font-normal ml-2">({results.playerWins}:{results.opponentWins})</span>
            </div>
            <button onClick={() => { setShowResults(false); setRounds([35, 35, 30]); }} className="text-xs text-gray-500 hover:text-gray-300 underline">
              Заново
            </button>
          </div>
        )}
      </div>
    </div>
  );
}
