import { Fragment, useEffect, useRef, useState, type ReactNode } from 'react';
import ReactMarkdown, { type Components } from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { ioTone, rehypeNumericColumns } from '../../utils/markdown';

const remarkPlugins = [remarkGfm];
const rehypePlugins = [rehypeNumericColumns];

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

// Строка блока кода: «← 100   # итерации» - направление протокола цветом,
// пояснение после « # » приглушено.
function CodeLine({ line }: { line: string }) {
  const at = line.search(/\s#\s/);
  const main = at < 0 ? line : line.slice(0, at);
  return (
    <>
      <span className={ioTone(line)}>{main}</span>
      {at >= 0 && <span className="text-gray-500">{line.slice(at)}</span>}
    </>
  );
}

const components: Components = {
  pre: ({ children }) => <ScrollX as="pre" label="Код, прокрутка по горизонтали">{children}</ScrollX>,
  table: ({ children }) => (
    <ScrollX as="div" label="Таблица, прокрутка по горизонтали">
      <table>{children}</table>
    </ScrollX>
  ),
  // у блока кода текст с переводами строк, у строчного - без
  code: ({ className, children }) => {
    if (typeof children !== 'string' || !children.includes('\n')) return <code className={className}>{children}</code>;
    const lines = children.replace(/\n$/, '').split('\n');
    return (
      <code className={className}>
        {lines.map((line, i) => (
          <Fragment key={i}>
            <CodeLine line={line} />
            {'\n'}
          </Fragment>
        ))}
      </code>
    );
  },
};

// Markdown правил игр и описаний турниров в стиле man-страницы (стили .md в index.css).
export function Markdown({ children }: { children: string }) {
  return (
    <div className="md">
      <ReactMarkdown remarkPlugins={remarkPlugins} rehypePlugins={rehypePlugins} components={components}>
        {children}
      </ReactMarkdown>
    </div>
  );
}
