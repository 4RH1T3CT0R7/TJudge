// @vitest-environment happy-dom
import { describe, it, expect } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { QuestTerminal } from './QuestTerminal';
import { useQuestState } from '../../hooks/useQuestState';

function Terminal() {
  const { state, dispatch } = useQuestState();
  return <QuestTerminal state={state} dispatch={dispatch} />;
}

describe('QuestTerminal', () => {
  it('Tab дополняет команду, а без вариантов не держит фокус', () => {
    render(<Terminal />);
    const input = screen.getByLabelText('Команда терминала') as HTMLInputElement;

    // fireEvent возвращает false, если обработчик вызвал preventDefault
    expect(fireEvent.keyDown(input, { key: 'Tab' })).toBe(true);
    expect(fireEvent.keyDown(input, { key: 'Tab', shiftKey: true })).toBe(true);

    fireEvent.change(input, { target: { value: 'whoa' } });
    expect(fireEvent.keyDown(input, { key: 'Tab' })).toBe(false);
    expect(input.value).toBe('whoami');
    // дополнять больше нечего - Tab уходит дальше
    expect(fireEvent.keyDown(input, { key: 'Tab' })).toBe(true);
  });
});
