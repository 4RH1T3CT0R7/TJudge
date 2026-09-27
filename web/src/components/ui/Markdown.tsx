import { useEffect, useRef, useState, type ReactNode } from 'react';
import ReactMarkdown, { type Components } from 'react-markdown';
import remarkGfm from 'remark-gfm';

const remarkPlugins = [remarkGfm];

// Широкие таблицы и блоки кода прокручиваются внутри себя, а не растягивают
// страницу на телефоне. В порядок Tab (с ролью и именем) блок попадает, только
// когда ему есть что прокручивать: иначе это пустая остановка клавиатуры.
function ScrollX({ as: Tag, label, children }: { as: 'pre' | 'div'; label: string; children: ReactNode }) {
  const ref = useRef<HTMLPreElement & HTMLDivElement>(null);
  const [scrolls, setScrolls] = useState(false);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    const ro = new ResizeObserver(() => setScrolls(el.scrollWidth > el.clientWidth));
    ro.observe(el);
    return () => ro.disconnect();
  }, []);

  return (
    <Tag
      ref={ref}
      className="overflow-x-auto"
      tabIndex={scrolls ? 0 : undefined}
      role={scrolls ? 'region' : undefined}
      aria-label={scrolls ? label : undefined}
    >
      {children}
    </Tag>
  );
}

const components: Components = {
  pre: ({ children }) => <ScrollX as="pre" label="Код, прокрутка по горизонтали">{children}</ScrollX>,
  table: ({ children }) => (
    <ScrollX as="div" label="Таблица, прокрутка по горизонтали">
      <table>{children}</table>
    </ScrollX>
  ),
};

export function Markdown({ children }: { children: string }) {
  return (
    <ReactMarkdown remarkPlugins={remarkPlugins} components={components}>
      {children}
    </ReactMarkdown>
  );
}
