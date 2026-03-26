// Game-specific icons and colors configuration (see https://github.com/bmstu-itstech/tjudge-cli)

export interface GameConfig {
  icon: string;
  color: string;
  bgClass: string;
  textClass: string;
  borderClass: string;
  cardBorderClass: string;
  gradientClass: string;
}

const gameConfig: Record<string, GameConfig> = {
  dilemma: {
    icon: '🤝',
    color: 'purple',
    bgClass: 'bg-primary-500',
    textClass: 'text-primary-400',
    borderClass: 'border-primary-500',
    cardBorderClass: 'border-primary-800 hover:border-primary-600',
    gradientClass: 'from-primary-500 to-primary-600',
  },
  tug_of_war: {
    icon: '🪢',
    color: 'green',
    bgClass: 'bg-green-500',
    textClass: 'text-green-400',
    borderClass: 'border-green-500',
    cardBorderClass: 'border-green-800 hover:border-green-600',
    gradientClass: 'from-green-500 to-green-600',
  },
  travelers_dilemma: {
    icon: '🧳',
    color: 'blue',
    bgClass: 'bg-blue-500',
    textClass: 'text-blue-400',
    borderClass: 'border-blue-500',
    cardBorderClass: 'border-blue-800 hover:border-blue-600',
    gradientClass: 'from-blue-500 to-blue-600',
  },
  public_goods: {
    icon: '🏛️',
    color: 'orange',
    bgClass: 'bg-orange-500',
    textClass: 'text-orange-400',
    borderClass: 'border-orange-500',
    cardBorderClass: 'border-orange-800 hover:border-orange-600',
    gradientClass: 'from-orange-500 to-orange-600',
  },
  dollar_auction: {
    icon: '💰',
    color: 'yellow',
    bgClass: 'bg-yellow-500',
    textClass: 'text-yellow-400',
    borderClass: 'border-yellow-500',
    cardBorderClass: 'border-yellow-800 hover:border-yellow-600',
    gradientClass: 'from-yellow-500 to-yellow-600',
  },
};

export const defaultGameConfig: GameConfig = {
  icon: '🎮',
  color: 'gray',
  bgClass: 'bg-primary-600',
  textClass: 'text-primary-400',
  borderClass: 'border-primary-500',
  cardBorderClass: 'border-gray-700 hover:border-gray-600',
  gradientClass: 'from-primary-500 to-primary-600',
};

export const getGameConfig = (gameName: string): GameConfig =>
  gameConfig[gameName] || defaultGameConfig;
