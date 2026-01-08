import { Link } from 'react-router-dom';
import { useState } from 'react';

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

// Prisoner's Dilemma Matrix Component
function PrisonersDilemmaMatrix() {
  return (
    <div className="relative pt-10 pl-16">
      {/* Player B label - centered above matrix */}
      <div className="absolute top-0 left-16 right-0 text-center text-sm font-semibold text-gray-600 dark:text-gray-300">
        Игрок B
      </div>
      {/* Player A label - rotated on the left */}
      <div className="absolute left-0 top-10 bottom-0 flex items-center">
        <span className="-rotate-90 text-sm font-semibold text-gray-600 dark:text-gray-300 whitespace-nowrap">
          Игрок A
        </span>
      </div>

      {/* Matrix */}
      <div className="grid grid-cols-3 gap-0 text-center">
        {/* Header row */}
        <div className="p-3"></div>
        <div className="p-3 font-semibold text-blue-600 dark:text-blue-400 text-sm">Сотрудничать</div>
        <div className="p-3 font-semibold text-red-600 dark:text-red-400 text-sm">Предать</div>

        {/* Row 1: Cooperate */}
        <div className="p-3 font-semibold text-blue-600 dark:text-blue-400 text-sm flex items-center justify-end">Сотрудничать</div>
        <div className="p-4 bg-green-100 dark:bg-green-900/30 border border-green-200 dark:border-green-800 rounded-tl-lg">
          <span className="font-mono font-bold text-green-700 dark:text-green-400">3, 3</span>
        </div>
        <div className="p-4 bg-red-100 dark:bg-red-900/30 border border-red-200 dark:border-red-800 rounded-tr-lg">
          <span className="font-mono font-bold text-red-700 dark:text-red-400">0, 5</span>
        </div>

        {/* Row 2: Defect */}
        <div className="p-3 font-semibold text-red-600 dark:text-red-400 text-sm flex items-center justify-end">Предать</div>
        <div className="p-4 bg-red-100 dark:bg-red-900/30 border border-red-200 dark:border-red-800 rounded-bl-lg">
          <span className="font-mono font-bold text-red-700 dark:text-red-400">5, 0</span>
        </div>
        <div className="p-4 bg-yellow-100 dark:bg-yellow-900/30 border border-yellow-200 dark:border-yellow-800 rounded-br-lg group relative">
          <span className="font-mono font-bold text-yellow-700 dark:text-yellow-400">1, 1</span>
          <div className="absolute -top-1 -right-1 w-3 h-3 bg-primary-500 rounded-full animate-pulse" title="Равновесие Нэша" />
        </div>
      </div>
    </div>
  );
}

// Tug of War Visualization
function TugOfWarVisualization() {
  return (
    <div className="relative h-48 flex flex-col justify-center">
      {/* Rope */}
      <div className="relative">
        {/* Center marker */}
        <div className="absolute left-1/2 -translate-x-1/2 -top-6 text-xs font-semibold text-gray-500 dark:text-gray-400">
          Центр
        </div>
        <div className="absolute left-1/2 -translate-x-1/2 w-0.5 h-4 -top-2 bg-gray-400 dark:bg-gray-500" />

        {/* Rope line */}
        <div className="h-3 bg-gradient-to-r from-amber-600 via-amber-500 to-amber-600 rounded-full shadow-inner relative overflow-hidden">
          {/* Rope texture */}
          <div className="absolute inset-0 opacity-30" style={{
            backgroundImage: 'repeating-linear-gradient(90deg, transparent, transparent 4px, rgba(0,0,0,0.2) 4px, rgba(0,0,0,0.2) 8px)'
          }} />
          {/* Animated marker */}
          <div className="absolute top-0 bottom-0 w-2 bg-red-500 rounded-full left-1/2 -translate-x-1/2 animate-pulse shadow-lg" />
        </div>

        {/* Players */}
        <div className="flex justify-between mt-4">
          <div className="flex items-center gap-2">
            <div className="w-10 h-10 bg-blue-500 rounded-full flex items-center justify-center text-white font-bold shadow-lg">
              A
            </div>
            <div className="text-sm text-gray-600 dark:text-gray-300">
              <div className="font-semibold">Игрок A</div>
              <div className="text-xs text-gray-500">Сила: 100</div>
            </div>
          </div>
          <div className="flex items-center gap-2">
            <div className="text-sm text-gray-600 dark:text-gray-300 text-right">
              <div className="font-semibold">Игрок B</div>
              <div className="text-xs text-gray-500">Сила: 100</div>
            </div>
            <div className="w-10 h-10 bg-red-500 rounded-full flex items-center justify-center text-white font-bold shadow-lg">
              B
            </div>
          </div>
        </div>
      </div>

      {/* Rounds indicator */}
      <div className="flex justify-center gap-2 mt-6">
        {[1, 2, 3, 4, 5].map((round) => (
          <div
            key={round}
            className={`w-8 h-8 rounded-lg flex items-center justify-center text-xs font-bold transition-all ${
              round <= 3
                ? 'bg-primary-100 dark:bg-primary-900/50 text-primary-700 dark:text-primary-300'
                : 'bg-gray-100 dark:bg-gray-800 text-gray-400'
            }`}
          >
            {round}
          </div>
        ))}
      </div>
      <div className="text-center text-xs text-gray-500 dark:text-gray-400 mt-2">
        Раунды
      </div>
    </div>
  );
}

// Good Deal Visualization
function GoodDealVisualization() {
  return (
    <div className="relative h-48 flex flex-col justify-center items-center">
      {/* Trading visualization */}
      <div className="flex items-center gap-8">
        {/* Player A */}
        <div className="text-center">
          <div className="w-16 h-16 bg-gradient-to-br from-blue-400 to-blue-600 rounded-2xl flex items-center justify-center text-white text-2xl font-bold shadow-lg mb-2">
            A
          </div>
          <div className="text-sm font-semibold text-gray-700 dark:text-gray-200">Продавец</div>
        </div>

        {/* Deal animation */}
        <div className="relative w-32">
          {/* Arrows */}
          <div className="flex flex-col gap-2">
            <div className="flex items-center justify-center">
              <div className="text-2xl animate-bounce">💰</div>
              <svg className="w-8 h-4 text-green-500" fill="none" viewBox="0 0 24 12">
                <path d="M0 6h20m0 0l-4-4m4 4l-4 4" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
              </svg>
            </div>
            <div className="flex items-center justify-center">
              <svg className="w-8 h-4 text-purple-500 rotate-180" fill="none" viewBox="0 0 24 12">
                <path d="M0 6h20m0 0l-4-4m4 4l-4 4" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
              </svg>
              <div className="text-2xl animate-bounce" style={{ animationDelay: '0.2s' }}>📦</div>
            </div>
          </div>

          {/* Price display */}
          <div className="absolute -bottom-8 left-1/2 -translate-x-1/2 bg-green-100 dark:bg-green-900/50 px-3 py-1 rounded-full">
            <span className="text-sm font-bold text-green-700 dark:text-green-300">Цена: ?</span>
          </div>
        </div>

        {/* Player B */}
        <div className="text-center">
          <div className="w-16 h-16 bg-gradient-to-br from-red-400 to-red-600 rounded-2xl flex items-center justify-center text-white text-2xl font-bold shadow-lg mb-2">
            B
          </div>
          <div className="text-sm font-semibold text-gray-700 dark:text-gray-200">Покупатель</div>
        </div>
      </div>

      {/* Negotiation bar */}
      <div className="w-full max-w-xs mt-12">
        <div className="flex justify-between text-xs text-gray-500 dark:text-gray-400 mb-1">
          <span>0</span>
          <span>Цена сделки</span>
          <span>100</span>
        </div>
        <div className="h-2 bg-gray-200 dark:bg-gray-700 rounded-full overflow-hidden">
          <div className="h-full w-1/2 bg-gradient-to-r from-blue-500 to-red-500 rounded-full relative">
            <div className="absolute right-0 top-1/2 -translate-y-1/2 w-4 h-4 bg-white dark:bg-gray-900 rounded-full border-2 border-primary-500 shadow-lg" />
          </div>
        </div>
      </div>
    </div>
  );
}

// Balance of Universe Visualization
function BalanceVisualization() {
  return (
    <div className="relative h-48 flex flex-col justify-center items-center">
      {/* Balance scale */}
      <div className="relative w-64">
        {/* Center pivot */}
        <div className="absolute left-1/2 -translate-x-1/2 -top-2 w-4 h-4 bg-gray-400 dark:bg-gray-500 rounded-full z-10" />

        {/* Balance beam - animated tilt */}
        <div className="relative h-2 bg-gradient-to-r from-gray-400 via-gray-300 to-gray-400 rounded-full transform origin-center animate-pulse"
          style={{ animation: 'tilt 3s ease-in-out infinite' }}>
        </div>

        {/* Left pan */}
        <div className="absolute -left-4 top-4">
          <div className="w-1 h-8 bg-gray-400 mx-auto" />
          <div className="w-20 h-3 bg-gradient-to-b from-amber-400 to-amber-600 rounded-b-lg shadow-lg flex items-end justify-center">
            <div className="flex gap-1 -mb-6">
              <div className="w-4 h-4 bg-blue-500 rounded-full shadow animate-bounce" style={{ animationDelay: '0s' }} />
              <div className="w-4 h-4 bg-blue-400 rounded-full shadow animate-bounce" style={{ animationDelay: '0.1s' }} />
            </div>
          </div>
          <div className="text-center mt-8 text-xs font-semibold text-blue-600 dark:text-blue-400">Порядок</div>
        </div>

        {/* Right pan */}
        <div className="absolute -right-4 top-4">
          <div className="w-1 h-8 bg-gray-400 mx-auto" />
          <div className="w-20 h-3 bg-gradient-to-b from-amber-400 to-amber-600 rounded-b-lg shadow-lg flex items-end justify-center">
            <div className="flex gap-1 -mb-6">
              <div className="w-4 h-4 bg-red-500 rounded-full shadow animate-bounce" style={{ animationDelay: '0.2s' }} />
              <div className="w-4 h-4 bg-red-400 rounded-full shadow animate-bounce" style={{ animationDelay: '0.3s' }} />
            </div>
          </div>
          <div className="text-center mt-8 text-xs font-semibold text-red-600 dark:text-red-400">Хаос</div>
        </div>

        {/* Stand */}
        <div className="absolute left-1/2 -translate-x-1/2 top-0 w-2 h-20 bg-gradient-to-b from-gray-400 to-gray-500 rounded-b-lg" />
        <div className="absolute left-1/2 -translate-x-1/2 top-20 w-12 h-2 bg-gray-500 rounded-lg" />
      </div>

      {/* Equilibrium indicator */}
      <div className="mt-16 flex items-center gap-2">
        <div className="w-3 h-3 bg-primary-500 rounded-full animate-pulse" />
        <span className="text-sm text-gray-600 dark:text-gray-300">Ищите баланс между крайностями</span>
      </div>
    </div>
  );
}

// Game Showcase Component with tabs
function GameShowcase() {
  const [activeGame, setActiveGame] = useState(0);

  const games = [
    {
      id: 'prisoners_dilemma',
      name: 'Дилемма заключённого',
      icon: '🤝',
      color: 'blue',
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
    {
      id: 'good_deal',
      name: 'Выгодная сделка',
      icon: '💰',
      color: 'purple',
      description: 'Торговая игра, где нужно договориться о цене. Продавец хочет дороже, покупатель — дешевле.',
      rules: [
        { text: 'Продавец называет минимальную цену', result: 'за которую готов продать', color: 'blue' },
        { text: 'Покупатель называет максимальную', result: 'за которую готов купить', color: 'red' },
        { text: 'Если цены пересекаются', result: 'сделка состоится', color: 'green' },
      ],
      insight: 'Сделка выгодна обоим, если существует зона согласия между минимумом продавца и максимумом покупателя.',
      visualization: <GoodDealVisualization />,
    },
    {
      id: 'balance_of_universe',
      name: 'Баланс вселенной',
      icon: '⚖️',
      color: 'orange',
      description: 'Философская игра о поиске равновесия. Игроки влияют на баланс между порядком и хаосом.',
      rules: [
        { text: 'Каждый ход меняет баланс', result: 'в сторону порядка или хаоса', color: 'blue' },
        { text: 'Очки начисляются за', result: 'поддержание равновесия', color: 'green' },
        { text: 'Крайности приводят к', result: 'потере очков для всех', color: 'red' },
      ],
      insight: 'Истинная победа — в сотрудничестве для поддержания баланса, а не в доминировании.',
      visualization: <BalanceVisualization />,
    },
  ];

  const currentGame = games[activeGame];
  const colorClasses: Record<string, { bg: string; text: string; border: string }> = {
    blue: {
      bg: 'bg-blue-100 dark:bg-blue-900/30',
      text: 'text-blue-600 dark:text-blue-400',
      border: 'border-blue-200 dark:border-blue-800',
    },
    green: {
      bg: 'bg-green-100 dark:bg-green-900/30',
      text: 'text-green-600 dark:text-green-400',
      border: 'border-green-200 dark:border-green-800',
    },
    purple: {
      bg: 'bg-purple-100 dark:bg-purple-900/30',
      text: 'text-purple-600 dark:text-purple-400',
      border: 'border-purple-200 dark:border-purple-800',
    },
    orange: {
      bg: 'bg-orange-100 dark:bg-orange-900/30',
      text: 'text-orange-600 dark:text-orange-400',
      border: 'border-orange-200 dark:border-orange-800',
    },
    yellow: {
      bg: 'bg-yellow-100 dark:bg-yellow-900/30',
      text: 'text-yellow-600 dark:text-yellow-400',
      border: 'border-yellow-200 dark:border-yellow-800',
    },
    red: {
      bg: 'bg-red-100 dark:bg-red-900/30',
      text: 'text-red-600 dark:text-red-400',
      border: 'border-red-200 dark:border-red-800',
    },
  };

  return (
    <div className="space-y-6">
      {/* Game tabs */}
      <div className="flex flex-wrap justify-center gap-2">
        {games.map((game, index) => (
          <button
            key={game.id}
            onClick={() => setActiveGame(index)}
            className={`flex items-center gap-2 px-4 py-2 rounded-xl font-medium transition-all ${
              activeGame === index
                ? `${colorClasses[game.color].bg} ${colorClasses[game.color].text} ${colorClasses[game.color].border} border-2 shadow-lg scale-105`
                : 'bg-gray-100 dark:bg-gray-800 text-gray-600 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-700 border-2 border-transparent'
            }`}
          >
            <span className="text-xl">{game.icon}</span>
            <span className="hidden sm:inline">{game.name}</span>
          </button>
        ))}
      </div>

      {/* Game content */}
      <div className="grid md:grid-cols-2 gap-8 items-start">
        {/* Description */}
        <div className="space-y-4" key={currentGame.id}>
          <div className="flex items-center gap-3">
            <span className="text-4xl">{currentGame.icon}</span>
            <h2 className={`text-2xl md:text-3xl font-bold ${colorClasses[currentGame.color].text}`}>
              {currentGame.name}
            </h2>
          </div>

          <p className="text-gray-600 dark:text-gray-300 leading-relaxed">
            {currentGame.description}
          </p>

          <div className="space-y-3">
            {currentGame.rules.map((rule, index) => (
              <div key={index} className="flex items-center gap-2">
                <div className={`w-3 h-3 rounded-full ${colorClasses[rule.color].bg.replace('/30', '')} ${colorClasses[rule.color].text.includes('dark:') ? '' : ''}`}
                  style={{ backgroundColor: rule.color === 'green' ? '#22c55e' : rule.color === 'red' ? '#ef4444' : rule.color === 'yellow' ? '#eab308' : rule.color === 'blue' ? '#3b82f6' : '#a855f7' }}
                />
                <span className="text-gray-700 dark:text-gray-300 text-sm">
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
        </div>

        {/* Visualization */}
        <div className="flex justify-center items-center min-h-[280px] bg-gray-50 dark:bg-gray-800/50 rounded-2xl p-6 transition-all">
          <div className="w-full animate-fade-in" key={currentGame.id + '-viz'}>
            {currentGame.visualization}
          </div>
        </div>
      </div>
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
    <div className="card group hover:shadow-lg dark:hover:shadow-black/30 transition-all">
      <div className="flex items-start justify-between mb-3">
        <h3 className="text-lg font-bold text-gray-900 dark:text-gray-100">{title}</h3>
        <span className="text-xs font-mono bg-gray-100 dark:bg-gray-800 px-2 py-1 rounded text-gray-500 dark:text-gray-400">
          {year}
        </span>
      </div>
      <p className="text-sm text-gray-600 dark:text-gray-300 mb-3">{description}</p>
      <p className="text-xs text-gray-500 dark:text-gray-400">— {author}</p>
    </div>
  );
}

export function Home() {
  return (
    <div className="space-y-16">
      {/* Hero Section */}
      <div className="relative overflow-hidden rounded-3xl bg-gradient-to-br from-primary-600 via-primary-700 to-indigo-800 p-8 md:p-12 text-white">
        {/* Game Theory Network Background */}
        <div className="absolute inset-0 overflow-hidden">
          {/* Grid pattern */}
          <svg className="absolute inset-0 w-full h-full opacity-5" xmlns="http://www.w3.org/2000/svg">
            <defs>
              <pattern id="grid" width="40" height="40" patternUnits="userSpaceOnUse">
                <path d="M 40 0 L 0 0 0 40" fill="none" stroke="white" strokeWidth="1"/>
              </pattern>
            </defs>
            <rect width="100%" height="100%" fill="url(#grid)" />
          </svg>

          {/* Network nodes and connections */}
          <svg className="absolute inset-0 w-full h-full" viewBox="0 0 800 400" preserveAspectRatio="xMidYMid slice">
            {/* Connection lines */}
            <g className="opacity-20">
              <line x1="650" y1="80" x2="720" y2="150" stroke="white" strokeWidth="2" />
              <line x1="720" y1="150" x2="680" y2="250" stroke="white" strokeWidth="2" />
              <line x1="680" y1="250" x2="750" y2="320" stroke="white" strokeWidth="2" />
              <line x1="650" y1="80" x2="580" y2="140" stroke="white" strokeWidth="2" />
              <line x1="580" y1="140" x2="620" y2="220" stroke="white" strokeWidth="2" />
              <line x1="620" y1="220" x2="680" y2="250" stroke="white" strokeWidth="2" />
              <line x1="580" y1="140" x2="520" y2="200" stroke="white" strokeWidth="2" />
              <line x1="720" y1="150" x2="780" y2="200" stroke="white" strokeWidth="2" />
            </g>

            {/* Nodes */}
            <g className="opacity-30">
              <circle cx="650" cy="80" r="12" fill="white" />
              <circle cx="720" cy="150" r="16" fill="white" />
              <circle cx="680" cy="250" r="14" fill="white" />
              <circle cx="750" cy="320" r="10" fill="white" />
              <circle cx="580" cy="140" r="10" fill="white" />
              <circle cx="620" cy="220" r="8" fill="white" />
              <circle cx="520" cy="200" r="6" fill="white" />
              <circle cx="780" cy="200" r="8" fill="white" />
            </g>

            {/* Animated pulsing node */}
            <circle cx="720" cy="150" r="16" fill="none" stroke="white" strokeWidth="2" opacity="0.4">
              <animate attributeName="r" values="16;24;16" dur="2s" repeatCount="indefinite" />
              <animate attributeName="opacity" values="0.4;0;0.4" dur="2s" repeatCount="indefinite" />
            </circle>
          </svg>

          {/* Payoff matrix hint */}
          <div className="absolute bottom-8 right-8 opacity-10 hidden lg:block">
            <div className="grid grid-cols-2 gap-1 text-4xl font-mono font-bold">
              <div className="w-16 h-16 bg-white/20 rounded flex items-center justify-center">3,3</div>
              <div className="w-16 h-16 bg-white/20 rounded flex items-center justify-center">0,5</div>
              <div className="w-16 h-16 bg-white/20 rounded flex items-center justify-center">5,0</div>
              <div className="w-16 h-16 bg-white/20 rounded flex items-center justify-center">1,1</div>
            </div>
          </div>

          {/* Floating symbols */}
          <div className="absolute top-12 right-20 text-6xl opacity-10 animate-pulse">∑</div>
          <div className="absolute bottom-20 right-40 text-5xl opacity-10">∞</div>
          <div className="absolute top-1/3 right-1/4 text-4xl opacity-10">≠</div>
        </div>

        <div className="relative z-10 max-w-3xl">
          <div className="inline-block px-3 py-1 bg-white/20 rounded-full text-sm font-medium mb-4 backdrop-blur-sm">
            Теория игр в действии
          </div>
          <h1 className="text-3xl md:text-5xl font-extrabold mb-4 leading-tight tracking-tight">
            Соревнуйтесь в стратегическом мышлении
          </h1>
          <p className="text-lg md:text-xl text-white/80 mb-8 leading-relaxed">
            TJudge — платформа для турниров по теории игр.
            Ваши алгоритмы сражаются друг с другом в классических задачах:
            дилемма заключённого, перетягивание каната и другие.
          </p>

          <div className="flex flex-wrap gap-4">
            <Link
              to="/tournaments"
              className="inline-flex items-center gap-2 px-6 py-3 bg-white text-primary-700 font-semibold rounded-xl hover:bg-gray-100 transition-colors shadow-lg shadow-black/20"
            >
              <TrophyIcon />
              К турнирам
            </Link>
            <Link
              to="/games"
              className="inline-flex items-center gap-2 px-6 py-3 bg-white/10 text-white font-semibold rounded-xl hover:bg-white/20 transition-colors border border-white/30 backdrop-blur-sm"
            >
              Правила игр
              <ArrowRightIcon />
            </Link>
          </div>
        </div>
      </div>

      {/* Game Showcase with tabs */}
      <div>
        <h2 className="text-2xl font-bold text-gray-900 dark:text-gray-100 mb-6 text-center">
          Игры на платформе
        </h2>
        <GameShowcase />
      </div>

      {/* Key Concepts */}
      <div>
        <h2 className="text-2xl font-bold text-gray-900 dark:text-gray-100 mb-2 text-center">
          Ключевые концепции
        </h2>
        <p className="text-gray-600 dark:text-gray-400 text-center mb-8 max-w-2xl mx-auto">
          Теория игр — раздел математики, изучающий стратегические взаимодействия
          между рациональными агентами
        </p>

        <div className="grid md:grid-cols-3 gap-6">
          <ConceptCard
            title="Равновесие Нэша"
            author="Джон Нэш"
            year="1950"
            description="Состояние, при котором ни один игрок не может улучшить свой результат, изменив только свою стратегию."
          />
          <ConceptCard
            title="Оптимальность по Парето"
            author="Вильфредо Парето"
            year="1896"
            description="Состояние, при котором невозможно улучшить положение одного игрока, не ухудшив положение другого."
          />
          <ConceptCard
            title="Доминирующая стратегия"
            author="Теория игр"
            year="XX век"
            description="Стратегия, которая приносит лучший результат независимо от действий других игроков."
          />
        </div>
      </div>

      {/* How it works */}
      <div className="bg-gray-100 dark:bg-gray-800/50 rounded-2xl p-8">
        <h2 className="text-2xl font-bold text-gray-900 dark:text-gray-100 mb-8 text-center">
          Как это работает
        </h2>

        <div className="grid md:grid-cols-4 gap-6">
          <div className="text-center">
            <div className="w-12 h-12 bg-primary-600 text-white rounded-xl flex items-center justify-center text-xl font-bold mx-auto mb-3">
              1
            </div>
            <h3 className="font-semibold text-gray-900 dark:text-gray-100 mb-2">Создайте команду</h3>
            <p className="text-sm text-gray-600 dark:text-gray-400">
              Соберите команду или участвуйте индивидуально
            </p>
          </div>

          <div className="text-center">
            <div className="w-12 h-12 bg-primary-600 text-white rounded-xl flex items-center justify-center text-xl font-bold mx-auto mb-3">
              2
            </div>
            <h3 className="font-semibold text-gray-900 dark:text-gray-100 mb-2">Напишите стратегию</h3>
            <p className="text-sm text-gray-600 dark:text-gray-400">
              Разработайте алгоритм принятия решений
            </p>
          </div>

          <div className="text-center">
            <div className="w-12 h-12 bg-primary-600 text-white rounded-xl flex items-center justify-center text-xl font-bold mx-auto mb-3">
              3
            </div>
            <h3 className="font-semibold text-gray-900 dark:text-gray-100 mb-2">Загрузите программу</h3>
            <p className="text-sm text-gray-600 dark:text-gray-400">
              Отправьте код на платформу для участия
            </p>
          </div>

          <div className="text-center">
            <div className="w-12 h-12 bg-primary-600 text-white rounded-xl flex items-center justify-center text-xl font-bold mx-auto mb-3">
              4
            </div>
            <h3 className="font-semibold text-gray-900 dark:text-gray-100 mb-2">Следите за матчами</h3>
            <p className="text-sm text-gray-600 dark:text-gray-400">
              Наблюдайте за результатами в реальном времени
            </p>
          </div>
        </div>
      </div>

      {/* CTA Section */}
      <div className="text-center py-8">
        <h2 className="text-2xl font-bold text-gray-900 dark:text-gray-100 mb-4">
          Готовы проверить свою стратегию?
        </h2>
        <p className="text-gray-600 dark:text-gray-400 mb-6 max-w-xl mx-auto">
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
