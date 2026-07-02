import type { InvaderPose } from '../SpaceInvader';
import type { TournamentStatus } from '../../types';

export const statusLabels: Record<TournamentStatus, string> = {
  pending: 'Ожидание',
  active: 'Активный',
  completed: 'Завершён',
};

/** Реакция админ-захватчика: поза + реплика (см. setAdminReaction в AdminPanel). */
export type AdminReactionSetter = (pose: InvaderPose, speech: string | null, duration?: number) => void;

export interface GameFormState {
  name: string;
  display_name: string;
  rules: string;
}

export interface TournamentFormState {
  name: string;
  description: string;
  game_type: string;
  max_team_size: number;
  max_participants: string;
  is_permanent: boolean;
  start_time: string;
  end_time: string;
}
