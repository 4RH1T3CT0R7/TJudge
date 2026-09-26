// @vitest-environment happy-dom
import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { ProtectedRoute } from './App';
import { AnimatedOutlet } from './components/motion/AnimatedOutlet';
import { Login } from './pages/Login';
import api from './api/client';
import { useAuthStore } from './store/authStore';
import type { AuthResponse } from './types';

describe('ProtectedRoute', () => {
  it('ссылка-приглашение: аноним входит и попадает обратно на /join/CODE', async () => {
    useAuthStore.setState({ user: null, isAuthenticated: false, isInitialized: true });
    const user = { id: 'u1', username: 'anya', email: 'a@b.c', role: 'user', created_at: '', updated_at: '' } as const;
    localStorage.setItem(`cinematic_first_login_${user.id}`, '1');
    vi.spyOn(api, 'login').mockResolvedValue({ user, access_token: 'a', refresh_token: 'r' } as AuthResponse);

    render(
      <MemoryRouter initialEntries={['/join/ABCD2345']}>
        <Routes>
          <Route path="/" element={<AnimatedOutlet />}>
            <Route index element={<p>главная</p>} />
            <Route path="login" element={<Login />} />
            <Route path="join/:code" element={<ProtectedRoute><p>страница приглашения</p></ProtectedRoute>} />
          </Route>
        </Routes>
      </MemoryRouter>
    );

    fireEvent.change(await screen.findByLabelText('// имя пользователя'), { target: { value: 'anya' } });
    fireEvent.change(screen.getByLabelText('Пароль'), { target: { value: 'secret' } });
    fireEvent.click(screen.getByRole('button', { name: 'auth.login()' }));

    expect(await screen.findByText('страница приглашения', undefined, { timeout: 3000 })).toBeTruthy();
  });
});
