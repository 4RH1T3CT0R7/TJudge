import { useEffect, useRef, useState } from 'react';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { useQueryClient } from '@tanstack/react-query';
import api from '../api/client';
import { queryKeys } from '../api/queryKeys';
import { TerminalLoader } from '../components/TerminalLoader';

// Ссылка-приглашение /join/:code: вступление в команду и переход на турнир.
export function JoinTeam() {
  const { code } = useParams<{ code: string }>();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [error, setError] = useState('');
  // StrictMode в dev вызывает эффект дважды, а вступать нужно один раз
  const started = useRef(false);

  useEffect(() => {
    if (!code || started.current) return;
    started.current = true;
    api
      .joinTeamByCode(code)
      .then((team) => {
        queryClient.setQueryData(queryKeys.myTeam(team.tournament_id), team);
        navigate(`/tournaments/${team.tournament_id}`, { replace: true });
      })
      .catch(() => setError('Не удалось вступить: код неверный, команда заполнена, турнир уже начался или вы уже в команде'));
  }, [code, navigate, queryClient]);

  if (!error) {
    return <TerminalLoader />;
  }

  return (
    <div className="text-center py-12">
      <p className="text-red-400">{error}</p>
      <Link to="/tournaments" className="btn btn-secondary mt-4">
        К турнирам
      </Link>
    </div>
  );
}
