import { useState, useEffect } from 'react';
import { useParams, useNavigate, Link } from 'react-router-dom';
import { useQueryClient } from '@tanstack/react-query';
import api from '../api/client';
import { queryKeys } from '../api/queryKeys';
import { useTeam } from '../hooks/queries';
import { useAuthStore } from '../store/authStore';
import { SpaceInvader } from '../components/SpaceInvader';
import { TerminalLoader } from '../components/TerminalLoader';
import { useDelayedLoading } from '../hooks/useDelayedLoading';

export function TeamManagement() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { user } = useAuthStore();
  const queryClient = useQueryClient();
  const { data: teamData, isPending, isError } = useTeam(id ?? '');
  const showLoading = useDelayedLoading(isPending);

  // Edit state
  const [isEditing, setIsEditing] = useState(false);
  const [newName, setNewName] = useState('');
  const [isSaving, setIsSaving] = useState(false);

  // Invite link state
  const [inviteLink, setInviteLink] = useState<string | null>(null);
  const [inviteCode, setInviteCode] = useState<string | null>(null);
  const [showInvite, setShowInvite] = useState(false);
  const [copied, setCopied] = useState(false);

  // Actions state
  const [isLeaving, setIsLeaving] = useState(false);
  const [confirmLeave, setConfirmLeave] = useState(false);
  const [memberToRemove, setMemberToRemove] = useState<string | null>(null);

  // Invader state
  const [invaderSpeech, setInvaderSpeech] = useState<string | null>(null);

  // Set initial invader speech based on role (no pose change to avoid layout shift)
  useEffect(() => {
    if (!teamData || !user) return;
    const isLeaderRole = user.id === teamData.leader_id;
    const speech = isLeaderRole ? '// капитан на мостике' : '// в строю';
    setInvaderSpeech(speech);
    const timer = setTimeout(() => setInvaderSpeech(null), 2500);
    return () => clearTimeout(timer);
  }, [teamData, user]);

  // React to confirmLeave (no pose change)
  useEffect(() => {
    if (confirmLeave) {
      setInvaderSpeech('// не уходи!');
    } else {
      setInvaderSpeech(null);
    }
  }, [confirmLeave]);

  const handleUpdateName = async () => {
    if (!id || !newName.trim()) return;

    setIsSaving(true);
    try {
      await api.updateTeamName(id, newName.trim());
      await queryClient.invalidateQueries({ queryKey: queryKeys.team(id) });
      setIsEditing(false);
    } catch (err) {
      console.error('Failed to update team name:', err);
    } finally {
      setIsSaving(false);
    }
  };

  const handleGetInvite = async () => {
    if (!id) return;

    try {
      const { code, link } = await api.getInviteLink(id);
      setInviteCode(code);
      setInviteLink(link);
      setShowInvite(true);
    } catch (err) {
      console.error('Failed to get invite link:', err);
    }
  };

  const handleCopyInvite = async () => {
    if (!inviteLink) return;

    try {
      await navigator.clipboard.writeText(inviteLink);
      setCopied(true);
      setInvaderSpeech('// скопировано!');
      setTimeout(() => { setCopied(false); setInvaderSpeech(null); }, 2000);
    } catch {
      // Fallback for older browsers
      const textarea = document.createElement('textarea');
      textarea.value = inviteLink;
      document.body.appendChild(textarea);
      textarea.select();
      document.execCommand('copy');
      document.body.removeChild(textarea);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  };

  const handleLeaveTeam = async () => {
    if (!id) return;

    setIsLeaving(true);
    try {
      await api.leaveTeam(id);
      queryClient.invalidateQueries({ queryKey: queryKeys.team(id) });
      if (teamData) {
        queryClient.invalidateQueries({ queryKey: queryKeys.myTeam(teamData.tournament_id) });
      }
      navigate(`/tournaments/${teamData?.tournament_id}`);
    } catch (err) {
      console.error('Failed to leave team:', err);
    } finally {
      setIsLeaving(false);
      setConfirmLeave(false);
    }
  };

  const handleRemoveMember = async (userId: string) => {
    if (!id) return;

    try {
      await api.removeMember(id, userId);
      await queryClient.invalidateQueries({ queryKey: queryKeys.team(id) });
      setMemberToRemove(null);
    } catch (err) {
      console.error('Failed to remove member:', err);
    }
  };

  if (showLoading) {
    return <TerminalLoader />;
  }

  if (isPending) {
    return null;
  }

  if (isError || !teamData) {
    return (
      <div className="text-center py-12">
        <div className="flex justify-center mb-4">
          <SpaceInvader size="sm" controlledPose="cry" speechBubble="// не найдено" eyeOverride="sad" />
        </div>
        <p className="text-red-400">{isError ? 'Не удалось загрузить данные команды' : 'Команда не найдена'}</p>
        <Link to="/tournaments" className="btn btn-secondary mt-4">
          Назад к турнирам
        </Link>
      </div>
    );
  }

  const { members } = teamData;
  const isLeader = user?.id === teamData.leader_id;
  const isMember = members.some((m) => m.id === user?.id);

  return (
    <div className="max-w-4xl mx-auto">
      {/* Breadcrumb */}
      <nav className="mb-4 text-sm">
        <Link to="/tournaments" className="text-gray-400 hover:text-gray-300">
          Турниры
        </Link>
        <span className="mx-2 text-gray-600">/</span>
        <Link
          to={`/tournaments/${teamData.tournament_id}`}
          className="text-gray-400 hover:text-gray-300"
        >
          Турнир
        </Link>
        <span className="mx-2 text-gray-600">/</span>
        <span className="text-gray-200">{teamData.name}</span>
      </nav>

      {/* Team Header */}
      <div className="card mb-6">
        {isEditing ? (
          <div className="flex gap-2 flex-1 max-w-md mb-4">
            <input
              type="text"
              value={newName}
              onChange={(e) => setNewName(e.target.value)}
              className="input flex-1"
              placeholder="Название команды"
            />
            <button
              onClick={handleUpdateName}
              disabled={isSaving || !newName.trim()}
              className="btn btn-primary"
            >
              {isSaving ? 'Сохранение...' : 'Сохранить'}
            </button>
            <button
              onClick={() => {
                setIsEditing(false);
                setNewName(teamData.name);
              }}
              className="btn btn-secondary"
            >
              Отмена
            </button>
          </div>
        ) : (
          <>
            <h1 className="text-2xl font-bold text-gray-100 text-center">{teamData.name}</h1>
            {isLeader && (
              <div className="flex justify-center mt-1 mb-1">
                <span className="text-xs bg-primary-900/50 text-primary-300 px-2 py-0.5 rounded">
                  Вы капитан
                </span>
              </div>
            )}
            {isLeader && (
              <div className="flex justify-center mt-5 mb-4">
                <button onClick={() => { setNewName(teamData.name); setIsEditing(true); }} className="btn btn-secondary">
                  Изменить название
                </button>
              </div>
            )}
            <div className="flex justify-center pt-10 pb-4">
              <SpaceInvader
                size="sm"
                controlledPose="idle"
                speechBubble={invaderSpeech}
              />
            </div>
          </>
        )}

        <div className="text-sm text-gray-400 space-y-1">
          <p>
            Код команды: <code className="bg-gray-800 text-primary-300 px-2 py-0.5 rounded font-mono">{teamData.code}</code>
          </p>
          <p>Создана: {new Date(teamData.created_at).toLocaleDateString('ru-RU')}</p>
        </div>
      </div>

      {/* Invite Section */}
      {isLeader && (
        <div className="card mb-6">
          <h2 className="text-lg font-semibold mb-4 text-gray-100">Пригласить участников</h2>

          {showInvite && inviteLink ? (
            <div className="space-y-3">
              <div className="flex gap-2">
                <input
                  type="text"
                  value={inviteLink}
                  readOnly
                  className="input flex-1 bg-gray-800"
                />
                <button onClick={handleCopyInvite} className="btn btn-primary">
                  {copied ? 'Скопировано!' : 'Копировать'}
                </button>
              </div>
              <p className="text-sm text-gray-400">
                Код приглашения: <code className="bg-gray-800 text-primary-300 px-2 py-0.5 rounded font-mono">{inviteCode}</code>
              </p>
              <p className="text-xs text-gray-500">
                Отправьте эту ссылку другим участникам для вступления в команду.
              </p>
            </div>
          ) : (
            <button onClick={handleGetInvite} className="btn btn-primary">
              Получить ссылку приглашения
            </button>
          )}
        </div>
      )}

      {/* Members List */}
      <div className="card mb-6">
        <h2 className="text-lg font-semibold mb-4 text-gray-100">Участники команды ({members.length})</h2>

        <div className="divide-y divide-gray-800">
          {members.map((member) => (
            <div key={member.id} className="py-3 flex justify-between items-center">
              <div>
                <p className="font-medium text-gray-200">
                  {member.username}
                  {member.id === teamData.leader_id && (
                    <span className="ml-2 text-xs bg-primary-900/50 text-primary-300 px-2 py-0.5 rounded">
                      Капитан
                    </span>
                  )}
                  {member.id === user?.id && (
                    <span className="ml-2 text-xs bg-blue-900/50 text-blue-300 px-2 py-0.5 rounded">
                      Вы
                    </span>
                  )}
                </p>
                <p className="text-sm text-gray-400">{member.email}</p>
              </div>

              {isLeader && member.id !== user?.id && (
                <>
                  {memberToRemove === member.id ? (
                    <div className="flex gap-2">
                      <button
                        onClick={() => handleRemoveMember(member.id)}
                        className="btn btn-danger text-sm"
                      >
                        Подтвердить
                      </button>
                      <button
                        onClick={() => setMemberToRemove(null)}
                        className="btn btn-secondary text-sm"
                      >
                        Отмена
                      </button>
                    </div>
                  ) : (
                    <button
                      onClick={() => setMemberToRemove(member.id)}
                      className="text-red-400 hover:text-red-300 text-sm"
                    >
                      Удалить
                    </button>
                  )}
                </>
              )}
            </div>
          ))}
        </div>
      </div>

      {/* Leave Team */}
      {isMember && (
        <div className="card border-red-900/50">
          <h2 className="text-lg font-semibold mb-4 text-red-400">Опасная зона</h2>

          {confirmLeave ? (
            <div className="space-y-3">
              <p className="text-gray-400">
                {isLeader
                  ? 'Вы капитан команды. Если вы покинете команду, капитанство перейдёт к другому участнику. Если вы последний участник, команда будет удалена.'
                  : 'Вы уверены, что хотите покинуть эту команду?'}
              </p>
              <div className="flex gap-2">
                <button
                  onClick={handleLeaveTeam}
                  disabled={isLeaving}
                  className="btn btn-danger"
                >
                  {isLeaving ? 'Выход...' : 'Да, покинуть команду'}
                </button>
                <button
                  onClick={() => setConfirmLeave(false)}
                  className="btn btn-secondary"
                >
                  Отмена
                </button>
              </div>
            </div>
          ) : (
            <button onClick={() => setConfirmLeave(true)} className="btn btn-danger">
              Покинуть команду
            </button>
          )}
        </div>
      )}
    </div>
  );
}
