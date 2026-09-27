import api from '../../api/client';
import { confirmDialog } from '../../store/confirmStore';
import { useToastStore } from '../../store/toastStore';
import type { MatchRound } from '../../types';

// Подтверждения необратимых действий админа. Числа последствий считаются прямо
// перед показом: запуск раунда - dry-run бэкенда тем же кодом, что и настоящий
// запуск, остальное - по счётчикам матчей с теми же статусами, что трогает действие.
// Ошибка запроса (нет участников, идут матчи) уходит вызывающему вместо диалога,
// тост о ней показывает клиент API.

type Forms = [one: string, few: string, many: string];

const pluralRules = new Intl.PluralRules('ru');

// 1 матч, 2 матча, 5 матчей
export function count(n: number, [one, few, many]: Forms) {
  const form = pluralRules.select(n);
  return `${n} ${form === 'one' ? one : form === 'few' ? few : many}`;
}

export const MATCHES: Forms = ['матч', 'матча', 'матчей'];
const UNFINISHED: Forms = ['недоигранный матч', 'недоигранных матча', 'недоигранных матчей'];

const sum = (rounds: MatchRound[], pick: (r: MatchRound) => number) =>
  rounds.reduce((acc, r) => acc + pick(r), 0);

interface GameRef {
  name: string;
  display_name: string;
}

// note - побочный эффект вызывающего (смена активной игры), последней строкой
export async function confirmRunRound(tournamentId: string, game: GameRef, note?: string) {
  const p = await api.previewGameRound(tournamentId, game.name);
  if (p.pending > 0) {
    return confirmDialog({
      title: 'Продолжение раунда',
      message: `Раунд игры «${game.display_name}» не доигран.`,
      details: [
        `вернёт в очередь ${count(p.pending, ['ожидающий матч', 'ожидающих матча', 'ожидающих матчей'])}`,
        'новый раунд не создастся, результаты сохранятся',
        ...(note ? [note] : []),
      ],
      confirmLabel: 'Продолжить раунд',
    });
  }
  const restart = p.matches_deleted > 0;
  const created = `создаст ${count(p.matches_created, MATCHES)} для ${count(p.participants, ['команды', 'команд', 'команд'])} с готовой программой`;
  return confirmDialog({
    title: restart ? 'Перезапуск раунда' : 'Запуск раунда',
    message: restart
      ? `Раунд игры «${game.display_name}» начнётся заново, результаты прошлого раунда пропадут.`
      : `Запустить раунд игры «${game.display_name}»?`,
    details: [
      ...(restart ? [`удалит ${count(p.matches_deleted, MATCHES)} прошлого раунда и историю рейтинга`] : []),
      created,
      ...(note ? [note] : []),
    ],
    confirmLabel: restart ? 'Перезапустить раунд' : 'Запустить раунд',
    danger: restart,
  });
}

export async function confirmResetRound(tournamentId: string, game: GameRef) {
  const rounds = (await api.getMatchesByRounds(tournamentId)).filter((r) => r.game_type === game.name);
  // сброс при идущих матчах бэкенд всё равно отклонит (409); запроса нет, поэтому
  // тост здесь: баннер ошибки у вызывающего бывает под фоном модалки или за экраном
  if (rounds.some((r) => r.running_count > 0)) {
    useToastStore.getState().addToast(`Идут матчи игры «${game.display_name}»: сброс возможен после их завершения`, 'error');
    return false;
  }
  return confirmDialog({
    title: 'Сброс раунда',
    message: `Результаты игры «${game.display_name}» будут стёрты.`,
    details: [
      `удалит ${count(sum(rounds, (r) => r.total_matches), MATCHES)} и историю рейтинга`,
      'обнулит рейтинг и статистику команд в этой игре',
    ],
    confirmLabel: 'Сбросить раунд',
    danger: true,
  });
}

export async function confirmCompleteTournament(tournament: { id: string; name: string }) {
  const rounds = await api.getMatchesByRounds(tournament.id);
  const unfinished = sum(rounds, (r) => r.pending_count + r.running_count);
  return confirmDialog({
    title: 'Завершение турнира',
    message: `Турнир «${tournament.name}» завершится, вернуть его в активные нельзя.`,
    details: [
      unfinished > 0 ? `отменит ${count(unfinished, UNFINISHED)}` : 'недоигранных матчей нет',
      'таблица зафиксируется, новые раунды запустить будет нельзя',
    ],
    typeToConfirm: tournament.name,
    confirmLabel: 'Завершить турнир',
    danger: true,
  });
}

export async function confirmDisqualify(tournamentId: string, team: { id: string; name: string }) {
  const rounds = await api.getMatchesByRounds(tournamentId, undefined, team.id);
  const played = sum(rounds, (r) => r.completed_count + r.failed_count);
  const unfinished = sum(rounds, (r) => r.pending_count + r.running_count);
  return confirmDialog({
    title: 'Дисквалификация',
    message: `Команда «${team.name}» выбывает из турнира.`,
    details: [
      `удалит ${count(played, ['сыгранный матч', 'сыгранных матча', 'сыгранных матчей'])} команды, очки соперников за них тоже пропадут`,
      unfinished > 0 ? `отменит ${count(unfinished, UNFINISHED)}` : 'недоигранных матчей нет',
      'восстановление вернёт команду, но не удалённые матчи',
    ],
    typeToConfirm: team.name,
    confirmLabel: 'Дисквалифицировать',
    danger: true,
  });
}

export async function confirmDeleteTournament(tournament: { id: string; name: string }) {
  const [teams, rounds] = await Promise.all([
    api.getTournamentTeams(tournament.id),
    api.getMatchesByRounds(tournament.id),
  ]);
  return confirmDialog({
    title: 'Удаление турнира',
    message: `Турнир «${tournament.name}» удалится со всеми данными, восстановить его нельзя.`,
    details: [
      `удалит ${count(teams.length, ['команду', 'команды', 'команд'])} с программами`,
      `удалит ${count(sum(rounds, (r) => r.total_matches), MATCHES)} и историю рейтинга`,
    ],
    typeToConfirm: tournament.name,
    confirmLabel: 'Удалить турнир',
    danger: true,
  });
}

// Число турниров с игрой клиент надёжно не посчитает: список турниров
// постраничный, а игры турнира - запрос на каждый
export function confirmDeleteGame(game: { display_name: string }) {
  return confirmDialog({
    title: 'Удаление игры',
    message: `Игра «${game.display_name}» удалится вместе с правилами.`,
    details: [
      'уберёт игру из всех турниров, где она добавлена',
      'если по игре уже загружены программы, сервер удаление отклонит',
    ],
    confirmLabel: 'Удалить игру',
    danger: true,
  });
}

export async function confirmClearQueue() {
  const stats = await api.getMatchStatistics();
  return confirmDialog({
    title: 'Очистка очереди',
    message: 'Очередь матчей очистится во всех турнирах.',
    details: [
      stats.pending > 0
        ? `отменит ${count(stats.pending, ['ожидающий матч', 'ожидающих матча', 'ожидающих матчей'])}`
        : 'ожидающих матчей нет',
      'идущие матчи доиграют',
    ],
    confirmLabel: 'Очистить очередь',
    danger: true,
  });
}
