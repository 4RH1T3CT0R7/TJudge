import { useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import api from '../../api/client';
import { queryKeys } from '../../api/queryKeys';
import { useToastStore } from '../../store/toastStore';
import { confirmDialog } from '../../store/confirmStore';
import { waitForMatchesAndAutoRetry } from './helpers';
import type { Game, Tournament } from '../../types';

// Админ-действия над играми турнира (запуск раунда, активная игра, сброс раунда).
// Логика перенесена из TournamentDetail как есть, поведение не менялось.
export function useGameAdminActions({
  tournamentId,
  tournament,
  games,
  setActionError,
}: {
  tournamentId: string;
  tournament: Tournament | null;
  games: Game[];
  setActionError: (error: string | null) => void;
}) {
  const queryClient = useQueryClient();

  // Games status state (for active game management)
  const [runningGameId, setRunningGameId] = useState<string | null>(null);
  const [settingActiveGameId, setSettingActiveGameId] = useState<string | null>(null);
  const [resettingGameId, setResettingGameId] = useState<string | null>(null);

  // Run matches for a specific game
  const handleRunGameMatches = async (gameId: string, gameName: string, gameDisplayName: string) => {
    if (!tournament || !tournamentId) return;

    setRunningGameId(gameId);
    setActionError(null);
    try {
      const result = await api.runGameMatches(tournamentId, gameName);

      // Find current game index and check if there's a next game
      const currentIndex = games.findIndex(g => g.id === gameId);
      const isLastGame = currentIndex === games.length - 1;

      if (!isLastGame) {
        // Switch to the next game
        const nextGame = games[currentIndex + 1];
        await api.setActiveGame(tournamentId, nextGame.id);
        useToastStore.getState().addToast(`Запущено ${result.enqueued} матчей для "${gameDisplayName}". Активная игра переключена на "${nextGame.display_name}". Ожидание завершения матчей...`, 'success', 8000);
      } else {
        // Last game - deactivate all games
        await api.deactivateAllGames(tournamentId);
        useToastStore.getState().addToast(`Запущено ${result.enqueued} матчей для "${gameDisplayName}". Это была последняя игра в турнире. Все игры деактивированы. Ожидание завершения матчей...`, 'success', 8000);
      }

      // Wait for matches to complete and auto-retry if needed (runs in background)
      void waitForMatchesAndAutoRetry(queryClient, tournamentId, result.enqueued).then(() => {
        // Final refresh after all matches complete
        void queryClient.invalidateQueries({ queryKey: queryKeys.tournamentGamesStatus(tournamentId) });
        void queryClient.invalidateQueries({ queryKey: queryKeys.matchesByRounds(tournamentId) });
        void queryClient.invalidateQueries({ queryKey: queryKeys.crossGameLeaderboard(tournamentId) });
      });

      // Immediate refresh
      void queryClient.invalidateQueries({ queryKey: queryKeys.tournamentGamesStatus(tournamentId) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.matchesByRounds(tournamentId) });
    } catch (err: unknown) {
      console.error('Failed to run game matches:', err);
      const axiosErr = err as { response?: { data?: { message?: string } } };
      setActionError(axiosErr.response?.data?.message || 'Не удалось запустить матчи');
    } finally {
      setRunningGameId(null);
    }
  };

  // Set active game for tournament
  const handleSetActiveGame = async (gameId: string) => {
    if (!tournamentId) return;

    setSettingActiveGameId(gameId);
    setActionError(null);
    try {
      await api.setActiveGame(tournamentId, gameId);
      // Reload games status
      await queryClient.invalidateQueries({ queryKey: queryKeys.tournamentGamesStatus(tournamentId) });
    } catch (err: unknown) {
      console.error('Failed to set active game:', err);
      const axiosErr = err as { response?: { data?: { message?: string } } };
      setActionError(axiosErr.response?.data?.message || 'Не удалось установить активную игру');
    } finally {
      setSettingActiveGameId(null);
    }
  };

  // Reset game round (delete all matches and reset ratings)
  const handleResetGameRound = async (gameId: string, gameDisplayName: string) => {
    if (!tournamentId) return;

    const confirmed = await confirmDialog({
      title: 'Сброс раунда',
      message:
        `Сбросить раунд для игры "${gameDisplayName}"?\n\n` +
        'Это действие:\n' +
        '- удалит все матчи этой игры\n' +
        '- сбросит рейтинги всех участников до 1000\n' +
        '- сбросит номер раунда\n\n' +
        'Это действие необратимо!',
      confirmLabel: 'Сбросить',
      danger: true,
    });

    if (!confirmed) return;

    setResettingGameId(gameId);
    setActionError(null);
    try {
      const result = await api.resetGameRound(tournamentId, gameId);
      useToastStore.getState().addToast(
        `Раунд сброшен: матчей удалено ${result.matches_deleted}, рейтингов сброшено ${result.participants_reset}`,
        'success',
        8000
      );
      // Reload games status, matches and leaderboard (ratings were reset)
      void queryClient.invalidateQueries({ queryKey: queryKeys.tournamentGamesStatus(tournamentId) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.matchesByRounds(tournamentId) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.crossGameLeaderboard(tournamentId) });
    } catch (err: unknown) {
      console.error('Failed to reset game round:', err);
      const axiosErr = err as { response?: { data?: { message?: string } } };
      setActionError(axiosErr.response?.data?.message || 'Не удалось сбросить раунд');
    } finally {
      setResettingGameId(null);
    }
  };

  return {
    runningGameId,
    settingActiveGameId,
    resettingGameId,
    handleRunGameMatches,
    handleSetActiveGame,
    handleResetGameRound,
  };
}
