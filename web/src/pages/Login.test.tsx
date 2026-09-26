// @vitest-environment happy-dom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { AxiosError, AxiosHeaders, type AxiosResponse } from 'axios';
import { Login } from './Login';
import api from '../api/client';

function httpError(status: number, headers: Record<string, string> = {}) {
  const config = { headers: new AxiosHeaders() };
  const response = { status, statusText: '', data: {}, headers, config } as AxiosResponse;
  return new AxiosError('fail', undefined, config, null, response);
}

async function submitWith(err: unknown) {
  vi.spyOn(api, 'login').mockRejectedValue(err);
  render(
    <MemoryRouter>
      <Login />
    </MemoryRouter>
  );
  fireEvent.change(screen.getByLabelText('// имя пользователя'), { target: { value: 'anya' } });
  fireEvent.change(screen.getByLabelText('Пароль'), { target: { value: 'secret' } });
  fireEvent.click(screen.getByRole('button', { name: 'auth.login()' }));
  return screen.findByRole('alert');
}

describe('Login', () => {
  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

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

  it.each([
    [httpError(401), 'неверный логин или пароль'],
    [new AxiosError('Network Error', 'ERR_NETWORK'), 'нет связи с сервером'],
    [httpError(502), 'сервер недоступен'],
  ])('сообщение по ответу сервера: %#', async (err, text) => {
    expect((await submitWith(err)).textContent).toContain(text);
  });

  it('401 держится до правки поля', async () => {
    const alert = await submitWith(httpError(401));
    fireEvent.change(screen.getByLabelText('Пароль'), { target: { value: 'secret2' } });
    expect(alert.textContent).not.toContain('неверный');
  });

  it('429 блокирует вход на Retry-After секунд', async () => {
    const alert = await submitWith(httpError(429, { 'retry-after': '42' }));
    expect(alert.textContent).toContain('подождите 42 с');
    const button = screen.getByRole('button', { name: /повтор через 42 с/ }) as HTMLButtonElement;
    expect(button.disabled).toBe(true);
    // правка поля отсчёт не снимает
    fireEvent.change(screen.getByLabelText('Пароль'), { target: { value: 'secret2' } });
    expect(alert.textContent).toContain('подождите 42 с');
  });
});
