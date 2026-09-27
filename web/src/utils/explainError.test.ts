import { describe, it, expect } from 'vitest';
import { explainMatchError, explainCompileError, stderrTail } from './explainError';

const failed = (code: number, text: string) => ({ status: 'failed' as const, error_code: code, error_message: text });
const header = (side: 1 | 2) =>
  `Программа ${side} завершилась с ошибкой:\n--- stderr ---\n${side === 1 ? 'left' : 'right'} player error: `;

describe('explainMatchError', () => {
  it('таймаут своей программы: вердикт от лица «вы» и подсказка про flush', () => {
    const e = explainMatchError(failed(2, header(2) + 'timed out waiting for fd to be ready'), 2);
    expect(e?.verdict).toBe('Ваша программа не ответила за 200 мс');
    expect(e?.hint).toContain('flush');
  });

  it('падение с хвостом stderr: причина из последней строки traceback', () => {
    const text =
      header(1) +
      'subprocess terminated unexpectedly\n--- stderr программы (последние 2 КБ) ---\n' +
      'Traceback (most recent call last):\n  File "main.py", line 9, in <module>\nZeroDivisionError: float division by zero\n';
    expect(explainMatchError(failed(1, text), 1)?.verdict).toBe(
      'Ваша программа завершилась посреди матча: ZeroDivisionError: float division by zero',
    );
    expect(stderrTail(text)).toContain('line 9');
  });

  it('ход не в том регистре и чужой модуль', () => {
    const move = explainMatchError(failed(1, header(1) + "unknown action 'cooperate', expected one of ['COOPERATE', 'DEFECT']"), 1);
    expect(move?.verdict).toBe('Ваша программа ответила «cooperate» вместо COOPERATE или DEFECT');
    expect(move?.hint).toContain('регистр');

    const lib = explainMatchError(
      failed(1, header(1) + "subprocess terminated unexpectedly\n--- stderr программы (последние 2 КБ) ---\nModuleNotFoundError: No module named 'numpy'"),
      1,
    );
    expect(lib?.hint).toContain('numpy');
  });

  it('соперник упал, зритель видит команду, сбой без стороны', () => {
    expect(explainMatchError(failed(1, 'Программа оппонента завершилась с ошибкой'), 2)?.verdict).toBe(
      'Программа соперника завершилась с ошибкой: победа ваша',
    );
    expect(explainMatchError(failed(2, header(2) + 'expected claim in [2, 100], got 101'), null, ['Голуби', 'Хаос'])?.verdict).toBe(
      'Программа команды «Хаос» заявила 101, а можно от 2 до 100',
    );
    expect(explainMatchError(failed(137, 'Ошибка выполнения (код 137):'), 1)?.verdict).toBe('Матч прерван системой (код 137)');
    expect(explainMatchError({ status: 'completed', error_code: 0, error_message: '' }, 1)).toBeNull();
  });
});

describe('explainCompileError', () => {
  it('частые ошибки сборки', () => {
    expect(explainCompileError("main.c:7:5: error: implicit declaration of function 'srand'")).toContain('#include <stdlib.h>');
    expect(explainCompileError('error[E0432]: unresolved import `rand`')).toContain('крейт rand');
    expect(explainCompileError('./main.go:5:2: "os" imported and not used')).toContain('неиспользуемыми');
    expect(explainCompileError('error: expected `;`')).toBeUndefined();
  });
});
