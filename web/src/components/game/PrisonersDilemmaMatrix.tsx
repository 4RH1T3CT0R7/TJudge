import { useState } from 'react';

// Prisoner's Dilemma Matrix Component
export function PrisonersDilemmaMatrix() {
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
