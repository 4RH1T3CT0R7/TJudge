import { useEffect, useMemo, useState } from 'react';
import type { ReactNode } from 'react';
import { Link, useParams } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import api, { isRetryableError } from '../api/client';
import { queryKeys } from '../api/queryKeys';
import { useMyTeam, useTournament, useTournamentGames } from '../hooks/queries';
import { useMotionPref } from '../hooks/useMotionPref';
import { useAuthStore } from '../store/authStore';
import { PageHeader } from '../components/ui/PageHeader';
import { StatusLabel } from '../components/ui/StatusLabel';
import { Spinner } from '../components/ui/Spinner';
import { EmptyState } from '../components/ui/EmptyState';
import { ErrorState } from '../components/ui/ErrorState';
import { YouMark } from '../components/ui/YouMark';
import { MatchError } from '../components/tournament/MatchError';
import { GAME_PAYOFFS, getGameConfig } from '../utils/gameConfig';
import {
  COOPERATE,
  auctionTurns,
  averageMove,
  cooperationShare,
  firstDefection,
  iterationsOf,
  runningTotal,
  sharePct,
} from '../utils/transcript';
import type { Side } from '../utils/explainError';
import type { Match, MatchTranscript } from '../types';

// что программа сообщает за ход в числовых играх: [подпись, среднее]
const MOVE_LABEL: Record<string, [string, string]> = {
  tug_of_war: ['трата энергии', 'средняя трата энергии'],
  travelers_dilemma: ['заявка', 'средняя заявка'],
  public_goods: ['вклад', 'средний вклад'],
};

// линии сторон на графике счёта (проверенная пара для тёмного фона)
const SIDE_COLOR = { 1: '#3987e5', 2: '#d95926' } as const;

// Цвет стороны с графика счёта - у её имени и над полосой, и в таблице: иначе
// оранжевую линию легко принять за предательство.
function SideMark({ side, className = 'mr-1.5' }: { side: Side; className?: string }) {
  return <span aria-hidden="true" className={`inline-block h-0.5 w-4 shrink-0 align-middle ${className}`} style={{ background: SIDE_COLOR[side] }} />;
}

// автопроигрывание: кадр раз в 60 мс, весь матч укладывается примерно в 6-9 с
const TICK_MS = 60;
const TICKS_PER_REPLAY = 150;

const pct = (x: number | null) => (x === null ? '—' : sharePct(x));
const num = (x: number | null) => (x === null ? '—' : x.toLocaleString('ru-RU', { maximumFractionDigits: 1 }));

// Разбор матча: счёт, ходы обеих сторон по итерациям, график счёта и проигрывание.
export function MatchDetail() {
  const { id: tournamentId = '', matchId = '' } = useParams<{ id: string; matchId: string }>();
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated);

  const tournamentQuery = useTournament(tournamentId);
  const gamesQuery = useTournamentGames(tournamentId);
  const myTeamQuery = useMyTeam(tournamentId, { enabled: isAuthenticated });
  const matchQuery = useQuery({
    queryKey: queryKeys.match(matchId),
    queryFn: () => api.getMatch(matchId),
    enabled: !!matchId,
    // матч в очереди или идёт: результат будет через секунды
    refetchInterval: (q) => (q.state.data?.status === 'pending' || q.state.data?.status === 'running' ? 3000 : false),
  });
  const match = matchQuery.data;
  const finished = match?.status === 'completed' || match?.status === 'failed';
  const transcriptQuery = useQuery({
    queryKey: queryKeys.matchTranscript(matchId),
    queryFn: () => api.getMatchTranscript(matchId),
    enabled: finished,
    // ходы сыгранного матча не меняются
    staleTime: Infinity,
  });
  // обратная ориентация: та же пара в том же раунде, программы поменялись местами
  const reverseQuery = useQuery({
    queryKey: [...queryKeys.match(matchId), 'reverse'],
    queryFn: async () => {
      const m = match!;
      const pair = await api.getProgramMatches(m.tournament_id, m.program1_id, 100, m.team2_id ?? undefined);
      return (
        pair.find((r) => r.program1_id === m.program2_id && r.program2_id === m.program1_id && r.round_number === m.round_number) ?? null
      );
    },
    enabled: !!match,
    staleTime: 60_000,
  });

  if (matchQuery.isPending) {
    return <div className="flex justify-center py-24 text-sm text-gray-400"><Spinner>загрузка матча</Spinner></div>;
  }
  if (!match) {
    const canRetry = isRetryableError(matchQuery.error);
    return (
      <ErrorState
        message={canRetry ? 'Не удалось загрузить матч' : 'Матч не найден: раунд игры могли перезапустить'}
        onRetry={canRetry ? () => void matchQuery.refetch() : undefined}
      >
        <Link to={`/tournaments/${tournamentId}?tab=matches`} className="btn btn-secondary">К матчам турнира</Link>
      </ErrorState>
    );
  }

  const game = gamesQuery.data?.find((g) => g.name === match.game_type);
  const gameName = game?.display_name ?? match.game_type;
  const myTeamId = myTeamQuery.data?.id;
  const mySide: Side | null = !myTeamId ? null : match.team1_id === myTeamId ? 1 : match.team2_id === myTeamId ? 2 : null;
  const names: [string, string] = [
    match.team1_name ?? `программа ${match.program1_id.slice(0, 8)}`,
    match.team2_name ?? `программа ${match.program2_id.slice(0, 8)}`,
  ];
  const reverse = reverseQuery.data;
  const gameHref = game ? `/tournaments/${tournamentId}/games/${game.id}?tab=matches` : `/tournaments/${tournamentId}?tab=matches`;

  return (
    <div>
      <PageHeader
        crumbs={[
          { label: 'турниры', to: '/tournaments' },
          { label: tournamentQuery.data?.name ?? 'турнир', to: `/tournaments/${tournamentId}` },
          { label: gameName, to: gameHref },
          { label: 'матч' },
        ]}
        title={
          <>
            <title>{`${names[0]} — ${names[1]} — TJudge`}</title>
            {names[0]} — {names[1]}
          </>
        }
        status={<StatusLabel entity="match" status={match.status} />}
      >
        <p className="flex flex-wrap items-center gap-x-4 gap-y-1 font-mono text-sm text-gray-400">
          <span>
            <span aria-hidden="true" className={`mr-1 ${getGameConfig(match.game_type).textClass}`}>{getGameConfig(match.game_type).icon}</span>
            {gameName} · раунд {match.round_number}
          </span>
          {reverse && (
            <Link to={`/tournaments/${tournamentId}/matches/${reverse.id}`} className="text-primary-400 underline hover:text-primary-300">
              <span aria-hidden="true">⇄ </span>обратный матч: {names[1]} — {names[0]}
            </Link>
          )}
          <Link to={gameHref} className="text-primary-400 underline hover:text-primary-300">все матчи игры</Link>
        </p>
      </PageHeader>

      <Scoreboard match={match} names={names} mySide={mySide} />

      <section aria-labelledby="moves-title" className="card mt-6">
        <h2 id="moves-title" className="mb-4 text-lg font-semibold text-gray-100">Ходы</h2>
        {!finished ? (
          <EmptyState command="ходы" hint="матч ещё не сыгран: ходы появятся, когда он закончится" />
        ) : transcriptQuery.isPending ? (
          <p className="py-6 text-sm text-gray-400"><Spinner>загрузка ходов</Spinner></p>
        ) : transcriptQuery.isError ? (
          <ErrorState message="Не удалось загрузить ходы матча" onRetry={() => void transcriptQuery.refetch()} />
        ) : !transcriptQuery.data || iterationsOf(transcriptQuery.data) === 0 ? (
          <EmptyState
            command="ходы"
            hint="ходы этого матча не записаны: он сыгран до включения записи или с числом итераций больше 1000"
          />
        ) : match.game_type === 'dollar_auction' ? (
          <AuctionBids transcript={transcriptQuery.data} names={names} />
        ) : (
          <Replay transcript={transcriptQuery.data} gameType={match.game_type} names={names} />
        )}
      </section>
    </div>
  );
}

// На телефоне названия сторон - строкой над счётом и переносятся целиком:
// в три колонки рядом со счётом от них оставалось по несколько букв.
function Scoreboard({ match, names, mySide }: { match: Match; names: [string, string]; mySide: Side | null }) {
  const side = (s: Side) => (
    <div className={`min-w-0 ${s === 2 ? 'text-right' : ''}`}>
      <p className="break-words text-sm text-gray-300">
        {/* метка в начале, как в матрице встреч: не теряется с хвостом длинного названия */}
        {/* пробел после метки - место переноса перед названием */}
        {mySide === s && <><YouMark className="mr-1 text-xs" />{' '}</>}
        {names[s - 1]}
      </p>
      <p className="font-mono text-xs text-gray-500">игрок {s}</p>
    </div>
  );
  const score = (s: Side) => (
    <span className={match.winner === s ? 'text-green-400' : 'text-gray-100'}>{(s === 1 ? match.score1 : match.score2) ?? '–'}</span>
  );
  const result =
    match.winner === 1 || match.winner === 2
      ? mySide
        ? `${match.winner === mySide ? 'победа' : 'поражение'} «${names[mySide - 1]}» (вы)`
        : `победа «${names[match.winner - 1]}»`
      : match.status === 'completed' ? 'ничья' : null;

  return (
    <div className={`card ${mySide ? 'row-mine' : ''}`}>
      <div className="grid grid-cols-2 items-center gap-x-4 gap-y-3 sm:grid-cols-[1fr_auto_1fr]">
        {side(1)}
        <p className="col-span-2 row-start-2 text-center font-mono text-3xl font-bold tabular-nums sm:col-span-1 sm:row-start-auto">
          {score(1)}
          <span className="mx-2 text-gray-500">:</span>
          {score(2)}
        </p>
        {side(2)}
      </div>
      {result && <p className="mt-2 text-center text-sm text-gray-400">{result}</p>}
      {match.status === 'failed' && (
        <div className="mt-3">
          <MatchError match={match} mySide={mySide} names={names} />
        </div>
      )}
    </div>
  );
}

// Проигрывание матча: полосы ходов, график счёта и таблица итераций показывают
// состояние после step итераций. Автопроигрывание только без reduced motion,
// шаги и ползунок работают всегда.
function Replay({ transcript, gameType, names }: { transcript: MatchTranscript; gameType: string; names: [string, string] }) {
  const n = iterationsOf(transcript);
  const [step, setStep] = useState(n);
  const [playing, setPlaying] = useState(false);
  const { reduced } = useMotionPref();
  const isPlaying = playing && !reduced && step < n;
  const stride = Math.max(1, Math.ceil(n / TICKS_PER_REPLAY));

  useEffect(() => {
    if (!isPlaying) return;
    const t = setInterval(() => setStep((s) => Math.min(n, s + stride)), TICK_MS);
    return () => clearInterval(t);
  }, [isPlaying, n, stride]);

  // ручные шаги останавливают проигрывание
  const seek = (s: number) => {
    setPlaying(false);
    setStep(Math.max(0, Math.min(n, s)));
  };
  const totals = useMemo(
    () => (transcript.points ? ([0, 1] as const).map((i) => [0, ...runningTotal(transcript.points![i])]) : null),
    [transcript]
  );
  const dilemma = gameType === 'dilemma';
  const range = useMemo(() => {
    const all = [...(transcript.moves[0] ?? []), ...(transcript.moves[1] ?? [])];
    return { min: Math.min(...all), max: Math.max(...all) };
  }, [transcript]);
  const first = dilemma ? firstDefection(transcript) : null;
  const [moveLabel, avgLabel] = MOVE_LABEL[gameType] ?? ['ход', 'средний ход'];

  // на телефоне шаги - главный способ идти по ходам (и единственный при reduced
  // motion), поэтому кнопки крупнее, а ползунок со счётчиком - своей строкой
  const stepBtn = 'btn btn-sm btn-secondary min-h-10 min-w-10 text-base sm:min-h-0 sm:min-w-0 sm:text-xs';
  return (
    <div className="space-y-5">
      <div className="flex flex-wrap items-center gap-2 font-mono text-sm">
        {!reduced && (
          <button
            type="button"
            onClick={() => {
              if (isPlaying) return setPlaying(false);
              if (step >= n) setStep(0);
              setPlaying(true);
            }}
            className="btn btn-sm btn-primary min-h-10 w-32 justify-center sm:min-h-0"
          >
            <span aria-hidden="true">{isPlaying ? '❚❚' : '▶'}</span>
            {isPlaying ? 'пауза' : 'проиграть'}
          </button>
        )}
        <button type="button" onClick={() => seek(step - 1)} disabled={step === 0} className={stepBtn} aria-label="На ход назад">
          ‹
        </button>
        <button type="button" onClick={() => seek(step + 1)} disabled={step === n} className={stepBtn} aria-label="На ход вперёд">
          ›
        </button>
        <div className="flex basis-full items-center gap-3 sm:basis-0 sm:flex-1">
          <input
            type="range"
            min={0}
            max={n}
            value={step}
            onChange={(e) => seek(Number(e.target.value))}
            aria-label="Итерация"
            aria-valuetext={`после ${step} из ${n}`}
            className="min-w-0 flex-1 accent-primary-400"
          />
          <span className="shrink-0 tabular-nums text-gray-400">{step} / {n}</span>
        </div>
      </div>

      <div className="space-y-2">
        {([1, 2] as const).map((s) => (
          <div key={s}>
            <p className="mb-1 truncate text-xs text-gray-300">
              <SideMark side={s} />
              {names[s - 1]}
            </p>
            <MoveStrip moves={transcript.moves[s - 1] ?? []} n={n} step={step} dilemma={dilemma} range={range} onSeek={seek} name={names[s - 1]} />
          </div>
        ))}
        <p className="font-mono text-xs text-gray-500">
          {dilemma ? (
            <>
              <span aria-hidden="true" className="text-primary-500">■</span> сотрудничество{' '}
              <span aria-hidden="true" className="ml-2 text-red-500">■</span> предательство
            </>
          ) : (
            <>
              <span aria-hidden="true">{'// '}</span>
              {moveLabel}: ярче — больше, от {range.min} до {range.max}
            </>
          )}
        </p>
      </div>

      <dl className="grid gap-x-6 gap-y-2 font-mono text-sm sm:grid-cols-2">
        {dilemma ? (
          <>
            <Fact term="сотрудничество">
              {names[0]} {pct(cooperationShare(transcript.moves[0]))} · {names[1]} {pct(cooperationShare(transcript.moves[1]))}
            </Fact>
            <Fact term="первое предательство">
              {first
                ? `ход ${first.iteration + 1} — ${first.sides.length === 2 ? 'обе стороны одновременно' : names[first.sides[0] - 1]}`
                : 'не было, обе стороны сотрудничали до конца'}
            </Fact>
          </>
        ) : (
          <Fact term={avgLabel}>
            {names[0]} {num(averageMove(transcript.moves[0]))} · {names[1]} {num(averageMove(transcript.moves[1]))}
          </Fact>
        )}
      </dl>

      {totals && <ScoreChart totals={totals} n={n} step={step} names={names} />}

      <IterationTable transcript={transcript} totals={totals} dilemma={dilemma} names={names} step={step} />
    </div>
  );
}

function Fact({ term, children }: { term: string; children: ReactNode }) {
  return (
    <div className="min-w-0">
      <dt className="text-xs text-gray-500">{term}</dt>
      <dd className="text-gray-200">{children}</dd>
    </div>
  );
}

// Пиксельная полоса ходов стороны: клетка на итерацию, после step - притушено.
function MoveStrip({
  moves,
  n,
  step,
  dilemma,
  range,
  onSeek,
  name,
}: {
  moves: number[];
  n: number;
  step: number;
  dilemma: boolean;
  range: { min: number; max: number };
  onSeek: (step: number) => void;
  name: string;
}) {
  // на коротком матче клетки с зазором, на длинном - сплошная полоса
  const w = n <= 200 ? 0.8 : 1;
  const cells = useMemo(
    () =>
      moves.map((m, i) =>
        dilemma ? (
          <rect key={i} x={i} width={w} height={1} className={m === COOPERATE ? 'fill-primary-500' : 'fill-red-500'} />
        ) : (
          <rect
            key={i}
            x={i}
            width={w}
            height={1}
            className="fill-primary-400"
            fillOpacity={0.12 + 0.88 * (range.max === range.min ? 1 : (m - range.min) / (range.max - range.min))}
          />
        )
      ),
    [moves, dilemma, range, w]
  );
  const summary = dilemma
    ? `${name}: сотрудничество ${moves.filter((m) => m === COOPERATE).length}, предательство ${moves.filter((m) => m !== COOPERATE).length}`
    : `${name}: ${moves.length} ходов`;

  return (
    <svg
      viewBox={`0 0 ${n} 1`}
      preserveAspectRatio="none"
      shapeRendering="crispEdges"
      className="block h-4 w-full cursor-pointer"
      role="img"
      aria-label={summary}
      onClick={(e) => {
        const r = e.currentTarget.getBoundingClientRect();
        onSeek(Math.floor(((e.clientX - r.left) / r.width) * n) + 1);
      }}
    >
      {cells}
      {step < n && <rect x={step} width={n - step} height={1} fill="#111827" fillOpacity={0.75} />}
    </svg>
  );
}

// Накопленный счёт обеих сторон; курсор - на step или под указателем.
function ScoreChart({ totals, n, step, names }: { totals: number[][]; n: number; step: number; names: [string, string] }) {
  const [hover, setHover] = useState<number | null>(null);
  const W = 600;
  const H = 180;
  const PAD = { top: 6, right: 4, bottom: 6, left: 4 };
  const max = Math.max(1, ...totals.flat());
  const min = Math.min(0, ...totals.flat());
  const x = (i: number) => PAD.left + (i / Math.max(1, n)) * (W - PAD.left - PAD.right);
  const y = (v: number) => PAD.top + (1 - (v - min) / (max - min)) * (H - PAD.top - PAD.bottom);
  const line = (vs: number[]) => vs.map((v, i) => `${x(i).toFixed(1)},${y(v).toFixed(1)}`).join(' ');
  const at = hover ?? step;
  const value = (s: 0 | 1) => totals[s][Math.min(at, totals[s].length - 1)];

  return (
    <figure>
      <figcaption className="mb-2 flex flex-wrap gap-x-5 gap-y-1 font-mono text-sm text-gray-300">
        <span className="text-gray-500">счёт после {at} из {n}:</span>
        {([0, 1] as const).map((s) => (
          <span key={s} className="inline-flex items-center gap-2">
            <SideMark side={(s + 1) as Side} className="" />
            {names[s]} <span className="tabular-nums text-gray-100">{value(s)}</span>
          </span>
        ))}
      </figcaption>
      {/* подписи оси - вне svg: растянутый viewBox исказил бы текст */}
      <div className="flex gap-2">
        <div aria-hidden="true" className="flex flex-col justify-between py-0.5 text-right font-mono text-xs tabular-nums text-gray-500">
          <span>{max}</span>
          <span>{min}</span>
        </div>
        <svg
          viewBox={`0 0 ${W} ${H}`}
          className="h-44 min-w-0 flex-1"
          preserveAspectRatio="none"
          role="img"
          aria-label={`Накопленный счёт по итерациям: ${names[0]} ${totals[0].at(-1)}, ${names[1]} ${totals[1].at(-1)}`}
          // курсор ведёт только мышь: у касания нет «ухода», и подпись залипала бы
          // поверх шага ползунка и проигрывания
          onPointerLeave={() => setHover(null)}
          onPointerMove={(e) => {
            if (e.pointerType !== 'mouse') return;
            const r = e.currentTarget.getBoundingClientRect();
            const px = ((e.clientX - r.left) / r.width) * W;
            setHover(Math.max(0, Math.min(n, Math.round(((px - PAD.left) / (W - PAD.left - PAD.right)) * n))));
          }}
        >
          <line x1={PAD.left} x2={W - PAD.right} y1={y(min)} y2={y(min)} stroke="#374151" strokeWidth="1" vectorEffect="non-scaling-stroke" />
          <line x1={PAD.left} x2={W - PAD.right} y1={y(max)} y2={y(max)} stroke="#374151" strokeDasharray="4 4" strokeWidth="1" vectorEffect="non-scaling-stroke" />
          {([0, 1] as const).map((s) => (
            <g key={s} stroke={SIDE_COLOR[(s + 1) as Side]} fill="none" strokeWidth="2" strokeLinejoin="round">
              {/* весь матч бледно, сыгранная к шагу часть - ярко */}
              <polyline points={line(totals[s])} strokeOpacity={0.25} vectorEffect="non-scaling-stroke" />
              <polyline points={line(totals[s].slice(0, step + 1))} vectorEffect="non-scaling-stroke" />
            </g>
          ))}
          <line x1={x(at)} x2={x(at)} y1={PAD.top} y2={H - PAD.bottom} stroke="#9ca3af" strokeWidth="1" vectorEffect="non-scaling-stroke" />
        </svg>
      </div>
    </figure>
  );
}

// Таблица итераций: рендерится только в раскрытом виде, на 1000 итераций это тысячи узлов.
function IterationTable({
  transcript,
  totals,
  dilemma,
  names,
  step,
}: {
  transcript: MatchTranscript;
  totals: number[][] | null;
  dilemma: boolean;
  names: [string, string];
  step: number;
}) {
  const [open, setOpen] = useState(false);
  const n = iterationsOf(transcript);
  const move = (v: number | undefined) =>
    v === undefined ? (
      <span className="text-gray-600">—</span>
    ) : dilemma ? (
      <>
        <span aria-hidden="true" className={v === COOPERATE ? 'text-primary-500' : 'text-red-500'}>■ </span>
        {v === COOPERATE ? 'сотр.' : 'пред.'}
      </>
    ) : (
      v
    );

  return (
    <details open={open} onToggle={(e) => setOpen(e.currentTarget.open)}>
      <summary className="cursor-pointer font-mono text-sm text-gray-300 hover:text-gray-100">
        таблица итераций <span className="text-gray-500">[{n}]</span>
      </summary>
      {open && (
        <div tabIndex={0} role="region" aria-label="Таблица итераций" className="mt-3 max-h-96 overflow-auto rounded border border-gray-800">
          <table className="w-full whitespace-nowrap font-mono text-sm tabular-nums">
            <thead className="sticky top-0 bg-gray-900 text-left text-xs text-gray-400">
              <tr>
                <th className="px-3 py-2">№</th>
                {([1, 2] as const).map((s) => (
                  <th key={s} className="max-w-32 truncate px-3 py-2">
                    <SideMark side={s} />
                    {names[s - 1]}
                  </th>
                ))}
                {totals && <th className="px-3 py-2 text-right">очки</th>}
                {totals && <th className="px-3 py-2 text-right">счёт</th>}
              </tr>
            </thead>
            <tbody>
              {Array.from({ length: n }, (_, i) => (
                <tr
                  key={i}
                  aria-current={i === step - 1 ? 'step' : undefined}
                  className={`border-t border-gray-800 ${i === step - 1 ? 'bg-gray-800' : ''}`}
                >
                  <td className="px-3 py-1 text-gray-500">{i + 1}</td>
                  <td className="px-3 py-1 text-gray-200">{move(transcript.moves[0]?.[i])}</td>
                  <td className="px-3 py-1 text-gray-200">{move(transcript.moves[1]?.[i])}</td>
                  {totals && (
                    <td className="px-3 py-1 text-right text-gray-300">
                      {transcript.points![0][i] ?? '—'}:{transcript.points![1][i] ?? '—'}
                    </td>
                  )}
                  {totals && (
                    <td className="px-3 py-1 text-right text-gray-100">
                      {totals[0][i + 1] ?? '—'}:{totals[1][i + 1] ?? '—'}
                    </td>
                  )}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </details>
  );
}

// Торги аукциона: ставки по очереди, очков по ходам нет, только итог.
function AuctionBids({ transcript, names }: { transcript: MatchTranscript; names: [string, string] }) {
  const prize = GAME_PAYOFFS.dollar_auction.prize;
  const turns = auctionTurns(transcript);
  return (
    <div>
      <ol className="space-y-1 font-mono text-sm">
        {turns.map((t, i) => (
          <li key={i} className="flex gap-3">
            <span className="w-8 text-right tabular-nums text-gray-500">{i + 1}.</span>
            <span className="min-w-0 flex-1 truncate text-gray-300">{names[t.side - 1]}</span>
            <span className="tabular-nums text-gray-100">
              {t.bid === 0 ? 'пас' : t.bid}
              {t.bid >= prize && <span className="ml-2 text-amber-400">≥ приза</span>}
            </span>
          </li>
        ))}
      </ol>
      <p className="mt-3 font-mono text-xs text-gray-500">
        <span aria-hidden="true">{'// '}</span>
        первой ставит программа 1, ставка 0 — пас; оба платят свою последнюю ставку, приз {prize} получает только победитель
      </p>
    </div>
  );
}
