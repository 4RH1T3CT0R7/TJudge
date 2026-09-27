import { TerminalOutput } from '../ui/TerminalOutput';
import { explainMatchError, failedSide, type Side } from '../../utils/explainError';
import type { Match } from '../../types';

interface MatchErrorProps {
  match: Pick<Match, 'status' | 'error_code' | 'error_message'>;
  /** Сторона своей программы; null - зритель или админ. */
  mySide: Side | null;
  /** Названия команд сторон. */
  names?: [string?, string?];
  /** Без вывода stderr: только вердикт и подсказка. */
  brief?: boolean;
}

// Обезличенные тексты, которыми бэкенд заменяет чужую ошибку (handlers/match.go redactMatchErrors).
const REDACTED = ['Программа оппонента завершилась с ошибкой', 'Ошибка выполнения матча'];

// Упавший матч: вердикт по-русски, подсказка и сам текст ошибки в терминальном окне.
// Обезличенную фразу вместо чужой ошибки под вердиктом не повторяет.
export function MatchError({ match, mySide, names, brief = false }: MatchErrorProps) {
  const e = explainMatchError(match, mySide, names);
  if (!e) return null;
  const opponentFailed = mySide !== null && failedSide(match) !== null && failedSide(match) !== mySide;
  const text = match.error_message ?? '';
  const hasDetails = text !== '' && !REDACTED.includes(text);

  return (
    <div className="space-y-2">
      <p className={`text-sm ${opponentFailed ? 'text-gray-300' : 'text-red-300'}`}>
        <span aria-hidden="true" className="font-mono">{opponentFailed ? '● ' : '✕ '}</span>
        {e.verdict}
      </p>
      {e.hint && (
        <p className="font-mono text-xs text-gray-500">
          <span aria-hidden="true">{'// '}</span>
          {e.hint}
        </p>
      )}
      {!brief && hasDetails && <TerminalOutput label="stderr:" text={text} wrap />}
    </div>
  );
}
