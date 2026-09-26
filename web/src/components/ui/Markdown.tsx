import ReactMarkdown, { type Components } from 'react-markdown';
import remarkGfm from 'remark-gfm';

const remarkPlugins = [remarkGfm];

// широкие таблицы и блоки кода прокручиваются внутри себя, а не растягивают
// страницу на телефоне; tabIndex - чтобы прокрутка была доступна с клавиатуры
const components: Components = {
  pre: ({ children }) => <pre tabIndex={0} className="overflow-x-auto">{children}</pre>,
  table: ({ children }) => (
    <div tabIndex={0} className="overflow-x-auto">
      <table>{children}</table>
    </div>
  ),
};

export function Markdown({ children }: { children: string }) {
  return (
    <ReactMarkdown remarkPlugins={remarkPlugins} components={components}>
      {children}
    </ReactMarkdown>
  );
}
