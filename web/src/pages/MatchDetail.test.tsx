// @vitest-environment happy-dom
import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { MatchDetail } from './MatchDetail';
import api from '../api/client';
import type { Match, Tournament } from '../types';

const match: Match = {
  id: 'm1',
  tournament_id: 't1',
  program1_id: 'p1',
  program2_id: 'p2',
  game_type: 'dilemma',
  status: 'completed',
  round_number: 1,
  score1: 15,
  score2: 15,
  winner: 0,
  created_at: '2026-09-27T10:00:00Z',
  team1_id: 'a',
  team1_name: 'Альфа',
  team2_id: 'b',
  team2_name: 'Бета',
};

describe('MatchDetail', () => {
  it('ходы, обратный матч и счёт на выбранной итерации', async () => {
    vi.spyOn(api, 'getTournament').mockResolvedValue({ id: 't1', name: 'Кубок' } as Tournament);
    vi.spyOn(api, 'getTournamentGames').mockResolvedValue([]);
    vi.spyOn(api, 'getMatch').mockResolvedValue(match);
    vi.spyOn(api, 'getMatchTranscript').mockResolvedValue({ moves: [[1, 0, 1], [1, 1, 0]], points: [[5, 10, 0], [5, 0, 10]] });
    const pair = vi.spyOn(api, 'getProgramMatches').mockResolvedValue([
      match,
      { ...match, id: 'm2', program1_id: 'p2', program2_id: 'p1' },
    ]);

    render(
      <QueryClientProvider client={new QueryClient()}>
        <MemoryRouter initialEntries={['/tournaments/t1/matches/m1']}>
          <Routes>
            <Route path="/tournaments/:id/matches/:matchId" element={<MatchDetail />} />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>
    );

    expect(await screen.findByText('ход 2 — Альфа')).toBeTruthy();
    expect(pair).toHaveBeenCalledWith('t1', 'p1', 100, 'b');
    expect((await screen.findByText(/обратный матч/)).closest('a')?.getAttribute('href')).toBe('/tournaments/t1/matches/m2');

    // после первой итерации оба сотрудничали: 5:5
    fireEvent.change(screen.getByRole('slider', { name: 'Итерация' }), { target: { value: '1' } });
    expect(screen.getByText('счёт после 1 из 3:')).toBeTruthy();
    expect(screen.getAllByText('5')).toHaveLength(2);
  });
});
