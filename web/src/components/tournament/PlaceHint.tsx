import type { ReactNode } from 'react';
import { Link } from 'react-router-dom';

// Пояснение под таблицей: место решает сумма очков, подробности в справке.
export function PlaceHint({ children }: { children: ReactNode }) {
  return (
    <p className="mt-3 font-mono text-xs text-gray-500">
      <span aria-hidden="true">{'// '}</span>
      {children} ·{' '}
      <Link to="/help#place" className="text-primary-400 underline hover:text-primary-300">
        как считается место
      </Link>
    </p>
  );
}
