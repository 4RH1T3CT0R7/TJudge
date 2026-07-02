import { useState, useCallback } from 'react';
import { useEscapeKey } from '../../hooks/useEscapeKey';
import { GameInfoModal } from './GameInfoModal';
import { PrisonersDilemmaMatrix } from './PrisonersDilemmaMatrix';
import { TugOfWarVisualization } from './TugOfWarVisualization';
import { TravelersDilemmaVisualization } from './TravelersDilemmaVisualization';
import { PublicGoodsVisualization } from './PublicGoodsVisualization';
import { DollarAuctionVisualization } from './DollarAuctionVisualization';

// Game Showcase Component with tabs
export function GameShowcase() {
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
    {
      id: 'travelers_dilemma',
      name: 'Дилемма путешественника',
      icon: '🧳',
      color: 'blue',
      description: 'Два путешественника называют стоимость потерянных чемоданов. Жадность наказывается, а скромность вознаграждается.',
      rules: [
        { text: 'Одинаковые заявки', result: 'оба получают названную сумму', color: 'green' },
        { text: 'Разные заявки', result: 'оба получают минимум, но скромный получает бонус +R', color: 'blue' },
        { text: 'Жадный получает штраф', result: '-R от минимальной суммы', color: 'red' },
      ],
      insight: 'Равновесие Нэша: оба называют минимум (2) — но в турнирах стратегии с заявкой ~90 побеждают.',
      visualization: <TravelersDilemmaVisualization />,
    },
    {
      id: 'public_goods',
      name: 'Общественное благо',
      icon: '🏛️',
      color: 'orange',
      description: 'Каждый решает, сколько вложить в общий пул. Пул умножается и делится поровну — но зачем вкладывать, если можно получить бесплатно?',
      rules: [
        { text: 'Каждый начинает с 20 токенов', result: 'и решает, сколько вложить в пул', color: 'blue' },
        { text: 'Пул умножается на 1.5x', result: 'и делится поровну между игроками', color: 'green' },
        { text: 'Безбилетник выигрывает', result: 'но если оба так поступят — оба проиграют', color: 'red' },
      ],
      insight: 'Равновесие Нэша: не вкладывать ничего (каждый получает 20). Но если оба вложат все — каждый получит 30.',
      visualization: <PublicGoodsVisualization />,
    },
    {
      id: 'dollar_auction',
      name: 'Аукцион двойной цены',
      icon: '💰',
      color: 'yellow',
      description: 'Приз выставляется на торги, но проигравший тоже платит свою ставку. Классическая ловушка эскалации.',
      rules: [
        { text: 'Приз стоит 100 очков', result: 'ставки делаются поочередно', color: 'blue' },
        { text: 'Оба игрока платят свои ставки', result: 'но приз получает только победитель', color: 'red' },
        { text: 'Можно спасовать (0)', result: 'торги заканчиваются', color: 'green' },
      ],
      insight: 'Ловушка невозвратных затрат: «выгоднее» повысить ставку, чем потерять уже вложенное. Спираль может привести к ставкам больше приза!',
      visualization: <DollarAuctionVisualization />,
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
