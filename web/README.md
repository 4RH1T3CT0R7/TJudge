# TJudge Frontend

Веб-интерфейс турнирной системы TJudge, построенный на React + TypeScript + Tailwind CSS.

## Технологии

| Технология | Версия | Назначение |
|------------|--------|------------|
| React | 18+ | UI фреймворк |
| TypeScript | 5+ | Типизация |
| Vite | 7 | Сборщик |
| Tailwind CSS | 4 | Стилизация |
| React Router | 7 | Маршрутизация |
| Zustand | — | Стейт менеджмент |
| React Query | — | Кэширование запросов |
| Axios | — | HTTP клиент |

## Быстрый старт

### Установка

```bash
# Установка зависимостей
npm install
```

### Режим разработки

```bash
# Запуск dev сервера
npm run dev

# Приложение будет доступно на http://localhost:5173
```

В режиме разработки запросы к API проксируются на `http://localhost:8080`.

### Сборка для продакшена

```bash
# Сборка
npm run build

# Результат в директории dist/
```

**Важно:** После сборки файлы из `dist/` встраиваются в Go binary через `go:embed`.

## Структура проекта

```
web/
├── src/
│   ├── api/               # API клиент
│   │   └── client.ts     # Axios инстанс + методы
│   ├── components/        # React компоненты
│   │   └── layout/       # Layout, Header, Footer
│   ├── hooks/            # Кастомные хуки
│   │   └── useWebSocket.ts  # WebSocket для real-time
│   ├── pages/            # Страницы
│   │   ├── Home.tsx
│   │   ├── Login.tsx
│   │   ├── Register.tsx
│   │   ├── Tournaments.tsx
│   │   ├── TournamentDetail.tsx
│   │   ├── GameDetail.tsx
│   │   ├── TeamManagement.tsx
│   │   └── AdminPanel.tsx
│   ├── store/            # Zustand сторы
│   │   └── authStore.ts  # Аутентификация
│   ├── types/            # TypeScript типы
│   │   └── index.ts
│   ├── App.tsx           # Главный компонент + роутинг
│   ├── main.tsx          # Точка входа
│   └── index.css         # Глобальные стили + Tailwind
├── public/               # Статические файлы
├── index.html           # HTML шаблон
├── package.json
├── tsconfig.json
├── vite.config.ts
└── postcss.config.js
```

## Страницы

| Страница | Путь | Описание |
|----------|------|----------|
| Home | `/` | Главная страница |
| Login | `/login` | Вход |
| Register | `/register` | Регистрация |
| Tournaments | `/tournaments` | Список турниров |
| Tournament Detail | `/tournaments/:id` | Детали турнира (вкладки: Info, Leaderboard, Games, Teams) |
| Game Detail | `/tournaments/:id/games/:gameId` | Правила игры, загрузка программы |
| Team Management | `/teams/:id` | Управление командой |
| Admin Panel | `/admin` | Админ-панель (только для admin) |

## API клиент

API клиент расположен в `src/api/client.ts`:

```typescript
import api from './api/client';

// Аутентификация
await api.login(email, password);
await api.register(username, email, password);
await api.logout();

// Турниры
const tournaments = await api.getTournaments();
const tournament = await api.getTournament(id);
const leaderboard = await api.getLeaderboard(tournamentId);

// Команды
await api.createTeam(tournamentId, name);
await api.joinTeamByCode(code);

// Программы
await api.uploadProgram(formData);
```

## WebSocket

Хук для real-time обновлений таблицы лидеров:

```typescript
import { useWebSocket } from './hooks/useWebSocket';

const { isConnected } = useWebSocket({
  tournamentId: '123',
  onMessage: (message) => {
    if (message.type === 'leaderboard_update') {
      setLeaderboard(message.payload.entries);
    }
  },
});
```

## State Management

Zustand используется для глобального состояния аутентификации:

```typescript
import { useAuthStore } from './store/authStore';

const { user, isAuthenticated, login, logout } = useAuthStore();
```

## Стилизация

Tailwind CSS v4 с кастомной темой:

```css
/* src/index.css */
@import "tailwindcss";

@theme {
  --color-primary-500: #3b82f6;
  --color-primary-600: #2563eb;
  /* ... */
}

@layer components {
  .btn { @apply px-4 py-2 rounded-lg font-medium transition-colors; }
  .btn-primary { @apply bg-primary-600 text-white hover:bg-primary-700; }
  .input { @apply w-full px-3 py-2 border border-gray-300 rounded-lg ...; }
  .card { @apply bg-white rounded-lg shadow-md p-6; }
}
```

## Команды

```bash
npm run dev        # Запуск dev сервера
npm run build      # Сборка для продакшена
npm run lint       # Проверка ESLint
npm run preview    # Превью собранного приложения
```

## Переменные окружения

Создайте `.env.local` для локальной разработки:

```bash
# API URL (по умолчанию проксируется на localhost:8080)
VITE_API_URL=/api/v1
```

## Интеграция с Go

Фронтенд встраивается в Go binary через `go:embed`:

```go
// internal/web/embed.go
//go:embed all:dist
var distFS embed.FS
```

После `npm run build` запустите `go build` для включения фронтенда в бинарник.
