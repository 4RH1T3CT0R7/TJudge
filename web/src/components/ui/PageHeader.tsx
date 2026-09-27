import { Link } from 'react-router-dom';
import type { ReactNode } from 'react';

export interface Crumb {
  label: string;
  /** Ссылка; у последней крошки (текущая страница) не нужна. */
  to?: string;
}

interface PageHeaderProps {
  /** Путь от «~», последняя крошка — текущая страница. */
  crumbs?: Crumb[];
  title: ReactNode;
  /** Бейдж статуса рядом с заголовком. */
  status?: ReactNode;
  /** Действия справа, на узком экране под заголовком. */
  actions?: ReactNode;
  /** Строка под заголовком (код турнира и т. п.). */
  children?: ReactNode;
}

// путь в стиле pwd: «Осенний кубок» -> «осенний-кубок»
const asPath = (label: string) => label.trim().toLowerCase().replace(/\s+/g, '-');

// Шапка страницы: крошка-pwd «~/турниры/осенний-кубок», один размер H1, статус и действия.
export function PageHeader({ crumbs, title, status, actions, children }: PageHeaderProps) {
  return (
    <header className="mb-8">
      {/* py-0.5 у ссылок: при переносе крошки на телефоне цель касания не ниже 24 px */}
      {crumbs && crumbs.length > 0 && (
        <nav aria-label="Путь" className="mb-3 font-mono text-sm text-gray-500">
          <ol className="flex flex-wrap items-center">
            <li>
              <Link to="/" className="inline-block py-0.5 hover:text-primary-400 transition-colors">~</Link>
            </li>
            {crumbs.map((crumb, i) => {
              const current = i === crumbs.length - 1;
              return (
                <li key={i} className="flex min-w-0 items-center">
                  <span aria-hidden="true" className="px-0.5">/</span>
                  {crumb.to && !current ? (
                    <Link to={crumb.to} className="inline-block py-0.5 hover:text-primary-400 transition-colors">{asPath(crumb.label)}</Link>
                  ) : (
                    <span aria-current={current ? 'page' : undefined} className="truncate text-gray-400">
                      {asPath(crumb.label)}
                    </span>
                  )}
                </li>
              );
            })}
          </ol>
        </nav>
      )}
      <div className="flex flex-col lg:flex-row lg:items-start lg:justify-between gap-4">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-3">
            <h1 className="text-2xl sm:text-3xl font-bold text-gray-100 break-words">{title}</h1>
            {status}
          </div>
          {children && <div className="mt-3">{children}</div>}
        </div>
        {actions && <div className="flex flex-wrap gap-3">{actions}</div>}
      </div>
    </header>
  );
}
