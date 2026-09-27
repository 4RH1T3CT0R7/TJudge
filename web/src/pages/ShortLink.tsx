import { Link, Navigate, useParams } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import api from '../api/client';
import { Spinner } from '../components/ui/Spinner';
import { ErrorState } from '../components/ui/ErrorState';

// Короткая ссылка с QR табло: /t/:code ведёт на таблицу турнира.
export function ShortLink() {
  const { code = '' } = useParams<{ code: string }>();
  const query = useQuery({
    queryKey: ['tournaments', 'code', code],
    queryFn: () => api.getTournamentByCode(code),
  });

  if (query.data) return <Navigate to={`/tournaments/${query.data.id}?tab=leaderboard`} replace />;
  if (query.isPending) {
    return (
      <div className="flex justify-center py-24 text-sm text-gray-400">
        <Spinner>поиск турнира {code}</Spinner>
      </div>
    );
  }
  return (
    <div className="py-12">
      <title>Турнир не найден — TJudge</title>
      <h1 className="sr-only">Короткая ссылка</h1>
      <ErrorState
        message={query.isError ? 'Не удалось найти турнир: сервер недоступен' : `Турнира с кодом ${code.toUpperCase()} нет`}
        onRetry={query.isError ? () => void query.refetch() : undefined}
      >
        <Link to="/tournaments" className="btn btn-secondary">К списку турниров</Link>
      </ErrorState>
    </div>
  );
}
