// @vitest-environment happy-dom
import { act, useState } from 'react';
import { expect, it } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/react';
import { ErrorState } from './ErrorState';

function Page() {
  const [failed, setFailed] = useState(true);
  return (
    <main id="main-content">
      <h1>Турниры</h1>
      {failed && <ErrorState message="сеть недоступна" onRetry={() => setFailed(false)} />}
    </main>
  );
}

it('после удачного повтора переводит фокус на h1 страницы', () => {
  render(<Page />);
  const retry = screen.getByRole('button', { name: 'Повторить' });
  act(() => retry.focus());
  fireEvent.click(retry);
  expect(screen.queryByRole('alert')).toBeNull();
  expect(document.activeElement).toBe(screen.getByRole('heading', { name: 'Турниры' }));
});
