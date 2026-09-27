import { useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import api from '../../api/client';
import { queryKeys } from '../../api/queryKeys';
import { useToastStore } from '../../store/toastStore';
import { MATCHES, confirmResetRound, confirmRunRound, count } from '../admin/confirmations';
import { extractErrorMessage, waitForMatches } from './helpers';
import type { Game, Tournament } from '../../types';

// Админ-действия над играми турнира (запуск раунда, активная игра, сброс раунда).
// Запуск и сброс спрашивают подтверждение с числами последствий.
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

    setActionError(null);
    const nextGame = games[games.findIndex((g) => g.id === gameId) + 1];
    const switchNote = nextGame
      ? `после запуска активной станет «${nextGame.display_name}»`
      : 'это последняя игра: активных игр не останется';
    try {
      if (!(await confirmRunRound(tournamentId, { name: gameName, display_name: gameDisplayName }, switchNote))) return;
    } catch (err: unknown) {
      setActionError(extractErrorMessage(err, 'Не удалось подготовить запуск раунда'));
      return;
    }

    setRunningGameId(gameId);
    try {
      const result = await api.runGameMatches(tournamentId, gameName);

      // Find current game index and check if there's a next game
      const currentIndex = games.findIndex(g => g.id === gameId);
      const isLastGame = currentIndex === games.length - 1;

      if (!isLastGame) {
        // Switch to the next game
        const nextGame = games[currentIndex + 1];
        await api.setActiveGame(tournamentId, nextGame.id);
        useToastStore.getState().addToast(`Запущено ${count(result.enqueued, MATCHES)} для "${gameDisplayName}". Активная игра переключена на "${nextGame.display_name}". Ожидание завершения матчей...`, 'success', 8000);
      } else {
        // Last game - deactivate all games
        await api.deactivateAllGames(tournamentId);
        useToastStore.getState().addToast(`Запущено ${count(result.enqueued, MATCHES)} для "${gameDisplayName}". Это была последняя игра в турнире. Все игры деактивированы. Ожидание завершения матчей...`, 'success', 8000);
      }

      // Wait for matches to complete and auto-retry if needed (runs in background)
      void waitForMatches(queryClient, tournamentId, result.enqueued).then(() => {
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
      setActionError(extractErrorMessage(err, 'Не удалось запустить матчи'));
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
      setActionError(extractErrorMessage(err, 'Не удалось установить активную игру'));
    } finally {
      setSettingActiveGameId(null);
    }
  };

  // Reset game round (delete all matches and reset ratings)
  const handleResetGameRound = async (gameId: string, gameName: string, gameDisplayName: string) => {
    if (!tournamentId) return;

    setActionError(null);
    try {
      if (!(await confirmResetRound(tournamentId, { name: gameName, display_name: gameDisplayName }))) return;
    } catch (err: unknown) {
      setActionError(extractErrorMessage(err, 'Не удалось подготовить сброс раунда'));
      return;
    }

    setResettingGameId(gameId);
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
      setActionError(extractErrorMessage(err, 'Не удалось сбросить раунд'));
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
