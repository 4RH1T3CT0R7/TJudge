// @vitest-environment happy-dom
import { describe, it, expect } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { Login } from './Login';

describe('Login', () => {
  it('пароль в нативном поле, на экране звёздочки', () => {
    const { container } = render(
      <MemoryRouter>
        <Login />
      </MemoryRouter>
    );

    const input = screen.getByLabelText('Пароль') as HTMLInputElement;
    fireEvent.change(input, { target: { value: 'secret' } });

    // значение не подменяется: менеджеры паролей и IME видят обычное поле
    expect(input.type).toBe('password');
    expect(input.value).toBe('secret');
    const mask = container.querySelector('#password + span[aria-hidden="true"]');
    expect(mask?.textContent).toBe('******');
  });
});
