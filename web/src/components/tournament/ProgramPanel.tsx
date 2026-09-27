import { useEffect, useId, useMemo, useRef, useState, type ReactNode } from 'react';
import { Link } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import api from '../../api/client';
import { queryKeys } from '../../api/queryKeys';
import { useToastStore } from '../../store/toastStore';
import { SpaceInvader } from '../SpaceInvader';
import { StatusLabel } from '../ui/StatusLabel';
import { Spinner } from '../ui/Spinner';
import { TerminalOutput } from '../ui/TerminalOutput';
import { MatchError } from './MatchError';
import { extractErrorMessage } from './helpers';
import { explainCompileError, explainMatchError } from '../../utils/explainError';
import { crashStats, latestVersion, playingVersion, uploadBlockReason } from '../../utils/participant';
import { precheckFile, SUPPORTED_EXTENSIONS } from '../../utils/precheckFile';
import type { MatchRound, Program, Team, Tournament, TournamentGameWithDetails } from '../../types';

interface ProgramPanelProps {
  tournament: Tournament | null;
  gameId: string;
  gameStatus: TournamentGameWithDetails | null;
  /** Статусы всех игр турнира: название игры, в которой идёт раунд. */
  gamesStatus: TournamentGameWithDetails[];
  /** Счётчики раундов турнира: идёт ли где-то раунд. */
  rounds: MatchRound[];
  myTeam: Team;
  /** Все версии программы команды в этой игре. */
  programs: Program[];
}

// Секундомер сборки: тикает, пока running; после остановки держит последнее значение.
function Stopwatch({ since, running }: { since: number; running: boolean }) {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    if (!running) return;
    const id = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(id);
  }, [running]);
  const sec = Math.max(0, Math.floor((now - since) / 1000));
  return <span className="tabular-nums">{String(Math.floor(sec / 60)).padStart(2, '0')}:{String(sec % 60).padStart(2, '0')}</span>;
}

// Строка терминальной сессии: «$ команда … результат».
function SessionLine({ cmd, children }: { cmd: string; children: ReactNode }) {
  return (
    <p className="flex justify-between gap-3">
      <span className="min-w-0 break-all text-gray-400">
        <span aria-hidden="true" className="text-primary-400">$ </span>
        {cmd}
      </span>
      <span className="shrink-0 text-right">{children}</span>
    </p>
  );
}

const Note = ({ tone = 'text-gray-500', children }: { tone?: string; children: ReactNode }) => (
  <p className={`font-mono text-xs ${tone}`}>
    <span aria-hidden="true">{'// '}</span>
    {children}
  </p>
);

async function download(program: Program) {
  try {
    const blob = await api.downloadProgram(program.id);
    const url = window.URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = program.name || `program_v${program.version}`;
    document.body.appendChild(a);
    a.click();
    window.URL.revokeObjectURL(url);
    document.body.removeChild(a);
  } catch (err) {
    console.error('Download failed:', err);
    useToastStore.getState().addToast('Не удалось скачать программу', 'error');
  }
}

// Колонка «Ваша программа»: честная карточка текущей версии (загрузка → сборка →
// самопроверка), какая версия реально играет, здоровье в матчах и зона загрузки.
export function ProgramPanel({ tournament, gameId, gameStatus, gamesStatus, rounds, myTeam, programs }: ProgramPanelProps) {
  const queryClient = useQueryClient();
  const tournamentId = tournament?.id ?? '';
  const [isUploading, setIsUploading] = useState(false);
  const [uploadError, setUploadError] = useState<string | null>(null);
  const [warnings, setWarnings] = useState<string[]>([]);
  // своя загрузка в этой сессии: от неё идёт секундомер и тосты об итоге сборки
  const [uploaded, setUploaded] = useState<{ id: string; at: number } | null>(null);
  const [isDragging, setIsDragging] = useState(false);
  const [uploadShake, setUploadShake] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const dropZoneRef = useRef<HTMLDivElement>(null);
  const reasonId = useId();

  const current = latestVersion(programs);
  const playing = playingVersion(programs, myTeam.is_disqualified);
  const tracked = uploaded ? programs.find((p) => p.id === uploaded.id) : undefined;

  const blockReason = uploadBlockReason(tournament, myTeam, gameStatus, gamesStatus, rounds);
  const canUpload = !blockReason && !isUploading;

  // Здоровье версии, которая играет: сколько её матчей упало по её вине.
  // Ключ под матчами игры: обновляется вместе с ними по событиям матчей.
  const healthQuery = useQuery({
    queryKey: [...queryKeys.gameMatches(tournamentId, gameId), 'program', playing?.id ?? ''] as const,
    queryFn: () => api.getProgramMatches(tournamentId, playing!.id),
    enabled: !!tournamentId && !!playing,
    staleTime: 30_000,
  });
  const health = useMemo(() => {
    if (!playing) return null;
    const list = healthQuery.data ?? [];
    const stats = crashStats(list, playing.id);
    // ponytail: потолок бэкенда 100 матчей (до 51 команды), больше - счётчики с сервера
    return stats && { ...stats, capped: list.length >= 100 };
  }, [healthQuery.data, playing]);

  // Тосты об итоге своей сборки и самопроверки: карточка на телефоне ниже вкладок.
  const prevRef = useRef<{ id?: string; status?: string; check?: string | null }>({});
  useEffect(() => {
    if (!tracked) return;
    const prev = prevRef.current;
    prevRef.current = { id: tracked.id, status: tracked.status, check: tracked.check_status };
    if (prev.id !== tracked.id) return;
    const { addToast } = useToastStore.getState();
    const v = `v${tracked.version}`;
    if (prev.status === 'compiling' && tracked.status === 'ready') addToast(`${v} собрана`, 'success');
    if (prev.status === 'compiling' && tracked.status === 'failed') {
      addToast(`${v} не собралась — ${explainCompileError(tracked.error_message ?? '') ?? 'смотрите вывод компилятора'}`, 'error');
    }
    if (prev.check !== 'failed' && tracked.check_status === 'failed') {
      const e = explainMatchError({ status: 'failed', error_code: 1, winner: 2, error_message: tracked.check_message ?? '' }, 1);
      // «Ваша программа …» после тире - со строчной
      const verdict = e ? e.verdict[0].toLowerCase() + e.verdict.slice(1) : 'программа упала';
      addToast(`${v}: самопроверка не пройдена — ${verdict}`, 'info');
    }
  }, [tracked]);

  const processFile = async (file: File) => {
    setUploadError(null);
    setWarnings([]);
    const fail = (message: string) => {
      setUploadError(message);
      setUploadShake(true);
      setTimeout(() => setUploadShake(false), 100);
    };
    if (!tournamentId) return fail('Не удалось загрузить: нет данных турнира');

    const check = await precheckFile(file);
    if (check.error) return fail(`${file.name}: ${check.error}`);
    setWarnings(check.warnings);

    setIsUploading(true);
    const startedAt = Date.now();
    try {
      const formData = new FormData();
      formData.append('file', file);
      formData.append('team_id', myTeam.id);
      formData.append('tournament_id', tournamentId);
      formData.append('game_id', gameId);
      formData.append('name', file.name);

      const program = await api.uploadProgram(formData);
      setUploaded({ id: program.id, at: startedAt });
      // новая версия сразу в карточке, не дожидаясь перечитывания списка
      queryClient.setQueryData<Program[]>(queryKeys.programVersions(myTeam.id, gameId), (old) => [
        program,
        ...(old ?? []).filter((p) => p.id !== program.id),
      ]);
      void queryClient.invalidateQueries({ queryKey: queryKeys.programs });
    } catch (err: unknown) {
      console.error('Upload failed:', err);
      fail(extractErrorMessage(err, 'Не удалось загрузить программу'));
    } finally {
      setIsUploading(false);
      if (fileInputRef.current) fileInputRef.current.value = '';
    }
  };

  const handleDragEnter = (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    if (canUpload) setIsDragging(true);
  };
  const handleDragLeave = (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    if (dropZoneRef.current && !dropZoneRef.current.contains(e.relatedTarget as Node)) setIsDragging(false);
  };
  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setIsDragging(false);
    if (blockReason) return setUploadError(`Загрузка закрыта: ${blockReason}`);
    const file = e.dataTransfer.files?.[0];
    if (file && !isUploading) void processFile(file);
  };

  // реплика маскота следует за состоянием своей загрузки
  const bubble = isUploading
    ? '// загружаю...'
    : isDragging
      ? '// давай сюда!'
      : tracked?.status === 'compiling'
        ? '// компилирую...'
        : tracked?.status === 'ready'
          ? '{ собрано: true }'
          : tracked?.status === 'failed'
            ? '// не собралось'
            : '// жду код...';

  const checkError =
    current?.status === 'ready' && current.check_status === 'failed'
      ? { status: 'failed' as const, error_code: 1, winner: 2, error_message: current.check_message ?? '' }
      : null;

  return (
    <div className="card">
      <h2 className="text-lg font-semibold mb-4 text-gray-100">Ваша программа</h2>

      {current && (
        <div className="mb-4 space-y-3">
          {/* Терминальная сессия текущей версии: upload → compile → check */}
          <section aria-label={`Версия ${current.version}`} className="rounded border border-gray-800 bg-gray-950 p-3 font-mono text-xs leading-relaxed">
            <div className="mb-2 flex items-start justify-between gap-2">
              <p className="min-w-0 break-all font-sans text-sm font-medium text-gray-100">{current.name}</p>
              <span className="shrink-0 text-gray-400">v{current.version}</span>
            </div>
            <SessionLine cmd="upload">
              <span className="text-green-400">ok</span>
            </SessionLine>
            <SessionLine cmd={`compile · ${current.language}`}>
              {current.status === 'compiling' || current.id === uploaded?.id ? (
                <span className={current.status === 'compiling' ? 'text-yellow-300' : 'text-gray-400'}>
                  {current.status === 'compiling' && <span aria-hidden="true">◔ </span>}
                  <Stopwatch since={current.id === uploaded?.id ? uploaded.at : Date.parse(current.created_at)} running={current.status === 'compiling'} />
                  {current.status !== 'compiling' && ' '}
                </span>
              ) : null}
              {current.status === 'ready' && <span className="text-green-400">ok</span>}
              {current.status === 'failed' && <span className="text-red-400">✕ ошибка</span>}
            </SessionLine>
            {current.status === 'ready' && current.check_status && (
              <SessionLine cmd="check · эталон">
                {current.check_status === 'pending' && <Spinner className="text-gray-400">идёт</Spinner>}
                {current.check_status === 'ok' && <span className="text-green-400">ok</span>}
                {current.check_status === 'failed' && <span className="text-amber-300">⚠ упала</span>}
              </SessionLine>
            )}
            <p role="status" className="mt-2 flex flex-wrap items-center gap-2 border-t border-gray-800 pt-2">
              <StatusLabel entity="program" status={current.status} />
              {playing?.id === current.id && <span className="text-primary-300"><span aria-hidden="true">▶ </span>в игре</span>}
            </p>
          </section>

          {current.status === 'failed' && current.error_message && (
            <>
              {explainCompileError(current.error_message) && <Note tone="text-amber-300">{explainCompileError(current.error_message)}</Note>}
              <TerminalOutput label={`компиляция · ${current.language}`} text={current.error_message} />
            </>
          )}

          {current.check_status === 'ok' && current.status === 'ready' && current.check_message && (
            <Note>самопроверка: {current.check_message}</Note>
          )}

          {/* Какая версия реально играет */}
          {playing && playing.id !== current.id && (
            <Note tone="text-amber-300">
              {current.status === 'failed'
                ? `v${current.version} не собралась — в матчах играет v${playing.version}`
                : `v${current.version} собирается — пока в матчах играет v${playing.version}`}
            </Note>
          )}
          {!playing && current.status !== 'compiling' && (
            <Note tone="text-amber-300">
              {myTeam.is_disqualified ? 'команда дисквалифицирована: в матчах не участвует' : 'в матчах команда не участвует: нет собранной версии'}
            </Note>
          )}

          {checkError && (
            <div className="rounded border border-amber-700/60 bg-amber-900/20 p-3 space-y-2">
              <p className="text-sm font-medium text-amber-300">
                <span aria-hidden="true">⚠ </span>
                Самопроверка против эталонного бота не пройдена
              </p>
              <MatchError match={checkError} mySide={1} />
              <Note>это только предупреждение: в раундах программа играет</Note>
            </div>
          )}

          {health && (
            <div className="rounded border border-amber-700/60 bg-amber-900/20 p-3 space-y-2">
              <p className="text-sm font-medium text-amber-300">
                <span aria-hidden="true">⚠ </span>
                {`v${playing!.version} собрана, но падает в ${health.crashed.length} из ${health.played}${health.capped ? '+' : ''} матчей`}
              </p>
              <MatchError match={health.crashed[0]} mySide={health.side} brief />
              {/* список - все упавшие матчи команды: и где упал соперник, и старые версии */}
              <Link
                to={{ search: '?tab=matches&status=failed' }}
                state={{ focus: 'matches' }}
                className="inline-block text-sm text-primary-400 hover:underline"
              >
                Все матчи команды с ошибкой →
              </Link>
            </div>
          )}

          <button onClick={() => void download(current)} className="btn btn-secondary w-full">
            Скачать программу
          </button>
        </div>
      )}

      {/* Upload Form with Drag & Drop */}
      <div className="space-y-3">
        <div className="flex justify-center pt-6">
          <SpaceInvader
            size="sm"
            speechBubble={bubble}
            shake={uploadShake || tracked?.status === 'failed'}
            jump={tracked?.status === 'ready'}
            eyeOverride={isDragging ? 'wide' : null}
          />
        </div>

        <input
          type="file"
          ref={fileInputRef}
          onChange={(e) => {
            const file = e.target.files?.[0];
            if (file) void processFile(file);
          }}
          className="hidden"
          accept={SUPPORTED_EXTENSIONS.join(',')}
          aria-label="Загрузить файл программы"
        />

        {/* Зона загрузки: закрытая сразу объясняет почему, до выбора файла */}
        <div
          ref={dropZoneRef}
          role="button"
          tabIndex={0}
          aria-label="Загрузить файл программы"
          aria-disabled={!canUpload || undefined}
          aria-describedby={blockReason ? reasonId : undefined}
          onKeyDown={(e) => {
            if (canUpload && (e.key === 'Enter' || e.key === ' ')) {
              e.preventDefault();
              fileInputRef.current?.click();
            }
          }}
          onDragEnter={handleDragEnter}
          onDragLeave={handleDragLeave}
          onDragOver={(e) => {
            e.preventDefault();
            e.stopPropagation();
          }}
          onDrop={handleDrop}
          onClick={canUpload ? () => fileInputRef.current?.click() : undefined}
          className={`relative border-2 border-dashed rounded-lg p-6 text-center transition-all ${
            !canUpload ? 'cursor-not-allowed border-line' : isDragging
              ? 'cursor-pointer border-primary-500 bg-primary-900/20'
              : 'cursor-pointer border-line hover:border-primary-500 hover:bg-gray-800/50'
          }`}
        >
          {blockReason ? (
            <div className="flex flex-col items-center gap-2">
              <p className="text-sm font-medium text-gray-300">Загрузка закрыта</p>
              <p id={reasonId} className="font-mono text-xs text-gray-400">
                <span aria-hidden="true">{'// '}</span>
                {blockReason}
              </p>
            </div>
          ) : isDragging ? (
            <p className="text-sm font-medium text-primary-400">Отпустите файл для загрузки</p>
          ) : (
            <div className="flex flex-col items-center gap-2">
              <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" aria-hidden="true" className="w-10 h-10 text-gray-500">
                <path strokeLinecap="round" strokeLinejoin="round" d="M19.5 14.25v-2.625a3.375 3.375 0 0 0-3.375-3.375h-1.5A1.125 1.125 0 0 1 13.5 7.125v-1.5a3.375 3.375 0 0 0-3.375-3.375H8.25m6.75 12-3-3m0 0-3 3m3-3v6m-1.5-15H5.625c-.621 0-1.125.504-1.125 1.125v17.25c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 0 0-9-9Z" />
              </svg>
              <p className="text-sm font-medium text-gray-300">{isUploading ? 'Загрузка...' : 'Перетащите файл сюда'}</p>
              {!isUploading && (
                <p className="text-xs text-gray-400">
                  или <span className="text-primary-400 underline">выберите файл</span>
                </p>
              )}
            </div>
          )}

          {isUploading && (
            <div className="absolute inset-0 bg-gray-900/50 rounded-lg flex items-center justify-center">
              <Spinner className="text-lg" />
            </div>
          )}
        </div>

        {warnings.length > 0 && (
          <div role="status" className="rounded border border-amber-700/60 bg-amber-900/20 p-2 space-y-1">
            {warnings.map((w) => (
              <Note key={w} tone="text-amber-300">{w}</Note>
            ))}
          </div>
        )}

        {uploadError && (
          <p role="alert" className="rounded border border-red-700 bg-red-900/30 p-2 text-sm text-red-300">
            <span aria-hidden="true" className="font-mono">✕ </span>
            {uploadError}
          </p>
        )}

        <p className="text-xs text-gray-400 text-center">
          Поддерживаемые форматы: {SUPPORTED_EXTENSIONS.join(', ')}
        </p>
      </div>

      {/* Previous Versions */}
      {programs.length > 1 && (
        <div className="mt-6">
          <h3 className="font-medium mb-2 text-gray-100">Предыдущие версии</h3>
          <div className="space-y-2">
            {programs
              .filter((p) => p.id !== current?.id)
              .sort((a, b) => b.version - a.version)
              .map((program) => (
                <div key={program.id} className="flex justify-between items-start gap-2 text-sm p-2 bg-gray-800 rounded">
                  <div className="flex min-w-0 flex-1 flex-col">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="text-gray-100">v{program.version}</span>
                      <StatusLabel entity="program" status={program.status} />
                      {playing?.id === program.id && (
                        <span className="font-mono text-xs text-primary-300"><span aria-hidden="true">▶ </span>в игре</span>
                      )}
                    </div>
                    <span className="text-xs text-gray-400">{new Date(program.created_at).toLocaleDateString('ru-RU')}</span>
                    {program.status === 'failed' && program.error_message && (
                      <details className="mt-1">
                        <summary className="w-fit cursor-pointer font-mono text-xs text-gray-400">вывод компилятора</summary>
                        <div className="mt-1 space-y-1">
                          {explainCompileError(program.error_message) && (
                            <Note tone="text-amber-300">{explainCompileError(program.error_message)}</Note>
                          )}
                          <TerminalOutput label={`компиляция · ${program.language}`} text={program.error_message} maxHeight="max-h-48" />
                        </div>
                      </details>
                    )}
                  </div>
                  <button
                    onClick={() => void download(program)}
                    className="text-primary-400 hover:text-primary-300 text-xs font-medium"
                    aria-label={`Скачать v${program.version}`}
                  >
                    Скачать
                  </button>
                </div>
              ))}
          </div>
        </div>
      )}
    </div>
  );
}
