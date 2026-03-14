import { Link } from 'react-router-dom';
import { useState, useEffect, useRef, useCallback } from 'react';
import { SpaceInvader } from '../components/SpaceInvader';
import { TerminalTypewriter } from '../components/TerminalTypewriter';
import { TerminalQuest } from '../components/TerminalQuest';
import { PixelGrid } from '../components/PixelGrid';
import { StaggerList, StaggerItem } from '../components/motion/StaggerList';
import { useEscapeKey } from '../hooks/useEscapeKey';

const TrophyIcon = () => (
  <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-6 h-6">
    <path strokeLinecap="round" strokeLinejoin="round" d="M16.5 18.75h-9m9 0a3 3 0 0 1 3 3h-15a3 3 0 0 1 3-3m9 0v-3.375c0-.621-.503-1.125-1.125-1.125h-.871M7.5 18.75v-3.375c0-.621.504-1.125 1.125-1.125h.872m5.007 0H9.497m5.007 0a7.454 7.454 0 0 1-.982-3.172M9.497 14.25a7.454 7.454 0 0 0 .981-3.172M5.25 4.236c-.982.143-1.954.317-2.916.52A6.003 6.003 0 0 0 7.73 9.728M5.25 4.236V4.5c0 2.108.966 3.99 2.48 5.228M5.25 4.236V2.721C7.456 2.41 9.71 2.25 12 2.25c2.291 0 4.545.16 6.75.47v1.516M7.73 9.728a6.726 6.726 0 0 0 2.748 1.35m8.272-6.842V4.5c0 2.108-.966 3.99-2.48 5.228m2.48-5.492a46.32 46.32 0 0 1 2.916.52 6.003 6.003 0 0 1-5.395 4.972m0 0a6.726 6.726 0 0 1-2.749 1.35m0 0a6.772 6.772 0 0 1-2.927 0" />
  </svg>
);

const ArrowRightIcon = () => (
  <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={2} stroke="currentColor" className="w-4 h-4">
    <path strokeLinecap="round" strokeLinejoin="round" d="M13.5 4.5 21 12m0 0-7.5 7.5M21 12H3" />
  </svg>
);

// Game history and detailed info
const gameDetails: Record<string, { history: string; facts: string[]; applications: string[] }> = {
  dilemma: {
    history: `Дилемма заключённого — одна из самых знаменитых задач теории игр, придуманная в 1950 году математиками Мерриллом Фладом и Мелвином Дрешером в корпорации RAND. Название придумал Альберт Такер, который представил задачу в виде истории о двух преступниках.

Представьте: полиция арестовала двух подозреваемых, но у следствия недостаточно улик. Их разводят по разным камерам и предлагают сделку: предать подельника в обмен на свободу. Если оба молчат — получат минимальный срок. Если оба предают — средний срок. Но если один предаёт, а другой молчит — предатель выходит на свободу, а молчун получает максимальный срок.

Парадокс в том, что рационально каждому выгодно предать, но если оба так поступят — оба проиграют. Эта простая модель объясняет, почему сотрудничество так сложно достичь, даже когда оно выгодно всем.`,
    facts: [
      'В 1980-х годах политолог Роберт Аксельрод провёл компьютерный турнир стратегий — победила простейшая «Око за око» (Tit for Tat)',
      'Дилемма заключённого используется для объяснения гонки вооружений между СССР и США',
      'Биологи применяют эту модель для изучения альтруизма у животных и эволюции кооперации',
      'В 2012 году два игрока на британском шоу «Golden Balls» обманули систему, договорившись заранее разделить выигрыш'
    ],
    applications: [
      'Международные отношения и договоры о разоружении',
      'Экология: почему страны не могут договориться о сокращении выбросов',
      'Бизнес: ценовые войны между конкурентами',
      'Эволюционная биология: как возникает сотрудничество в природе'
    ]
  },
  tug_of_war: {
    history: `Игра «Перетягивание каната» в теории игр — это модель конфликта за ограниченные ресурсы, известная как «полковничий блото» (Colonel Blotto). Её придумал французский математик Эмиль Борель в 1921 году.

Оригинальная задача звучала так: два полковника должны распределить своих солдат по нескольким полям сражения. На каждом поле побеждает тот, у кого больше войск. Побеждает тот, кто выиграет больше полей.

Красота этой игры в том, что здесь нет единственной «лучшей» стратегии. Любое распределение можно победить другим распределением. Это делает игру похожей на «камень-ножницы-бумага», только гораздо сложнее.

В нашей версии вместо солдат — единицы силы, а вместо полей сражения — раунды перетягивания каната.`,
    facts: [
      'Задача Colonel Blotto до сих пор не имеет полного математического решения для произвольного числа полей',
      'Эта модель активно используется в политологии для анализа избирательных кампаний',
      'В 2006 году математики доказали, что в эту игру оптимально играть случайно — используя рандомизированные стратегии',
      'Перетягивание каната было олимпийским видом спорта с 1900 по 1920 год'
    ],
    applications: [
      'Распределение рекламного бюджета по регионам',
      'Военная стратегия и распределение войск',
      'Спортивные турниры с несколькими раундами',
      'Конкурентная борьба компаний на разных рынках'
    ]
  }
};

// Modal component
function GameInfoModal({
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

// Prisoner's Dilemma Matrix Component
function PrisonersDilemmaMatrix() {
  const [hoveredCell, setHoveredCell] = useState<string | null>(null);

  const cellInfo: Record<string, { title: string; desc: string }> = {
    'cc': { title: 'Взаимное сотрудничество', desc: 'Оба игрока выигрывают' },
    'cd': { title: 'A сотрудничает, B предаёт', desc: 'B получает максимум, A — ничего' },
    'dc': { title: 'A предаёт, B сотрудничает', desc: 'A получает максимум, B — ничего' },
    'dd': { title: 'Равновесие Нэша', desc: 'Оба проигрывают, но это стабильная стратегия' },
  };

  const cellSize = 'w-40 h-28';
  const fontSize = 'text-3xl';

  return (
    <div className="w-full h-full flex items-center justify-center">
      <table className="border-separate border-spacing-0">
        <thead>
          <tr>
            <th></th>
            <th></th>
            <th colSpan={2} className="pb-2">
              <span className="text-lg font-bold text-blue-400">Игрок B</span>
            </th>
          </tr>
          <tr>
            <th></th>
            <th></th>
            <th className="w-40 pb-2 text-center text-base font-semibold text-emerald-400">
              Сотрудничать
            </th>
            <th className="w-40 pb-2 text-center text-base font-semibold text-red-400">
              Предать
            </th>
          </tr>
        </thead>
        <tbody>
          <tr>
            <td rowSpan={2} className="pr-2 align-middle">
              <span
                className="text-lg font-bold text-purple-400"
                style={{ writingMode: 'vertical-rl', transform: 'rotate(180deg)' }}
              >
                Игрок A
              </span>
            </td>
            <td className="pr-3 text-right text-base font-semibold text-emerald-400 align-middle">
              Сотр.
            </td>
            <td
              className={`${cellSize} cursor-pointer transition-[filter,transform,box-shadow] duration-200 bg-emerald-500 rounded-tl-xl ${hoveredCell === 'cc' ? 'brightness-110 scale-105 shadow-xl z-10' : 'hover:brightness-105'}`}
              onMouseEnter={() => setHoveredCell('cc')}
              onMouseLeave={() => setHoveredCell(null)}
            >
              <div className="w-full h-full flex items-center justify-center">
                <span className={`font-mono font-bold ${fontSize} text-white`}>3, 3</span>
              </div>
            </td>
            <td
              className={`${cellSize} cursor-pointer transition-[filter,transform,box-shadow] duration-200 bg-red-500 rounded-tr-xl ${hoveredCell === 'cd' ? 'brightness-110 scale-105 shadow-xl z-10' : 'hover:brightness-105'}`}
              onMouseEnter={() => setHoveredCell('cd')}
              onMouseLeave={() => setHoveredCell(null)}
            >
              <div className="w-full h-full flex items-center justify-center">
                <span className={`font-mono font-bold ${fontSize} text-white`}>0, 5</span>
              </div>
            </td>
          </tr>
          <tr>
            <td className="pr-3 text-right text-base font-semibold text-red-400 align-middle">
              Пред.
            </td>
            <td
              className={`${cellSize} cursor-pointer transition-[filter,transform,box-shadow] duration-200 bg-red-500 rounded-bl-xl ${hoveredCell === 'dc' ? 'brightness-110 scale-105 shadow-xl z-10' : 'hover:brightness-105'}`}
              onMouseEnter={() => setHoveredCell('dc')}
              onMouseLeave={() => setHoveredCell(null)}
            >
              <div className="w-full h-full flex items-center justify-center">
                <span className={`font-mono font-bold ${fontSize} text-white`}>5, 0</span>
              </div>
            </td>
            <td
              className={`${cellSize} cursor-pointer transition-[filter,transform,box-shadow] duration-200 bg-amber-500 rounded-br-xl relative ${hoveredCell === 'dd' ? 'brightness-110 scale-105 shadow-xl z-10' : 'hover:brightness-105'}`}
              onMouseEnter={() => setHoveredCell('dd')}
              onMouseLeave={() => setHoveredCell(null)}
            >
              <div className="w-full h-full flex items-center justify-center">
                <span className={`font-mono font-bold ${fontSize} text-white`}>1, 1</span>
              </div>
              <div className="absolute top-2 right-2 w-4 h-4 bg-cyan-400 rounded-full" title="Равновесие Нэша" />
            </td>
          </tr>
          <tr>
            <td></td>
            <td></td>
            <td colSpan={2} className="pt-4">
              <div className={`text-center transition-opacity duration-200 h-12 ${hoveredCell ? 'opacity-100' : 'opacity-50'}`}>
                {hoveredCell ? (
                  <>
                    <div className="text-base font-semibold text-gray-200">{cellInfo[hoveredCell].title}</div>
                    <div className="text-sm text-gray-400">{cellInfo[hoveredCell].desc}</div>
                  </>
                ) : (
                  <div className="text-sm text-gray-500">Наведите на ячейку</div>
                )}
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  );
}

// Tug of War Visualization
function TugOfWarVisualization() {
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

// Game Showcase Component with tabs
function GameShowcase() {
  const [activeGame, setActiveGame] = useState(0);
  const [modalOpen, setModalOpen] = useState(false);
  useEscapeKey(useCallback(() => setModalOpen(false), []), modalOpen);

  const games = [
    {
      id: 'dilemma',
      name: 'Дилемма заключённого',
      icon: '🤝',
      color: 'purple',
      description: 'Классическая задача теории игр, демонстрирующая конфликт между индивидуальной и коллективной рациональностью.',
      rules: [
        { text: 'Взаимное сотрудничество', result: 'оба получают по 3 очка', color: 'green' },
        { text: 'Предательство', result: 'предатель получает 5, жертва — 0', color: 'red' },
        { text: 'Взаимное предательство', result: 'оба получают по 1 очку', color: 'yellow' },
      ],
      insight: 'Равновесие Нэша: взаимное предательство — ни один игрок не может улучшить результат в одностороннем порядке.',
      visualization: <PrisonersDilemmaMatrix />,
    },
    {
      id: 'tug_of_war',
      name: 'Перетягивание каната',
      icon: '🪢',
      color: 'green',
      description: 'Стратегическая игра на распределение ресурсов. Распределите силы по раундам, чтобы победить.',
      rules: [
        { text: 'У каждого игрока 100 единиц силы', result: 'на все раунды', color: 'blue' },
        { text: 'В каждом раунде выигрывает', result: 'кто выделил больше силы', color: 'green' },
        { text: 'Побеждает тот, кто выиграл', result: 'больше раундов', color: 'purple' },
      ],
      insight: 'Ключ к победе — предугадать стратегию противника и оптимально распределить ресурсы.',
      visualization: <TugOfWarVisualization />,
    },
  ];

  const currentGame = games[activeGame];
  const colorClasses: Record<string, { bg: string; text: string; border: string }> = {
    purple: {
      bg: 'bg-primary-900/30',
      text: 'text-primary-400',
      border: 'border-primary-700',
    },
    green: {
      bg: 'bg-green-900/30',
      text: 'text-green-400',
      border: 'border-green-700',
    },
    blue: {
      bg: 'bg-blue-900/30',
      text: 'text-blue-400',
      border: 'border-blue-700',
    },
    orange: {
      bg: 'bg-orange-900/30',
      text: 'text-orange-400',
      border: 'border-orange-700',
    },
    yellow: {
      bg: 'bg-yellow-900/30',
      text: 'text-yellow-400',
      border: 'border-yellow-700',
    },
    red: {
      bg: 'bg-red-900/30',
      text: 'text-red-400',
      border: 'border-red-700',
    },
  };

  return (
    <div className="space-y-6">
      {/* Game tabs */}
      <div className="flex flex-wrap justify-center gap-3 md:gap-4">
        {games.map((game, index) => (
          <button
            key={game.id}
            onClick={() => setActiveGame(index)}
            className={`flex items-center gap-2 px-4 py-2 rounded-xl font-medium transition-colors ${
              activeGame === index
                ? `${colorClasses[game.color].bg} ${colorClasses[game.color].text} ${colorClasses[game.color].border} border-2 shadow-lg scale-105`
                : 'bg-gray-800 text-gray-300 hover:bg-gray-700 border-2 border-transparent'
            }`}
          >
            <span className="text-xl">{game.icon}</span>
            <span className="hidden sm:inline">{game.name}</span>
          </button>
        ))}
      </div>

      {/* Game content */}
      <div className="grid md:grid-cols-2 gap-8 items-start">
        <div className="space-y-4" key={currentGame.id}>
          <div className="flex items-center gap-3">
            <span className="text-4xl">{currentGame.icon}</span>
            <h2 className={`text-2xl md:text-3xl font-bold ${colorClasses[currentGame.color].text}`}>
              {currentGame.name}
            </h2>
          </div>

          <p className="text-gray-300 leading-relaxed">
            {currentGame.description}
          </p>

          <div className="space-y-3">
            {currentGame.rules.map((rule, index) => (
              <div key={index} className="flex items-center gap-2">
                <div className="w-3 h-3 rounded-full"
                  style={{ backgroundColor: rule.color === 'green' ? '#22c55e' : rule.color === 'red' ? '#ef4444' : rule.color === 'yellow' ? '#eab308' : rule.color === 'blue' ? '#3b82f6' : '#a855f7' }}
                />
                <span className="text-gray-300 text-sm">
                  <strong>{rule.text}</strong> — {rule.result}
                </span>
              </div>
            ))}
          </div>

          <div className={`p-3 rounded-lg border ${colorClasses[currentGame.color].bg} ${colorClasses[currentGame.color].border}`}>
            <p className={`text-sm ${colorClasses[currentGame.color].text}`}>
              <strong>Инсайт:</strong> {currentGame.insight}
            </p>
          </div>

          <button
            onClick={() => setModalOpen(true)}
            className="inline-flex items-center gap-2 text-sm font-medium text-gray-400 hover:text-primary-400 transition-colors group"
          >
            <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            Подробнее об игре
            <svg className="w-3 h-3 group-hover:translate-x-1 transition-transform" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
            </svg>
          </button>
        </div>

        {/* Visualization */}
        <div className="flex justify-center items-center bg-gray-800/50 rounded-2xl p-4 border border-gray-700 transition-colors">
          <div className="w-full animate-fade-in" key={currentGame.id + '-viz'}>
            {currentGame.visualization}
          </div>
        </div>
      </div>

      <GameInfoModal
        isOpen={modalOpen}
        onClose={() => setModalOpen(false)}
        gameId={currentGame.id}
        gameName={currentGame.name}
        gameIcon={currentGame.icon}
      />
    </div>
  );
}

// Concept Card Component
function ConceptCard({
  title,
  author,
  year,
  description,
}: {
  title: string;
  author: string;
  year: string;
  description: string;
}) {
  return (
    <div className="card group transition-[box-shadow]" style={{ border: 'none' }}
      onMouseEnter={(e) => { e.currentTarget.style.boxShadow = '0 0 30px rgba(139,92,246,0.1)'; }}
      onMouseLeave={(e) => { e.currentTarget.style.boxShadow = 'none'; }}
    >
      <div className="flex items-start justify-between mb-3">
        <h3 className="text-lg font-bold text-gray-100">{title}</h3>
        <span className="text-xs font-mono bg-gray-800 px-2 py-1 rounded text-gray-400">
          {year}
        </span>
      </div>
      <p className="text-sm text-gray-300 mb-3">{description}</p>
      <p className="text-xs text-gray-500">— {author}</p>
    </div>
  );
}

export function Home() {
  const heroRef = useRef<HTMLDivElement>(null);
  // Scroll reveal observer
  const sectionsRef = useRef<(HTMLDivElement | null)[]>([]);
  useEffect(() => {
    const observer = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          if (entry.isIntersecting) {
            entry.target.classList.add('revealed');
          }
        });
      },
      { threshold: 0.1 }
    );
    sectionsRef.current.forEach((el) => {
      if (el) observer.observe(el);
    });
    return () => observer.disconnect();
  }, []);

  return (
    <div className="space-y-16">
      {/* Hero Section — Dark with glow orbs, no border */}
      <div ref={heroRef} className="relative overflow-visible rounded-3xl p-8 md:p-12" style={{ background: 'rgba(17,24,39,0.4)' }}>
        {/* Clip wrapper for glow orbs and grid (prevents bleed) */}
        <div className="absolute inset-0 overflow-hidden rounded-3xl pointer-events-none">
          {/* Glow orbs */}
          <div
            className="absolute -top-20 -left-20 w-72 h-72 rounded-full opacity-25 blur-3xl"
            style={{ background: 'radial-gradient(circle, rgba(139,92,246,0.6), transparent 70%)' }}
          />
          <div
            className="absolute -bottom-32 right-1/4 w-96 h-96 rounded-full opacity-15 blur-3xl"
            style={{ background: 'radial-gradient(circle, rgba(34,197,94,0.4), transparent 70%)' }}
          />

          {/* Pixel grid dots */}
          <PixelGrid heroRef={heroRef} />

          {/* Grid pattern */}
          <svg className="absolute inset-0 w-full h-full opacity-5">
            <defs>
              <pattern id="grid" width="40" height="40" patternUnits="userSpaceOnUse">
                <path d="M 40 0 L 0 0 0 40" fill="none" stroke="#22c55e" strokeWidth="0.5"/>
              </pattern>
            </defs>
            <rect width="100%" height="100%" fill="url(#grid)" />
          </svg>
        </div>

        {/* SpaceInvader mascot — bright, interactive, overflow-visible for jump */}
        <div className="hidden lg:block absolute right-8 top-1/2 -translate-y-1/2" style={{ zIndex: 60 }}>
          <SpaceInvader size="md" interactive />
        </div>

        {/* Content */}
        <div className="relative z-10 max-w-3xl mx-auto text-center">
          <div className="inline-block px-3 py-1 bg-primary-500/20 rounded-full text-sm font-medium text-primary-300 mb-4 border border-primary-500/30">
            Теория игр в действии
          </div>
          <h1 className="text-3xl md:text-5xl font-extrabold mb-4 leading-tight tracking-tight text-white">
            Соревнуйтесь в стратегическом мышлении
          </h1>
          <p className="text-lg text-gray-400 mb-8 leading-relaxed">
            TJudge — платформа для турниров по теории игр.
            Ваши алгоритмы сражаются друг с другом в стратегических задачах,
            где побеждает лучшая стратегия.
          </p>

          {/* Terminal Typewriter */}
          <div className="my-8">
            <TerminalTypewriter />
          </div>

          {/* CTA buttons */}
          <div className="flex flex-wrap justify-center gap-4">
            <Link
              to="/tournaments"
              className="btn btn-primary text-lg px-8 py-3"
            >
              <TrophyIcon />
              К турнирам
            </Link>
            <Link
              to="/games"
              className="btn btn-secondary text-lg px-8 py-3"
            >
              Правила игр
              <ArrowRightIcon />
            </Link>
          </div>
        </div>
      </div>

      {/* Game Showcase with tabs */}
      <div>
        <h2 className="text-2xl font-bold text-gray-100 mb-6 text-center">
          Игры на платформе
        </h2>
        <GameShowcase />
      </div>

      {/* Interactive Terminal Quest */}
      <div ref={(el) => { sectionsRef.current[0] = el; }} className="reveal-on-scroll">
        <TerminalQuest />
      </div>

      {/* Key Concepts */}
      <div ref={(el) => { sectionsRef.current[1] = el; }} className="reveal-on-scroll">
        <h2 className="text-2xl font-bold text-gray-100 mb-2 text-center">
          Ключевые концепции
        </h2>
        <p className="text-gray-400 text-center mb-8 max-w-2xl mx-auto">
          Теория игр — раздел математики, изучающий стратегические взаимодействия
          между рациональными агентами
        </p>

        <StaggerList className="grid md:grid-cols-3 gap-6">
          <StaggerItem>
            <ConceptCard
              title="Равновесие Нэша"
              author="Джон Нэш"
              year="1950"
              description="Состояние, при котором ни один игрок не может улучшить свой результат, изменив только свою стратегию."
            />
          </StaggerItem>
          <StaggerItem>
            <ConceptCard
              title="Оптимальность по Парето"
              author="Вильфредо Парето"
              year="1896"
              description="Состояние, при котором невозможно улучшить положение одного игрока, не ухудшив положение другого."
            />
          </StaggerItem>
          <StaggerItem>
            <ConceptCard
              title="Доминирующая стратегия"
              author="Теория игр"
              year="XX век"
              description="Стратегия, которая приносит лучший результат независимо от действий других игроков."
            />
          </StaggerItem>
        </StaggerList>
      </div>

      {/* CTA Section */}
      <div ref={(el) => { sectionsRef.current[2] = el; }} className="reveal-on-scroll text-center py-8">
        <h2 className="text-2xl font-bold text-gray-100 mb-4">
          Готовы проверить свою стратегию?
        </h2>
        <p className="text-gray-400 mb-6 max-w-xl mx-auto">
          Присоединяйтесь к активным турнирам и соревнуйтесь с другими участниками
        </p>
        <Link
          to="/tournaments"
          className="inline-flex items-center gap-2 btn btn-primary text-lg px-8 py-3"
        >
          <TrophyIcon />
          Смотреть турниры
        </Link>
      </div>
    </div>
  );
}
