// Разбор Markdown правил игр и описаний турниров: превью для карточек,
// числовые колонки таблиц, окраска строк протокола.

/** Цвет строки протокола: ← от судьи (stdin), → от программы (stdout). */
export function ioTone(text: string): string | undefined {
  const t = text.trimStart();
  if (t.startsWith('←')) return 'text-cyan-300';
  if (t.startsWith('→')) return 'text-primary-300';
  return undefined;
}

/** Превью для карточки: первый абзац текста без разметки. */
export function mdPreview(md: string): string {
  const para = md
    .split(/\n\s*\n/)
    .map((block) => block.trim())
    .find((block) => block && !/^(#|\||```|~~~|---|\*\*\*|>|[-*+] |\d+\. )/.test(block));
  return (para ?? '')
    .replace(/!?\[([^\]]*)\]\([^)]*\)/g, '$1')
    .replace(/`([^`]+)`/g, '$1')
    .replace(/(\*\*|~~|\*)(.+?)\1/g, '$2')
    // подчёркивание внутри слова (score_multiplier) - не выделение
    .replace(/(^|[^\p{L}\p{N}])(__|_)(.+?)\2(?![\p{L}\p{N}])/gu, '$1$3')
    .replace(/\s+/g, ' ')
    .trim();
}

/** Есть ли в тексте разметка Markdown (заголовки, списки, таблицы, выделение, ссылки, код). */
export function hasMarkup(md: string): boolean {
  return /(^|\n) {0,3}(#{1,6} |[-*+] |\d+[.)] |>|```|~~~|\|)|[*_`[\]]|https?:\/\//.test(md);
}

// Узел hast: только то, что нужно плагину.
interface HastNode {
  type: string;
  tagName?: string;
  value?: string;
  properties?: Record<string, unknown>;
  children?: HastNode[];
}

const text = (node: HastNode): string =>
  node.type === 'text' ? node.value ?? '' : (node.children ?? []).map(text).join('');

const cells = (row: HastNode) => (row.children ?? []).filter((c) => c.tagName === 'td' || c.tagName === 'th');

const rows = (node: HastNode): HastNode[] =>
  node.tagName === 'tr' ? [node] : (node.children ?? []).flatMap(rows);

// число со знаком и дробью: «5», «-60», «1.5»
const NUMBER = /^[-+−]?\d+([.,]\d+)?$/;

/** rehype-плагин: колонка таблицы, где в теле только числа, получает класс num (вправо, моноширинно). */
export function rehypeNumericColumns() {
  const visit = (node: HastNode) => {
    if (node.tagName === 'table') {
      const all = rows(node);
      const body = all.filter((r) => cells(r).every((c) => c.tagName === 'td'));
      const width = Math.max(0, ...all.map((r) => cells(r).length));
      for (let col = 0; col < width; col++) {
        const values = body.map((r) => text(cells(r)[col] ?? { type: 'text' }).trim());
        if (values.length === 0 || !values.every((v) => NUMBER.test(v))) continue;
        for (const r of all) {
          const cell = cells(r)[col];
          if (cell) cell.properties = { ...cell.properties, className: ['num'] };
        }
      }
      return;
    }
    node.children?.forEach(visit);
  };
  return visit;
}
