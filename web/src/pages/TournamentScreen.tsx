import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Link, useParams, useSearchParams } from 'react-router-dom';
import { motion } from 'motion/react';
import { DUR, EASE_OUT } from '../components/motion/tokens';
import {
  useCrossGameLeaderboard,
  useMatchesByRounds,
  useTournament,
  useTournamentGames,
} from '../hooks/queries';
import { useTournamentLive } from '../hooks/useTournamentLive';
import { usePlaceChanges } from '../hooks/usePlaceChanges';
import { useAuthStore } from '../store/authStore';
import { CrossGameLeaderboardTable } from '../components/tournament/LeaderboardTab';
import { LiveStatusLine } from '../components/tournament/LiveStatusLine';
import { QrCode } from '../components/tournament/QrCode';
import { Spinner } from '../components/ui/Spinner';
import { ErrorState } from '../components/ui/ErrorState';
import { gameProgress, honestStandings, liveGameIds, pageCount, pageSlice, type StandingRow } from '../utils/liveStandings';
import { getGameConfig } from '../utils/gameConfig';
import type { MatchResultPayload } from '../types/ws';
import type { CrossGameLeaderboardEntry, Game } from '../types';

const DEFAULT_ROTATE_S = 15;
const BANNER_MS = 20_000;
const IDLE_MS = 3000;
const FEED_SIZE = 4;
// во время раунда результаты идут десятками в секунду: лента обновляется раз в секунду
const FEED_FLUSH_MS = 1000;
const CEREMONY_PLACES = 10;

interface FeedItem {
  id: string;
  game: string;
  team1: string;
  team2: string;
  score1: number | null;
  score2: number | null;
  winner: number;
}

// Экран не гаснет, пока табло открыто и видно; вкладка в фоне замок теряет сама.
function useWakeLock() {
  useEffect(() => {
    if (!('wakeLock' in navigator)) return;
    let sentinel: WakeLockSentinel | null = null;
    let stopped = false;
    const acquire = () => {
      if (stopped || document.visibilityState !== 'visible') return;
      navigator.wakeLock.request('screen').then(
        (s) => {
          if (stopped) void s.release();
          else sentinel = s;
        },
        () => {}
      );
    };
    acquire();
    document.addEventListener('visibilitychange', acquire);
    return () => {
      stopped = true;
      document.removeEventListener('visibilitychange', acquire);
      void sentinel?.release();
    };
  }, []);
}

function useFullscreen() {
  const [active, setActive] = useState(() => !!document.fullscreenElement);
  useEffect(() => {
    const sync = () => setActive(!!document.fullscreenElement);
    document.addEventListener('fullscreenchange', sync);
    return () => {
      document.removeEventListener('fullscreenchange', sync);
      // уход с табло не оставляет приложение в полноэкранном режиме
      if (document.fullscreenElement) void document.exitFullscreen();
    };
  }, []);
  const toggle = useCallback(() => {
    if (document.fullscreenElement) void document.exitFullscreen();
    else void document.documentElement.requestFullscreen?.().catch(() => {});
  }, []);
  return { active, toggle };
}

// Курсор и панель управления прячутся, пока мышь не двигается.
function useIdle() {
  const [idle, setIdle] = useState(false);
  useEffect(() => {
    let t = setTimeout(() => setIdle(true), IDLE_MS);
    const wake = () => {
      setIdle(false);
      clearTimeout(t);
      t = setTimeout(() => setIdle(true), IDLE_MS);
    };
    window.addEventListener('mousemove', wake);
    window.addEventListener('pointerdown', wake);
    return () => {
      clearTimeout(t);
      window.removeEventListener('mousemove', wake);
      window.removeEventListener('pointerdown', wake);
    };
  }, []);
  return idle;
}

// Кнопки и ссылки сами обрабатывают пробел и Enter.
const ownsKey = (t: EventTarget | null) => t instanceof HTMLElement && !!t.closest('button, a, input, select, textarea');

function topOfGame(entries: CrossGameLeaderboardEntry[], game: Game) {
  return entries
    .filter((e) => e.game_ratings[game.id])
    .sort((a, b) => b.game_ratings[game.id].rating - a.game_ratings[game.id].rating || b.game_ratings[game.id].wins - a.game_ratings[game.id].wins)
    .slice(0, 3);
}

// Табло для проектора организатора: без шапки приложения, полноэкранный режим
// по F (выход по Esc), экран не гаснет. Строки таблицы листаются страницами,
// первые три закреплены; ←/→ листают, пробел ставит на паузу.
export function TournamentScreen() {
  const { id = '' } = useParams<{ id: string }>();
  const [params, setParams] = useSearchParams();
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated);
  const isAdmin = useAuthStore((s) => s.user?.role === 'admin');
  const rotateS = Math.max(5, Number(params.get('rotate')) || DEFAULT_ROTATE_S);
  const ceremony = isAdmin && params.get('view') === 'ceremony';

  const [feed, setFeed] = useState<FeedItem[]>([]);
  const incoming = useRef<FeedItem[]>([]);
  useEffect(() => {
    const t = setInterval(() => {
      if (incoming.current.length === 0) return;
      const fresh = incoming.current;
      incoming.current = [];
      setFeed((f) => [...fresh, ...f.filter((x) => !fresh.some((n) => n.id === x.id))].slice(0, FEED_SIZE));
    }, FEED_FLUSH_MS);
    return () => clearInterval(t);
  }, []);
  const gamesQuery = useTournamentGames(id);
  const games = useMemo(() => gamesQuery.data ?? [], [gamesQuery.data]);
  const onMatchResult = useCallback(
    (p: MatchResultPayload) => {
      // без названий команд (старый бэкенд) строку ленты не собрать
      if (p.team1_name === undefined || p.team2_name === undefined) return;
      const item: FeedItem = {
        id: p.match_id,
        game: getGameConfig(p.game_type ?? '').short ?? games.find((g) => g.name === p.game_type)?.display_name ?? '',
        team1: p.team1_name ?? '—',
        team2: p.team2_name ?? '—',
        score1: p.score1 ?? null,
        score2: p.score2 ?? null,
        winner: p.winner,
      };
      incoming.current = [item, ...incoming.current.filter((x) => x.id !== item.id)];
    },
    [games]
  );
  const live = useTournamentLive({ tournamentId: id, enabled: isAuthenticated, onMatchResult });
  const tournamentQuery = useTournament(id);
  const leaderboardQuery = useCrossGameLeaderboard(id, { pollInterval: live.pollInterval });
  const roundsQuery = useMatchesByRounds(id, { pollInterval: live.pollInterval });

  const progress = useMemo(() => gameProgress(roundsQuery.data ?? []), [roundsQuery.data]);
  const liveGames = useMemo(() => liveGameIds(games, progress), [games, progress]);
  const standings = useMemo(() => honestStandings(leaderboardQuery.data ?? [], liveGames), [leaderboardQuery.data, liveGames]);
  const changes = usePlaceChanges(standings);

  // «$ итог:» по игре, у которой только что доиграли матчи раунда
  const liveSig = [...progress.values()].filter((p) => p.live).map((p) => p.gameType).sort().join(',');
  const [prevLiveSig, setPrevLiveSig] = useState(liveSig);
  const [banner, setBanner] = useState<{ gameType: string } | null>(null);
  if (liveSig !== prevLiveSig) {
    setPrevLiveSig(liveSig);
    const nowLive = liveSig.split(',');
    const finished = prevLiveSig.split(',').filter((g) => g && !nowLive.includes(g) && progress.has(g));
    if (finished.length > 0) setBanner({ gameType: finished[finished.length - 1] });
  }
  const refetchLeaderboard = leaderboardQuery.refetch;
  useEffect(() => {
    if (!banner) return;
    // раунды могли узнать о конце раньше таблицы
    void refetchLeaderboard();
    const t = setTimeout(() => setBanner(null), BANNER_MS);
    return () => clearTimeout(t);
  }, [banner, refetchLeaderboard]);

  useWakeLock();
  const fullscreen = useFullscreen();
  const idle = useIdle();

  // страницы: строк столько, сколько влезает в область таблицы
  const [areaH, setAreaH] = useState(0);
  const measureArea = useCallback((el: HTMLDivElement | null) => {
    if (!el) return;
    const ro = new ResizeObserver(([e]) => setAreaH(e.contentRect.height));
    ro.observe(el);
    return () => ro.disconnect();
  }, []);
  const vh = window.innerHeight / 100;
  // те же высоты, что у шапки и строк broadcast-таблицы
  const perPage = areaH > 0 ? Math.max(4, Math.floor((areaH - Math.max(40, 7 * vh) - 4) / Math.max(36, 5 * vh))) : 10;
  const pages = pageCount(standings.length, perPage);
  const [pageRaw, setPage] = useState(0);
  const page = pageRaw % pages;
  const [paused, setPaused] = useState(false);

  useEffect(() => {
    if (paused || pages <= 1 || ceremony) return;
    const t = setInterval(() => setPage((p) => (p + 1) % pages), rotateS * 1000);
    return () => clearInterval(t);
  }, [paused, pages, ceremony, rotateS, page]);

  const placed = useMemo(() => standings.filter((r) => r.place !== null).slice(0, CEREMONY_PLACES), [standings]);
  const [revealed, setRevealed] = useState(0);
  const setCeremony = useCallback(
    (on: boolean) => {
      setRevealed(0);
      setParams((p) => {
        const next = new URLSearchParams(p);
        if (on) next.set('view', 'ceremony');
        else next.delete('view');
        return next;
      }, { replace: true });
    },
    [setParams]
  );

  const toggleFullscreen = fullscreen.toggle;
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.altKey || e.ctrlKey || e.metaKey) return;
      if (e.code === 'KeyF') {
        toggleFullscreen();
        return;
      }
      if ((e.key === ' ' || e.key === 'Enter') && ownsKey(e.target)) return;
      if (ceremony) {
        if (e.key === ' ' || e.key === 'Enter' || e.key === 'ArrowRight') {
          e.preventDefault();
          setRevealed((n) => Math.min(n + 1, placed.length));
        } else if (e.key === 'ArrowLeft') {
          setRevealed((n) => Math.max(n - 1, 0));
        } else if (e.key === 'Escape' && !document.fullscreenElement) {
          setCeremony(false);
        }
        return;
      }
      if (e.key === 'ArrowRight') setPage((p) => (p % pages + 1) % pages);
      else if (e.key === 'ArrowLeft') setPage((p) => (p % pages - 1 + pages) % pages);
      else if (e.key === ' ') {
        e.preventDefault();
        setPaused((v) => !v);
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [ceremony, pages, placed.length, setCeremony, toggleFullscreen]);

  const tournament = tournamentQuery.data;
  if (tournamentQuery.isError) {
    return (
      <main className="min-h-screen flex items-center justify-center">
        <title>Табло — TJudge</title>
        <h1 className="sr-only">Табло</h1>
        <ErrorState message="Турнир не найден или сервер недоступен" onRetry={() => void tournamentQuery.refetch()}>
          <Link to="/tournaments" className="btn btn-secondary">К списку турниров</Link>
        </ErrorState>
      </main>
    );
  }
  if (!tournament) {
    return (
      <main className="min-h-screen flex items-center justify-center text-gray-400">
        <Spinner>загрузка табло</Spinner>
      </main>
    );
  }

  const { pinned, body } = pageSlice(standings, perPage, page);
  const shortLink = `${window.location.host}/t/${tournament.code}`;
  const bannerGame = banner && games.find((g) => g.name === banner.gameType);
  const bannerTop = bannerGame ? topOfGame(leaderboardQuery.data ?? [], bannerGame) : [];

  return (
    <main
      className={`fixed inset-0 flex flex-col gap-[1.6vh] overflow-hidden bg-[#0a0a0b] px-[2.5vw] py-[2.5vh] text-gray-100 ${idle ? 'cursor-none' : ''}`}
    >
      <title>{`${tournament.name} — табло — TJudge`}</title>

      <header className="flex items-start justify-between gap-[2vw]">
        <div className="min-w-0">
          <p className="font-mono text-primary-400 text-[clamp(14px,1.9vh,26px)]">
            <span aria-hidden="true">$ </span>tjudge watch {tournament.code}
          </p>
          <h1 className="truncate font-bold leading-tight text-[clamp(28px,5.2vh,68px)]">{tournament.name}</h1>
        </div>
        {/* панель видна при движении мыши и при фокусе с клавиатуры */}
        <nav
          aria-label="Управление табло"
          className={`flex shrink-0 flex-wrap justify-end gap-2 transition-opacity focus-within:opacity-100 ${idle ? 'opacity-0' : 'opacity-100'}`}
        >
          <button type="button" onClick={toggleFullscreen} className="btn btn-sm btn-secondary">
            {fullscreen.active ? 'Выйти из полного экрана' : 'На весь экран'}
          </button>
          {!ceremony && pages > 1 && (
            <button type="button" onClick={() => setPaused((v) => !v)} aria-pressed={paused} className="btn btn-sm btn-secondary">
              Пауза
            </button>
          )}
          {isAdmin && (
            <button type="button" onClick={() => setCeremony(!ceremony)} className="btn btn-sm btn-secondary">
              {ceremony ? 'К таблице' : 'Церемония'}
            </button>
          )}
          <Link to={`/tournaments/${tournament.id}`} className="btn btn-sm btn-secondary">Закрыть</Link>
        </nav>
      </header>

      <LiveStatusLine
        progress={progress}
        games={games}
        status={tournament.status}
        isConnected={live.isConnected}
        isOnline={live.isOnline}
        updatedAt={Math.max(leaderboardQuery.dataUpdatedAt, roundsQuery.dataUpdatedAt)}
        className="text-[clamp(16px,2.4vh,32px)]"
      />

      <div role="status">
        {bannerGame && !ceremony && (
          <p className="truncate border-l-4 border-primary-500 bg-primary-900/30 px-[1vw] py-[0.8vh] font-mono text-[clamp(16px,2.4vh,34px)]">
            <span aria-hidden="true" className="text-primary-400">$ </span>
            итог: игра «{getGameConfig(bannerGame.name).short ?? bannerGame.display_name}» завершена
            {bannerTop.map((e, i) => (
              <span key={e.program_id} className="text-gray-300">
                {' · '}{i + 1}. <span className="font-bold text-gray-100">{e.team_name}</span>{' '}
                {bannerGame && e.game_ratings[bannerGame.id].rating.toLocaleString('ru-RU')}
              </span>
            ))}
          </p>
        )}
      </div>

      {ceremony ? (
        <Ceremony rows={placed} revealed={revealed} />
      ) : (
        <>
          <section ref={measureArea} aria-label="Таблица турнира" className="min-h-0 flex-1">
            <CrossGameLeaderboardTable
              rows={[...pinned, ...body]}
              games={games}
              live={liveGames}
              progress={progress}
              changes={changes}
              broadcast
              pinned={pinned.length}
            />
          </section>
          <footer className="flex items-end justify-between gap-[2vw] font-mono">
            <div className="min-w-0 flex-1 text-[clamp(14px,2vh,26px)]">
              <p className="mb-[0.6vh] text-gray-400">
                {pages > 1 && (
                  <span className="text-gray-300">
                    стр. {page + 1}/{pages}{paused ? ' · пауза' : ''}
                  </span>
                )}
                {pages > 1 && liveGames.size > 0 && ' · '}
                {liveGames.size > 0 && (
                  <span>
                    <span aria-hidden="true">{'// '}</span>
                    место — по доигранным играм, очки идущей (◐) предварительные
                  </span>
                )}
              </p>
              <Feed items={feed} />
            </div>
            <div className="flex shrink-0 items-center gap-[1vw]">
              <p className="text-right text-gray-300 text-[clamp(14px,2vh,26px)]">
                таблица на своём ПК
                <br />
                <span className="font-bold text-gray-100">{shortLink}</span>
              </p>
              <QrCode text={`${window.location.origin}/t/${tournament.code}`} className="h-[13vh] w-[13vh] min-h-20 min-w-20" />
            </div>
          </footer>
        </>
      )}
    </main>
  );
}

// «$ tail -f matches.log»: последние результаты с названиями команд, свежий слева.
function Feed({ items }: { items: FeedItem[] }) {
  return (
    <div className="flex min-w-0 items-baseline gap-[1vw] overflow-hidden whitespace-nowrap">
      <span className="shrink-0 text-primary-400">
        <span aria-hidden="true">$ </span>tail -f matches.log
      </span>
      {items.length === 0 ? (
        <span className="text-gray-400">
          <span aria-hidden="true">{'// '}</span>результаты появятся, когда пойдут матчи
        </span>
      ) : (
        items.map((m) => (
          <span key={m.id} className="shrink-0 text-gray-300">
            <span aria-hidden="true" className="text-gray-600">│ </span>
            <span className={m.winner === 1 ? 'font-bold text-gray-100' : ''}>{m.team1}</span>{' '}
            <span className="tabular-nums text-primary-300">{m.score1 ?? '?'}:{m.score2 ?? '?'}</span>{' '}
            <span className={m.winner === 2 ? 'font-bold text-gray-100' : ''}>{m.team2}</span>
            <span className="text-gray-400"> · {m.game}</span>
          </span>
        ))
      )}
    </div>
  );
}

const CUP = [
  '╔═══════╗',
  '║   1   ║',
  '╚╗     ╔╝',
  ' ╚═╗ ╔═╝ ',
  '   ║ ║   ',
  ' ╔═╝ ╚═╗ ',
  ' ╚═════╝ ',
].join('\n');

// Церемония: места раскрываются снизу вверх по пробелу или →, ← возвращает шаг.
function Ceremony({ rows, revealed }: { rows: StandingRow[]; revealed: number }) {
  // раскрыты последние revealed строк списка
  const isShown = (i: number) => i >= rows.length - revealed;
  const rest = rows.slice(3);
  const podium = [1, 0, 2].filter((i) => i < rows.length);
  const heights = ['h-[22vh]', 'h-[15vh]', 'h-[10vh]'];
  const reveal = { initial: { opacity: 0, y: 16 }, animate: { opacity: 1, y: 0 }, transition: { duration: DUR.slow, ease: EASE_OUT } };

  return (
    <section aria-label="Церемония награждения" className="flex min-h-0 flex-1 flex-col font-mono">
      <p className="text-primary-400 text-[clamp(16px,2.6vh,34px)]">
        <span aria-hidden="true">$ </span>церемония награждения
      </p>
      {/* две колонки, заполняются сверху вниз */}
      <ol
        className="mt-[1vh] grid grid-flow-col grid-cols-2 gap-x-[4vw] text-[clamp(16px,2.8vh,36px)]"
        style={{ gridTemplateRows: `repeat(${Math.ceil(rest.length / 2)}, auto)` }}
      >
        {rest.map((r, i) => (
          <li key={r.entry.program_id} className="flex gap-[1vw] py-[0.4vh] text-gray-300">
            <span className="w-[3ch] text-right tabular-nums text-gray-400">{r.place}.</span>
            {isShown(i + 3) ? (
              <motion.span {...reveal} className="flex flex-1 justify-between gap-[1vw]">
                <span className="truncate">{r.entry.team_name}</span>
                <span className="tabular-nums">{r.total.toLocaleString('ru-RU')}</span>
              </motion.span>
            ) : (
              <span className="text-gray-600">[ ? ]</span>
            )}
          </li>
        ))}
      </ol>
      <div className="flex min-h-0 flex-1 items-end justify-center gap-[2vw]">
        {podium.map((i) => {
          const r = rows[i];
          return (
            <div key={i} className="flex w-[26vw] flex-col items-center text-center">
              {isShown(i) && (
                <motion.div {...reveal} className="mb-[1vh] w-full">
                  {i === 0 && (
                    <pre aria-hidden="true" className="mb-[1vh] leading-none text-amber-400 text-[clamp(10px,1.8vh,24px)]">{CUP}</pre>
                  )}
                  <p className="truncate font-sans font-bold text-[clamp(22px,4vh,56px)]">{r.entry.team_name}</p>
                  <p className="tabular-nums text-gray-300 text-[clamp(16px,2.6vh,34px)]">{r.total.toLocaleString('ru-RU')}</p>
                </motion.div>
              )}
              <div
                className={`flex w-full items-start justify-center border-t-4 pt-[1vh] font-bold text-[clamp(28px,6vh,80px)] ${heights[i]} ${
                  i === 0 ? 'border-amber-400 bg-amber-900/20 text-amber-400' : i === 1 ? 'border-gray-300 bg-gray-700/30 text-gray-300' : 'border-orange-400 bg-orange-900/20 text-orange-400'
                }`}
              >
                {isShown(i) ? r.place : '?'}
              </div>
            </div>
          );
        })}
      </div>
      <p className="mt-[1vh] text-center text-gray-400 text-[clamp(14px,1.9vh,24px)]">
        <span aria-hidden="true">{'// '}</span>
        {revealed < rows.length
          ? `пробел или → — следующее место, ← — назад · раскрыто ${revealed} из ${rows.length}`
          : 'все места раскрыты · Esc — к таблице'}
      </p>
    </section>
  );
}
