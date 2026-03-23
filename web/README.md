# TJudge Frontend

Веб-интерфейс турнирной системы TJudge, построенный на React + TypeScript + Tailwind CSS.

## Технологии

| Технология | Версия | Назначение |
|------------|--------|------------|
| React | 19 | UI фреймворк |
| TypeScript | 5.9 | Типизация |
| Vite | 7 | Сборщик |
| Tailwind CSS | 4 | Стилизация |
| React Router | 7 | Маршрутизация |
| Zustand | 5 | Стейт менеджмент |
| Axios | 1.13 | HTTP клиент |
| Motion (Framer Motion) | 12 | Анимации и переходы |
| React Markdown + remark-gfm | 10 / 4 | Рендеринг Markdown (правила игр) |
| Three.js | 0.182 | 3D-эффекты |

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

# Результат в директории ../internal/web/dist/
```

**Важно:** После сборки файлы из `dist/` встраиваются в Go binary через `go:embed`.

## Структура проекта

```
web/
├── src/
│   ├── api/                    # API клиент
│   │   └── client.ts           # Axios инстанс + методы
│   ├── components/             # React компоненты
│   │   ├── layout/
│   │   │   └── Layout.tsx      # Общий layout (Header, Footer, навигация)
│   │   ├── motion/             # Анимации
│   │   │   ├── AnimatedOutlet.tsx   # Анимированные переходы страниц
│   │   │   ├── InvaderPresence.tsx  # Анимации появления инвейдера
│   │   │   ├── StaggerList.tsx      # Последовательная анимация списков
│   │   │   └── invaderVariants.ts   # Конфигурация вариантов анимаций
│   │   ├── quest/              # Система квестов
│   │   │   ├── QuestTerminal.tsx    # Терминал квеста
│   │   │   ├── QuestInvader.tsx     # Инвейдер в квесте
│   │   │   ├── QuestEnvironment.tsx # Окружение квеста
│   │   │   └── MiniGames.tsx        # Мини-игры (Pong, Typing, Strategy)
│   │   ├── SpaceInvader.tsx    # Пиксельный маскот (CSS pixel art)
│   │   ├── TerminalQuest.tsx   # Терминальный квест
│   │   ├── TerminalLoader.tsx  # Терминальный лоадер
│   │   ├── TerminalTypewriter.tsx  # Эффект печатающегося текста
│   │   ├── CinematicOverlay.tsx # Кинематографические переходы
│   │   ├── PixelGrid.tsx       # Пиксельная сетка
│   │   ├── ErrorBoundary.tsx   # Обработка ошибок React
│   │   └── PageLoader.tsx      # Лоадер для lazy-загрузки страниц
│   ├── context/
│   │   └── InvaderContext.tsx   # Глобальное состояние маскота-инвейдера
│   ├── hooks/                  # Кастомные хуки
│   │   ├── useWebSocket.ts     # WebSocket для real-time обновлений
│   │   ├── useQuestState.ts    # Состояние и логика квеста (5 уровней)
│   │   ├── useEasterEggs.ts    # Пасхалки (Konami Code, God Mode и др.)
│   │   ├── useDarkMode.ts      # Тёмная тема
│   │   ├── useDelayedLoading.ts    # Отложенный показ лоадера
│   │   └── useEscapeKey.ts     # Обработка клавиши Escape
│   ├── pages/                  # Страницы
│   │   ├── Home.tsx
│   │   ├── Login.tsx
│   │   ├── Profile.tsx
│   │   ├── Tournaments.tsx
│   │   ├── TournamentDetail.tsx
│   │   ├── GameDetail.tsx
│   │   ├── Games.tsx
│   │   ├── GameView.tsx
│   │   ├── TeamManagement.tsx
│   │   ├── AdminPanel.tsx
│   │   └── NotFound.tsx
│   ├── store/                  # Zustand сторы
│   │   └── authStore.ts        # Аутентификация
│   ├── types/                  # TypeScript типы
│   │   └── index.ts
│   ├── utils/                  # Утилиты
│   │   ├── commandParser.ts    # Парсер команд терминала квеста
│   │   └── soundManager.ts     # Управление звуковыми эффектами
│   ├── App.tsx                 # Главный компонент + роутинг
│   ├── main.tsx                # Точка входа
│   └── index.css               # Глобальные стили + Tailwind
├── public/                     # Статические файлы
├── index.html                  # HTML шаблон
├── package.json
├── vite.config.ts
├── tailwind.config.js
├── postcss.config.js
├── eslint.config.js
├── tsconfig.json
├── tsconfig.app.json
└── tsconfig.node.json
```

## Страницы

| Страница | Путь | Описание |
|----------|------|----------|
| Home | `/` | Главная страница (требует авторизации) |
| Login | `/login` | Вход в систему |
| Profile | `/profile` | Профиль пользователя (требует авторизации) |
| Tournaments | `/tournaments` | Список турниров |
| Tournament Detail | `/tournaments/:id` | Детали турнира (вкладки: Info, Leaderboard, Games, Teams) |
| Game Detail | `/tournaments/:tournamentId/games/:gameId` | Правила игры, загрузка программы |
| Games | `/games` | Каталог игр |
| Game View | `/games/:id` | Просмотр правил отдельной игры |
| Team Management | `/teams/:id` | Управление командой (требует авторизации) |
| Admin Panel | `/admin` | Админ-панель (только для admin) |
| Not Found | `*` | Страница 404 |

## Lazy Loading и Code Splitting

Все страницы загружаются через `React.lazy()` с `Suspense`. При инициализации приложения запускается опережающая подгрузка всех страниц (`prefetchAllPages`) со ступенчатой задержкой в 30 мс, чтобы не блокировать основной поток.

Vite дополнительно разделяет бандл на чанки через `manualChunks`:

| Чанк | Содержимое |
|------|------------|
| `three` | Three.js |
| `vendor-react` | react, react-dom, react-router-dom |
| `vendor-data` | axios, zustand |
| `vendor-markdown` | react-markdown, remark-gfm |
| `vendor-motion` | motion (Framer Motion) |

## Маскот и геймификация

Фронтенд обладает развитой системой геймификации вокруг персонажа Space Invader -- пиксельного маскота, реализованного через CSS pixel art.

### Space Invader

Анимированный маскот, который присутствует по всему интерфейсу и реагирует на действия пользователя:

- **Реакции на события** -- победа в матче, поражение, загрузка программы, создание команды, ошибки API (400, 403, 404, 500). Каждое событие вызывает уникальную позу, фразу и анимацию.
- **17 поз** -- idle, handsUp, dance, run, spin, cry, sleep, fly, attack, shield, teleport, transform, celebrate, peek, salute, dizzy, typing.
- **Интерактивность** -- реагирует на фокус полей ввода, скролл, бездействие пользователя.

### Пасхалки

- **Konami Code** -- классическая комбинация активирует God Mode с эффектом сканлайнов.
- **Последовательный набор** -- скрытые фразы запускают специальные анимации.
- **Быстрые клики** -- серия кликов по элементам интерфейса вызывает реакцию маскота.

### Система квестов

Полноценный терминальный квест из 5 уровней, где пользователь помогает инвейдеру «сбежать из кода»:

1. **Разведка** -- сканирование системы, поиск скрытых файлов, чтение логов.
2. **Файрвол** -- расшифровка ключей, атака, мини-игра Pong.
3. **Зона багов** -- отладка, активация щита, мини-игра Typing.
4. **Охрана** -- усыпление охраны, телепортация, мини-игра Strategy.
5. **Побег** -- полёт, трансформация, финальный побег.

Квест включает встроенные мини-игры, парсер команд терминала и систему целей.

### Кинематографические эффекты

- **CinematicOverlay** -- кинематографическая заставка при первом входе.
- **TerminalTypewriter** -- эффект последовательной печати текста в стиле терминала.

## API клиент

API клиент расположен в `src/api/client.ts`:

```typescript
import api from './api/client';

// Аутентификация
await api.login(username, password);
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

Состояние маскота управляется через `InvaderContext` (React Context), который обрабатывает события с приоритетами и последовательностями анимаций.

## Стилизация

Tailwind CSS v4 с кастомной тёмной темой (фиолетовая палитра):

```css
/* src/index.css */
@import "tailwindcss";

@variant dark (&:where(.dark, .dark *));

@theme {
  --color-primary-500: #8b5cf6;
  --color-primary-600: #7c3aed;
  /* ... */
}
```

Интерфейс выполнен в тёмной цветовой схеме с неоновыми акцентами, glassmorphism-эффектами и моноширинным шрифтом в стиле терминала.

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

Vite настроен на сборку в `../internal/web/dist`. После `npm run build` запустите `go build` для включения фронтенда в бинарник.
