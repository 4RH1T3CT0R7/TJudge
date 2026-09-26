import { useState } from 'react';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { useQueryClient } from '@tanstack/react-query';
import api from '../api/client';
import { queryKeys } from '../api/queryKeys';

// Ссылка-приглашение /join/:code: вступление по кнопке и переход на турнир.
// Сразу при открытии не вступает: чужая ссылка не должна молча переводить в команду.
export function JoinTeam() {
  const { code } = useParams<{ code: string }>();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [error, setError] = useState('');
  const [joining, setJoining] = useState(false);

  const join = async () => {
    if (!code) return;
    setJoining(true);
    setError('');
    try {
      const team = await api.joinTeamByCode(code);
      queryClient.setQueryData(queryKeys.myTeam(team.tournament_id), team);
      navigate(`/tournaments/${team.tournament_id}`, { replace: true });
    } catch {
      setError('Не удалось вступить: код неверный, команда заполнена, турнир уже начался или вы уже в команде');
      setJoining(false);
    }
  };

  return (
    <div className="text-center py-12">
      <button type="button" className="btn btn-primary" onClick={join} disabled={joining}>
        {joining ? 'Вступление...' : `Вступить в команду по коду ${code}`}
      </button>
      {error && <p className="text-red-400 mt-4">{error}</p>}
      <div className="mt-4">
        <Link to="/tournaments" className="btn btn-secondary">
          К турнирам
        </Link>
      </div>
    </div>
  );
}
