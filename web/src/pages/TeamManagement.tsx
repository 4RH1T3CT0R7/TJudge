import { useState, useEffect } from 'react';
import { useParams, useNavigate, Link } from 'react-router-dom';
import api from '../api/client';
import { useAuthStore } from '../store/authStore';
import type { Team, User } from '../types';

interface TeamWithMembers {
  team: Team;
  members: User[];
}

export function TeamManagement() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { user } = useAuthStore();
  const [teamData, setTeamData] = useState<TeamWithMembers | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

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

  useEffect(() => {
    if (id) {
      loadTeamData();
    }
  }, [id]);

  const loadTeamData = async () => {
    if (!id) return;

    setIsLoading(true);
    setError(null);

    try {
      const data = await api.getTeam(id);
      setTeamData(data);
      setNewName(data.team.name);
    } catch (err) {
      setError('Failed to load team data');
      console.error(err);
    } finally {
      setIsLoading(false);
    }
  };

  const handleUpdateName = async () => {
    if (!id || !newName.trim()) return;

    setIsSaving(true);
    try {
      await api.updateTeamName(id, newName.trim());
      setTeamData((prev) =>
        prev ? { ...prev, team: { ...prev.team, name: newName.trim() } } : null
      );
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
      setTimeout(() => setCopied(false), 2000);
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
      navigate(`/tournaments/${teamData?.team.tournament_id}`);
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
      setTeamData((prev) =>
        prev
          ? { ...prev, members: prev.members.filter((m) => m.id !== userId) }
          : null
      );
      setMemberToRemove(null);
    } catch (err) {
      console.error('Failed to remove member:', err);
    }
  };

  if (isLoading) {
    return (
      <div className="text-center py-12">
        <p className="text-gray-500">Loading team...</p>
      </div>
    );
  }

  if (error || !teamData) {
    return (
      <div className="text-center py-12">
        <p className="text-red-500">{error || 'Team not found'}</p>
        <Link to="/tournaments" className="btn btn-secondary mt-4">
          Back to Tournaments
        </Link>
      </div>
    );
  }

  const { team, members } = teamData;
  const isLeader = user?.id === team.leader_id;
  const isMember = members.some((m) => m.id === user?.id);

  return (
    <div className="max-w-3xl mx-auto">
      {/* Breadcrumb */}
      <nav className="mb-4 text-sm">
        <Link to="/tournaments" className="text-gray-500 hover:text-gray-700">
          Tournaments
        </Link>
        <span className="mx-2 text-gray-400">/</span>
        <Link
          to={`/tournaments/${team.tournament_id}`}
          className="text-gray-500 hover:text-gray-700"
        >
          Tournament
        </Link>
        <span className="mx-2 text-gray-400">/</span>
        <span className="text-gray-900">{team.name}</span>
      </nav>

      {/* Team Header */}
      <div className="card mb-6">
        <div className="flex justify-between items-start mb-4">
          {isEditing ? (
            <div className="flex gap-2 flex-1 max-w-md">
              <input
                type="text"
                value={newName}
                onChange={(e) => setNewName(e.target.value)}
                className="input flex-1"
                placeholder="Team name"
              />
              <button
                onClick={handleUpdateName}
                disabled={isSaving || !newName.trim()}
                className="btn btn-primary"
              >
                {isSaving ? 'Saving...' : 'Save'}
              </button>
              <button
                onClick={() => {
                  setIsEditing(false);
                  setNewName(team.name);
                }}
                className="btn btn-secondary"
              >
                Cancel
              </button>
            </div>
          ) : (
            <div>
              <h1 className="text-2xl font-bold">{team.name}</h1>
              {isLeader && (
                <span className="text-xs bg-yellow-100 text-yellow-800 px-2 py-0.5 rounded">
                  You are the leader
                </span>
              )}
            </div>
          )}

          {isLeader && !isEditing && (
            <button onClick={() => setIsEditing(true)} className="btn btn-secondary">
              Edit Name
            </button>
          )}
        </div>

        <div className="text-sm text-gray-500 space-y-1">
          <p>
            Team code: <code className="bg-gray-100 px-2 py-0.5 rounded">{team.code}</code>
          </p>
          <p>Created: {new Date(team.created_at).toLocaleDateString()}</p>
        </div>
      </div>

      {/* Invite Section */}
      {isLeader && (
        <div className="card mb-6">
          <h2 className="text-lg font-semibold mb-4">Invite Members</h2>

          {showInvite && inviteLink ? (
            <div className="space-y-3">
              <div className="flex gap-2">
                <input
                  type="text"
                  value={inviteLink}
                  readOnly
                  className="input flex-1 bg-gray-50"
                />
                <button onClick={handleCopyInvite} className="btn btn-primary">
                  {copied ? 'Copied!' : 'Copy'}
                </button>
              </div>
              <p className="text-sm text-gray-500">
                Invite code: <code className="bg-gray-100 px-2 py-0.5 rounded">{inviteCode}</code>
              </p>
              <p className="text-xs text-gray-400">
                Share this link with others to let them join your team.
              </p>
            </div>
          ) : (
            <button onClick={handleGetInvite} className="btn btn-primary">
              Get Invite Link
            </button>
          )}
        </div>
      )}

      {/* Members List */}
      <div className="card mb-6">
        <h2 className="text-lg font-semibold mb-4">Team Members ({members.length})</h2>

        <div className="divide-y">
          {members.map((member) => (
            <div key={member.id} className="py-3 flex justify-between items-center">
              <div>
                <p className="font-medium">
                  {member.username}
                  {member.id === team.leader_id && (
                    <span className="ml-2 text-xs bg-yellow-100 text-yellow-800 px-2 py-0.5 rounded">
                      Leader
                    </span>
                  )}
                  {member.id === user?.id && (
                    <span className="ml-2 text-xs bg-blue-100 text-blue-800 px-2 py-0.5 rounded">
                      You
                    </span>
                  )}
                </p>
                <p className="text-sm text-gray-500">{member.email}</p>
              </div>

              {isLeader && member.id !== user?.id && (
                <>
                  {memberToRemove === member.id ? (
                    <div className="flex gap-2">
                      <button
                        onClick={() => handleRemoveMember(member.id)}
                        className="btn btn-danger text-sm"
                      >
                        Confirm
                      </button>
                      <button
                        onClick={() => setMemberToRemove(null)}
                        className="btn btn-secondary text-sm"
                      >
                        Cancel
                      </button>
                    </div>
                  ) : (
                    <button
                      onClick={() => setMemberToRemove(member.id)}
                      className="text-red-600 hover:text-red-800 text-sm"
                    >
                      Remove
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
        <div className="card border-red-200">
          <h2 className="text-lg font-semibold mb-4 text-red-600">Danger Zone</h2>

          {confirmLeave ? (
            <div className="space-y-3">
              <p className="text-gray-600">
                {isLeader
                  ? 'You are the team leader. If you leave, leadership will be transferred to another member. If you are the last member, the team will be deleted.'
                  : 'Are you sure you want to leave this team?'}
              </p>
              <div className="flex gap-2">
                <button
                  onClick={handleLeaveTeam}
                  disabled={isLeaving}
                  className="btn btn-danger"
                >
                  {isLeaving ? 'Leaving...' : 'Yes, Leave Team'}
                </button>
                <button
                  onClick={() => setConfirmLeave(false)}
                  className="btn btn-secondary"
                >
                  Cancel
                </button>
              </div>
            </div>
          ) : (
            <button onClick={() => setConfirmLeave(true)} className="btn btn-danger">
              Leave Team
            </button>
          )}
        </div>
      )}
    </div>
  );
}
