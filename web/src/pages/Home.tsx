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

// Game history and detailed info
const gameDetails: Record<string, { history: string; facts: string[]; applications: string[] }> = {
  prisoners_dilemma: {
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
  },
  good_deal: {
    history: `«Выгодная сделка» основана на модели двусторонних переговоров, которую формализовал Джон Нэш в своей работе «Проблема торга» (The Bargaining Problem) в 1950 году. За эту и другие работы он получил Нобелевскую премию по экономике в 1994 году.

Суть проста: у продавца есть товар, который он оценивает в X рублей (ниже не продаст). У покупателя есть максимум Y рублей, который он готов заплатить. Если Y ≥ X — сделка возможна, и оба выиграют. Вопрос только в том, как разделить «выигрыш» (разницу Y − X).

Нэш доказал, что существует единственное «справедливое» решение — делить пополам. Но в реальности всё зависит от переговорной силы сторон: кто может дольше ждать, у кого есть альтернативы, кто лучше блефует.

Эта простая модель объясняет всё: от торговли на базаре до международных торговых соглашений.`,
    facts: [
      'Джон Нэш страдал шизофренией, но продолжал делать выдающиеся открытия — его история показана в фильме «Игры разума»',
      'Теорема Нэша о торге требует всего 4 аксиомы: эффективность, симметрия, независимость от масштаба и независимость от нерелевантных альтернатив',
      'На аукционах eBay средняя цена обычно оказывается ровно посередине между ценой продавца и максимальной ставкой покупателя',
      'Исследования показывают, что первый названный в переговорах price служит «якорем» и сильно влияет на итоговую цену'
    ],
    applications: [
      'Переговоры о зарплате при найме на работу',
      'Сделки купли-продажи недвижимости',
      'Международная торговля и таможенные тарифы',
      'Слияния и поглощения компаний'
    ]
  },
  balance_of_universe: {
    history: `«Баланс вселенной» — это наша интерпретация игр координации и общественных благ, которые изучаются в теории игр с 1960-х годов.

Классический пример — «Трагедия общин», описанная экологом Гарретом Хардином в 1968 году. Представьте общее пастбище: каждому фермеру выгодно добавить ещё одну корову, но если все так поступят — пастбище погибнет.

Похожая логика работает в играх координации: игрокам нужно договориться о каком-то балансе, даже если каждому по отдельности выгодно «перетянуть одеяло» на себя.

В нашей игре «порядок» и «хаос» — это метафора любых противоположных интересов. Максимальный выигрыш достигается в равновесии, но соблазн «доминировать» очень велик. Это модель экологии, международных отношений и даже семейной жизни.`,
    facts: [
      'Элинор Остром получила Нобелевскую премию 2009 года за исследование того, как сообщества решают проблему общих ресурсов без государства',
      'Концепция «баланса сил» в международных отношениях восходит к древнегреческому историку Фукидиду',
      'В теории хаоса (математике) даже детерминированные системы могут вести себя непредсказуемо — эффект бабочки',
      'Игры координации объясняют, почему все ездят по одной стороне дороги — важен не выбор стороны, а согласованность'
    ],
    applications: [
      'Экология: управление общими ресурсами (леса, рыба, вода)',
      'Климатические соглашения между странами',
      'Стандартизация в технологиях (USB, Wi-Fi, форматы файлов)',
      'Социальные нормы и общественный договор'
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
        className="relative bg-white dark:bg-gray-900 rounded-2xl shadow-2xl max-w-2xl w-full max-h-[85vh] overflow-hidden animate-scale-in"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="sticky top-0 bg-white dark:bg-gray-900 border-b dark:border-gray-800 px-6 py-4 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <span className="text-3xl">{gameIcon}</span>
            <h2 className="text-xl font-bold text-gray-900 dark:text-gray-100">{gameName}</h2>
          </div>
          <button
            onClick={onClose}
            className="w-8 h-8 rounded-full bg-gray-100 dark:bg-gray-800 flex items-center justify-center hover:bg-gray-200 dark:hover:bg-gray-700 transition-colors"
          >
            <svg className="w-5 h-5 text-gray-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        {/* Content */}
        <div className="px-6 py-5 overflow-y-auto max-h-[calc(85vh-80px)] space-y-6">
          {/* History */}
          <div>
            <h3 className="text-sm font-bold text-primary-600 dark:text-primary-400 uppercase tracking-wide mb-3">
              История
            </h3>
            <div className="text-gray-700 dark:text-gray-300 text-sm leading-relaxed whitespace-pre-line">
              {details.history}
            </div>
          </div>

          {/* Interesting facts */}
          <div>
            <h3 className="text-sm font-bold text-amber-600 dark:text-amber-400 uppercase tracking-wide mb-3">
              Интересные факты
            </h3>
            <ul className="space-y-2">
              {details.facts.map((fact, i) => (
                <li key={i} className="flex gap-2 text-sm text-gray-700 dark:text-gray-300">
                  <span className="text-amber-500 mt-1">•</span>
                  <span>{fact}</span>
                </li>
              ))}
            </ul>
          </div>

          {/* Applications */}
          <div>
            <h3 className="text-sm font-bold text-blue-600 dark:text-blue-400 uppercase tracking-wide mb-3">
              Где применяется
            </h3>
            <div className="flex flex-wrap gap-2">
              {details.applications.map((app, i) => (
                <span key={i} className="px-3 py-1 bg-blue-50 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300 text-xs rounded-full">
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

// Prisoner's Dilemma Matrix Component - Clean and centered design
function PrisonersDilemmaMatrix() {
  const [hoveredCell, setHoveredCell] = useState<string | null>(null);

  const cellInfo: Record<string, { title: string; desc: string }> = {
    'cc': { title: 'Взаимное сотрудничество', desc: 'Оба выигрывают!' },
    'cd': { title: 'A сотрудничает, B предаёт', desc: 'B получает максимум' },
    'dc': { title: 'A предаёт, B сотрудничает', desc: 'A получает максимум' },
    'dd': { title: 'Равновесие Нэша', desc: 'Оба проигрывают' },
  };

  return (
    <div className="flex flex-col items-center justify-center w-full">
      {/* Main table container */}
      <table className="border-collapse">
        {/* Header row with Player B */}
        <thead>
          <tr>
            <th colSpan={2}></th>
            <th colSpan={2} className="text-center pb-2 text-base font-bold text-gray-800 dark:text-gray-200">
              Игрок B
            </th>
          </tr>
          <tr>
            <th colSpan={2}></th>
            <th className="w-24 text-center pb-1 text-sm font-semibold text-emerald-600 dark:text-emerald-400">Сотр.</th>
            <th className="w-24 text-center pb-1 text-sm font-semibold text-rose-600 dark:text-rose-400">Пред.</th>
          </tr>
        </thead>
        <tbody>
          {/* Row 1: Cooperate */}
          <tr>
            <td rowSpan={2} className="pr-2 align-middle">
              <div className="text-base font-bold text-gray-800 dark:text-gray-200 -rotate-90 whitespace-nowrap">
                Игрок A
              </div>
            </td>
            <td className="text-right pr-2 text-sm font-semibold text-emerald-600 dark:text-emerald-400 align-middle">Сотр.</td>
            <td
              className={`w-24 h-16 text-center cursor-pointer transition-all rounded-tl-lg ${hoveredCell === 'cc' ? 'scale-105 shadow-xl z-10' : 'hover:brightness-110'}`}
              style={{ backgroundColor: '#059669' }}
              onMouseEnter={() => setHoveredCell('cc')}
              onMouseLeave={() => setHoveredCell(null)}
            >
              <span className="font-mono font-bold text-lg text-white">3, 3</span>
            </td>
            <td
              className={`w-24 h-16 text-center cursor-pointer transition-all rounded-tr-lg ${hoveredCell === 'cd' ? 'scale-105 shadow-xl z-10' : 'hover:brightness-110'}`}
              style={{ backgroundColor: '#dc2626' }}
              onMouseEnter={() => setHoveredCell('cd')}
              onMouseLeave={() => setHoveredCell(null)}
            >
              <span className="font-mono font-bold text-lg text-white">0, 5</span>
            </td>
          </tr>
          {/* Row 2: Defect */}
          <tr>
            <td className="text-right pr-2 text-sm font-semibold text-rose-600 dark:text-rose-400 align-middle">Пред.</td>
            <td
              className={`w-24 h-16 text-center cursor-pointer transition-all rounded-bl-lg ${hoveredCell === 'dc' ? 'scale-105 shadow-xl z-10' : 'hover:brightness-110'}`}
              style={{ backgroundColor: '#dc2626' }}
              onMouseEnter={() => setHoveredCell('dc')}
              onMouseLeave={() => setHoveredCell(null)}
            >
              <span className="font-mono font-bold text-lg text-white">5, 0</span>
            </td>
            <td
              className={`w-24 h-16 text-center cursor-pointer transition-all rounded-br-lg relative ${hoveredCell === 'dd' ? 'scale-105 shadow-xl z-10' : 'hover:brightness-110'}`}
              style={{ backgroundColor: '#ca8a04' }}
              onMouseEnter={() => setHoveredCell('dd')}
              onMouseLeave={() => setHoveredCell(null)}
            >
              <span className="font-mono font-bold text-lg text-white">1, 1</span>
              <div className="absolute top-1 right-1 w-2.5 h-2.5 bg-cyan-400 rounded-full" title="Равновесие Нэша" />
            </td>
          </tr>
        </tbody>
      </table>

      {/* Tooltip */}
      <div className={`mt-4 text-center transition-all duration-200 h-10 ${hoveredCell ? 'opacity-100' : 'opacity-50'}`}>
        {hoveredCell ? (
          <>
            <div className="text-sm font-semibold text-gray-800 dark:text-gray-200">{cellInfo[hoveredCell].title}</div>
            <div className="text-xs text-gray-500 dark:text-gray-400">{cellInfo[hoveredCell].desc}</div>
          </>
        ) : (
          <div className="text-xs text-gray-400 dark:text-gray-500">Наведите на ячейку для подробностей</div>
        )}
      </div>
    </div>
  );
}

// Tug of War Visualization - With visual rope
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

  // Calculate rope position based on current round result
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
        {/* Background field */}
        <div className="absolute inset-0 flex">
          <div className={`flex-1 rounded-l-xl transition-colors ${ropePosition < 45 ? 'bg-blue-100 dark:bg-blue-900/30' : 'bg-gray-100 dark:bg-gray-800'}`} />
          <div className={`flex-1 rounded-r-xl transition-colors ${ropePosition > 55 ? 'bg-red-100 dark:bg-red-900/30' : 'bg-gray-100 dark:bg-gray-800'}`} />
        </div>

        {/* Center line */}
        <div className="absolute left-1/2 top-0 bottom-0 w-0.5 bg-gray-300 dark:bg-gray-600 -translate-x-1/2" />

        {/* Rope */}
        <svg className="absolute inset-0 w-full h-full" viewBox="0 0 300 60" preserveAspectRatio="none">
          {/* Rope path */}
          <path
            d={`M 10,30 Q 75,${25 + Math.sin(Date.now() / 500) * 3} 150,30 Q 225,${35 + Math.sin(Date.now() / 500) * 3} 290,30`}
            fill="none"
            stroke="#b45309"
            strokeWidth="6"
            strokeLinecap="round"
          />
          <path
            d={`M 10,30 Q 75,${25 + Math.sin(Date.now() / 500) * 3} 150,30 Q 225,${35 + Math.sin(Date.now() / 500) * 3} 290,30`}
            fill="none"
            stroke="#d97706"
            strokeWidth="4"
            strokeLinecap="round"
          />
          {/* Knot */}
          <circle
            cx={ropePosition * 3}
            cy="30"
            r="10"
            fill="#dc2626"
            className="transition-all duration-500"
          />
          <circle
            cx={ropePosition * 3}
            cy="30"
            r="6"
            fill="#fca5a5"
          />
        </svg>

        {/* Players */}
        <div className="absolute left-2 top-1/2 -translate-y-1/2 w-10 h-10 rounded-full bg-blue-500 flex items-center justify-center text-white font-bold text-sm shadow-lg">
          A
        </div>
        <div className="absolute right-2 top-1/2 -translate-y-1/2 w-10 h-10 rounded-full bg-red-500 flex items-center justify-center text-white font-bold text-sm shadow-lg">
          B
        </div>
      </div>

      {/* Round selector when showing results */}
      {showResults && (
        <div className="flex justify-center gap-2">
          {rounds.map((_, i) => (
            <button
              key={i}
              onClick={() => setCurrentRound(i)}
              className={`px-3 py-1 rounded-lg text-xs font-medium transition-all ${
                currentRound === i
                  ? 'bg-primary-500 text-white'
                  : 'bg-gray-200 dark:bg-gray-700 text-gray-600 dark:text-gray-300'
              }`}
            >
              Раунд {i + 1}
            </button>
          ))}
        </div>
      )}

      {/* Force allocation */}
      <div className="space-y-2">
        {rounds.map((force, index) => (
          <div key={index} className="flex items-center gap-2 text-xs">
            <span className="w-14 text-gray-500 dark:text-gray-400">Раунд {index + 1}</span>
            <button onClick={() => adjustRound(index, -5)} disabled={force <= 0 || showResults} className="w-6 h-6 rounded bg-blue-100 dark:bg-blue-900/50 text-blue-600 disabled:opacity-30">−</button>
            <div className="w-8 text-center font-bold text-blue-600 dark:text-blue-400">{force}</div>
            <button onClick={() => adjustRound(index, 5)} disabled={remaining <= 0 || showResults} className="w-6 h-6 rounded bg-blue-100 dark:bg-blue-900/50 text-blue-600 disabled:opacity-30">+</button>
            <div className="flex-1 h-2 bg-gray-200 dark:bg-gray-700 rounded-full overflow-hidden">
              <div className="h-full bg-blue-500 transition-all" style={{ width: `${force}%` }} />
            </div>
            {showResults && (
              <span className={`w-8 text-center font-bold ${
                force > opponentRounds[index] ? 'text-green-600' : force < opponentRounds[index] ? 'text-red-600' : 'text-gray-500'
              }`}>
                {force > opponentRounds[index] ? '✓' : force < opponentRounds[index] ? '✗' : '–'}
              </span>
            )}
          </div>
        ))}
      </div>

      {/* Remaining indicator */}
      {!showResults && remaining > 0 && (
        <div className="text-center text-xs text-gray-500 dark:text-gray-400">
          Осталось распределить: <span className="font-bold text-blue-600">{remaining}</span>
        </div>
      )}

      {/* Battle button / Results */}
      <div className="text-center">
        {!showResults ? (
          <button
            onClick={() => { setShowResults(true); setCurrentRound(0); }}
            disabled={remaining > 0}
            className="px-5 py-2 rounded-xl bg-gradient-to-r from-amber-500 to-orange-500 text-white text-sm font-bold shadow-lg hover:scale-105 transition-all disabled:opacity-50 disabled:hover:scale-100"
          >
            {remaining > 0 ? `Ещё ${remaining}` : '⚔️ Тянуть!'}
          </button>
        ) : (
          <div className="space-y-1">
            <div className={`text-sm font-bold ${
              results.winner === 'A' ? 'text-blue-600' : results.winner === 'B' ? 'text-red-600' : 'text-gray-600'
            }`}>
              {results.winner === 'A' ? '🎉 Победа!' : results.winner === 'B' ? '😔 Поражение' : '🤝 Ничья'}
              <span className="text-gray-400 font-normal ml-2">({results.playerWins}:{results.opponentWins})</span>
            </div>
            <button onClick={() => { setShowResults(false); setRounds([35, 35, 30]); }} className="text-xs text-gray-400 hover:text-gray-600 underline">
              Заново
            </button>
          </div>
        )}
      </div>
    </div>
  );
}

// Good Deal Visualization - Compact design with +/- buttons instead of sliders
function GoodDealVisualization() {
  const [sellerMin, setSellerMin] = useState(30);
  const [buyerMax, setBuyerMax] = useState(70);

  const dealPossible = buyerMax >= sellerMin;
  const dealPrice = dealPossible ? Math.round((sellerMin + buyerMax) / 2) : null;

  const adjustSeller = (delta: number) => {
    const newValue = Math.max(0, Math.min(100, sellerMin + delta));
    setSellerMin(newValue);
  };

  const adjustBuyer = (delta: number) => {
    const newValue = Math.max(0, Math.min(100, buyerMax + delta));
    setBuyerMax(newValue);
  };

  return (
    <div className="flex flex-col justify-center space-y-4">
      {/* Visual price scale */}
      <div className="relative pt-7 pb-2">
        {/* Scale bar */}
        <div className="h-14 bg-gray-200 dark:bg-gray-700 rounded-xl relative overflow-hidden">
          {/* Deal zone highlight */}
          {dealPossible && (
            <div
              className="absolute top-0 bottom-0 bg-gradient-to-r from-green-400/60 to-green-500/60 dark:from-green-500/40 dark:to-green-600/40"
              style={{ left: `${sellerMin}%`, right: `${100 - buyerMax}%` }}
            />
          )}

          {/* Scale markers */}
          <div className="absolute bottom-1 left-3 text-xs text-gray-400 font-mono">0</div>
          <div className="absolute bottom-1 left-1/4 text-xs text-gray-400 font-mono">25</div>
          <div className="absolute bottom-1 left-1/2 -translate-x-1/2 text-xs text-gray-400 font-mono">50</div>
          <div className="absolute bottom-1 left-3/4 text-xs text-gray-400 font-mono">75</div>
          <div className="absolute bottom-1 right-3 text-xs text-gray-400 font-mono">100</div>

          {/* Seller marker */}
          <div
            className="absolute top-0 bottom-0 w-1.5 bg-blue-500 shadow-lg shadow-blue-500/50"
            style={{ left: `${sellerMin}%` }}
          >
            <div className="absolute -top-7 left-1/2 -translate-x-1/2 px-2 py-0.5 bg-blue-500 rounded text-xs font-bold text-white whitespace-nowrap">
              A: {sellerMin}
            </div>
          </div>

          {/* Buyer marker */}
          <div
            className="absolute top-0 bottom-0 w-1.5 bg-red-500 shadow-lg shadow-red-500/50"
            style={{ left: `${buyerMax}%` }}
          >
            <div className="absolute -top-7 left-1/2 -translate-x-1/2 px-2 py-0.5 bg-red-500 rounded text-xs font-bold text-white whitespace-nowrap">
              B: {buyerMax}
            </div>
          </div>

          {/* Deal price marker */}
          {dealPrice && (
            <div
              className="absolute top-1/2 -translate-y-1/2 w-10 h-10 bg-green-500 rounded-full flex items-center justify-center shadow-xl shadow-green-500/40 border-2 border-white"
              style={{ left: `calc(${dealPrice}% - 20px)` }}
            >
              <span className="text-white text-sm font-bold">{dealPrice}</span>
            </div>
          )}
        </div>
      </div>

      {/* Controls - using +/- buttons instead of sliders */}
      <div className="grid grid-cols-2 gap-4">
        {/* Seller control */}
        <div className="bg-blue-50 dark:bg-blue-900/20 rounded-xl p-3">
          <div className="flex items-center justify-between mb-3">
            <div className="flex items-center gap-2">
              <div className="w-8 h-8 rounded-lg bg-blue-500 text-white flex items-center justify-center font-bold text-sm">A</div>
              <div className="text-xs text-gray-500 dark:text-gray-400">Продавец</div>
            </div>
            <div className="text-lg font-bold text-blue-600 dark:text-blue-400">{sellerMin}</div>
          </div>
          <div className="flex items-center justify-center gap-2">
            <button
              onClick={() => adjustSeller(-10)}
              className="w-10 h-10 rounded-lg bg-blue-100 dark:bg-blue-800 text-blue-600 dark:text-blue-400 font-bold hover:bg-blue-200 dark:hover:bg-blue-700 transition-colors"
            >
              -10
            </button>
            <button
              onClick={() => adjustSeller(-1)}
              className="w-10 h-10 rounded-lg bg-blue-200 dark:bg-blue-700 text-blue-700 dark:text-blue-300 font-bold hover:bg-blue-300 dark:hover:bg-blue-600 transition-colors"
            >
              -1
            </button>
            <button
              onClick={() => adjustSeller(1)}
              className="w-10 h-10 rounded-lg bg-blue-200 dark:bg-blue-700 text-blue-700 dark:text-blue-300 font-bold hover:bg-blue-300 dark:hover:bg-blue-600 transition-colors"
            >
              +1
            </button>
            <button
              onClick={() => adjustSeller(10)}
              className="w-10 h-10 rounded-lg bg-blue-100 dark:bg-blue-800 text-blue-600 dark:text-blue-400 font-bold hover:bg-blue-200 dark:hover:bg-blue-700 transition-colors"
            >
              +10
            </button>
          </div>
        </div>

        {/* Buyer control */}
        <div className="bg-red-50 dark:bg-red-900/20 rounded-xl p-3">
          <div className="flex items-center justify-between mb-3">
            <div className="flex items-center gap-2">
              <div className="w-8 h-8 rounded-lg bg-red-500 text-white flex items-center justify-center font-bold text-sm">B</div>
              <div className="text-xs text-gray-500 dark:text-gray-400">Покупатель</div>
            </div>
            <div className="text-lg font-bold text-red-600 dark:text-red-400">{buyerMax}</div>
          </div>
          <div className="flex items-center justify-center gap-2">
            <button
              onClick={() => adjustBuyer(-10)}
              className="w-10 h-10 rounded-lg bg-red-100 dark:bg-red-800 text-red-600 dark:text-red-400 font-bold hover:bg-red-200 dark:hover:bg-red-700 transition-colors"
            >
              -10
            </button>
            <button
              onClick={() => adjustBuyer(-1)}
              className="w-10 h-10 rounded-lg bg-red-200 dark:bg-red-700 text-red-700 dark:text-red-300 font-bold hover:bg-red-300 dark:hover:bg-red-600 transition-colors"
            >
              -1
            </button>
            <button
              onClick={() => adjustBuyer(1)}
              className="w-10 h-10 rounded-lg bg-red-200 dark:bg-red-700 text-red-700 dark:text-red-300 font-bold hover:bg-red-300 dark:hover:bg-red-600 transition-colors"
            >
              +1
            </button>
            <button
              onClick={() => adjustBuyer(10)}
              className="w-10 h-10 rounded-lg bg-red-100 dark:bg-red-800 text-red-600 dark:text-red-400 font-bold hover:bg-red-200 dark:hover:bg-red-700 transition-colors"
            >
              +10
            </button>
          </div>
        </div>
      </div>

      {/* Result */}
      <div className={`text-center py-3 px-4 rounded-xl ${
        dealPossible
          ? 'bg-green-100 dark:bg-green-900/30'
          : 'bg-gray-100 dark:bg-gray-800'
      }`}>
        {dealPossible ? (
          <div className="flex items-center justify-center gap-3">
            <span className="text-2xl">🤝</span>
            <div>
              <span className="text-base font-bold text-green-700 dark:text-green-300">
                Сделка по цене {dealPrice}!
              </span>
              <div className="text-xs text-green-600 dark:text-green-400">
                Выигрыш: A +{dealPrice! - sellerMin}, B +{buyerMax - dealPrice!}
              </div>
            </div>
          </div>
        ) : (
          <div className="flex items-center justify-center gap-2 text-gray-500 dark:text-gray-400">
            <span className="text-xl">❌</span>
            <span className="text-sm">Нет сделки — цены не пересекаются</span>
          </div>
        )}
      </div>
    </div>
  );
}

// Balance of Universe Visualization - Interactive with clickable weights
function BalanceVisualization() {
  const [leftWeight, setLeftWeight] = useState(3);
  const [rightWeight, setRightWeight] = useState(3);

  const diff = leftWeight - rightWeight;
  const tilt = diff * 5; // -10 to +10 degrees
  const isBalanced = diff === 0;
  const leftWins = diff > 0;
  const rightWins = diff < 0;

  const addWeight = (side: 'left' | 'right') => {
    if (side === 'left' && leftWeight < 5) setLeftWeight(leftWeight + 1);
    if (side === 'right' && rightWeight < 5) setRightWeight(rightWeight + 1);
  };

  const removeWeight = (side: 'left' | 'right') => {
    if (side === 'left' && leftWeight > 1) setLeftWeight(leftWeight - 1);
    if (side === 'right' && rightWeight > 1) setRightWeight(rightWeight - 1);
  };

  return (
    <div className="flex flex-col justify-center space-y-4">
      {/* Balance scale */}
      <div className="relative flex justify-center py-2">
        <svg width="260" height="130" viewBox="0 0 260 130" className="overflow-visible">
          {/* Stand */}
          <rect x="125" y="60" width="10" height="55" fill="#9ca3af" rx="2" />
          <rect x="100" y="112" width="60" height="10" fill="#6b7280" rx="5" />

          {/* Balance beam with tilt */}
          <g style={{ transform: `rotate(${tilt}deg)`, transformOrigin: '130px 55px', transition: 'transform 0.3s ease-out' }}>
            {/* Beam */}
            <rect x="20" y="51" width="220" height="8" fill="#d1d5db" rx="4" />

            {/* Pivot point */}
            <circle cx="130" cy="55" r="10" fill="#6b7280" />
            <circle cx="130" cy="55" r="5" fill="#9ca3af" />

            {/* Left pan (Order - Blue) */}
            <g>
              <line x1="45" y1="59" x2="45" y2="85" stroke="#9ca3af" strokeWidth="2" />
              <ellipse cx="45" cy="90" rx="35" ry="8" fill="#fbbf24" />
              <ellipse cx="45" cy="88" rx="32" ry="6" fill="#f59e0b" />
              {/* Weights */}
              {Array.from({ length: leftWeight }).map((_, i) => (
                <circle
                  key={i}
                  cx={30 + (i % 3) * 15}
                  cy={75 - Math.floor(i / 3) * 12}
                  r={7}
                  fill={`hsl(220, ${70 + i * 5}%, ${50 + i * 5}%)`}
                  className="drop-shadow-md"
                />
              ))}
            </g>

            {/* Right pan (Chaos - Red) */}
            <g>
              <line x1="215" y1="59" x2="215" y2="85" stroke="#9ca3af" strokeWidth="2" />
              <ellipse cx="215" cy="90" rx="35" ry="8" fill="#fbbf24" />
              <ellipse cx="215" cy="88" rx="32" ry="6" fill="#f59e0b" />
              {/* Weights */}
              {Array.from({ length: rightWeight }).map((_, i) => (
                <circle
                  key={i}
                  cx={200 + (i % 3) * 15}
                  cy={75 - Math.floor(i / 3) * 12}
                  r={7}
                  fill={`hsl(0, ${70 + i * 5}%, ${50 + i * 5}%)`}
                  className="drop-shadow-md"
                />
              ))}
            </g>
          </g>
        </svg>
      </div>

      {/* Controls */}
      <div className="flex justify-between px-2">
        {/* Left side controls */}
        <div className="flex flex-col items-center gap-2">
          <span className={`text-xs font-semibold ${leftWins ? 'text-blue-600 dark:text-blue-400' : 'text-gray-500 dark:text-gray-400'}`}>
            Порядок ({leftWeight})
          </span>
          <div className="flex gap-1">
            <button
              onClick={() => removeWeight('left')}
              className="w-8 h-8 rounded-full bg-blue-100 dark:bg-blue-900/50 text-blue-600 dark:text-blue-400 font-bold hover:bg-blue-200 dark:hover:bg-blue-800/50 transition-colors disabled:opacity-30"
              disabled={leftWeight <= 1}
            >
              −
            </button>
            <button
              onClick={() => addWeight('left')}
              className="w-8 h-8 rounded-full bg-blue-500 text-white font-bold hover:bg-blue-600 transition-colors disabled:opacity-30"
              disabled={leftWeight >= 5}
            >
              +
            </button>
          </div>
        </div>

        {/* Center status */}
        <div className="flex flex-col items-center">
          <div className={`text-2xl transition-transform ${isBalanced ? 'scale-125' : ''}`}>
            {isBalanced ? '✨' : leftWins ? '📐' : '🌀'}
          </div>
          <span className={`text-xs font-bold ${
            isBalanced ? 'text-green-600 dark:text-green-400' :
            leftWins ? 'text-blue-600 dark:text-blue-400' : 'text-red-600 dark:text-red-400'
          }`}>
            {isBalanced ? 'Баланс!' : leftWins ? 'Порядок' : 'Хаос'}
          </span>
        </div>

        {/* Right side controls */}
        <div className="flex flex-col items-center gap-2">
          <span className={`text-xs font-semibold ${rightWins ? 'text-red-600 dark:text-red-400' : 'text-gray-500 dark:text-gray-400'}`}>
            Хаос ({rightWeight})
          </span>
          <div className="flex gap-1">
            <button
              onClick={() => removeWeight('right')}
              className="w-8 h-8 rounded-full bg-red-100 dark:bg-red-900/50 text-red-600 dark:text-red-400 font-bold hover:bg-red-200 dark:hover:bg-red-800/50 transition-colors disabled:opacity-30"
              disabled={rightWeight <= 1}
            >
              −
            </button>
            <button
              onClick={() => addWeight('right')}
              className="w-8 h-8 rounded-full bg-red-500 text-white font-bold hover:bg-red-600 transition-colors disabled:opacity-30"
              disabled={rightWeight >= 5}
            >
              +
            </button>
          </div>
        </div>
      </div>

      {/* Instruction */}
      <div className="text-center">
        <div className="inline-flex items-center gap-2 px-3 py-1.5 rounded-full text-xs font-medium bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300">
          <span className="text-lg">⚖️</span>
          Добавляй и убирай грузы для баланса
        </div>
      </div>
    </div>
  );
}

// Game Showcase Component with tabs
function GameShowcase() {
  const [activeGame, setActiveGame] = useState(0);
  const [modalOpen, setModalOpen] = useState(false);

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
      <div className="flex flex-wrap justify-center gap-3 md:gap-4">
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

          {/* Learn more button */}
          <button
            onClick={() => setModalOpen(true)}
            className="inline-flex items-center gap-2 text-sm font-medium text-gray-600 dark:text-gray-400 hover:text-primary-600 dark:hover:text-primary-400 transition-colors group"
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
        <div className="flex justify-center items-center bg-white dark:bg-gray-800/80 rounded-2xl p-4 border border-gray-200 dark:border-gray-700 transition-all">
          <div className="w-full animate-fade-in" key={currentGame.id + '-viz'}>
            {currentGame.visualization}
          </div>
        </div>
      </div>

      {/* Game info modal */}
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
