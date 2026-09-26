// @vitest-environment happy-dom
import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { JoinTeam } from './JoinTeam';
import api from '../api/client';
import type { Team } from '../types';

describe('JoinTeam', () => {
  it('вступает только по кнопке, а не при открытии ссылки', async () => {
    const join = vi.spyOn(api, 'joinTeamByCode').mockResolvedValue({ tournament_id: 't1' } as Team);
    render(
      <QueryClientProvider client={new QueryClient()}>
        <MemoryRouter initialEntries={['/join/ABCD2345']}>
          <Routes>
            <Route path="/join/:code" element={<JoinTeam />} />
            <Route path="/tournaments/:id" element={<p>турнир</p>} />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>
    );

    expect(join).not.toHaveBeenCalled();
    fireEvent.click(screen.getByText('Вступить в команду по коду ABCD2345'));
    await waitFor(() => expect(join).toHaveBeenCalledWith('ABCD2345'));
    expect(await screen.findByText('турнир')).toBeTruthy();
  });
});
