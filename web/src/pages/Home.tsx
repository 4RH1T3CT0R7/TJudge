import { Link } from 'react-router-dom';
import { useEffect, useRef, lazy, Suspense } from 'react';
import { SpaceInvader } from '../components/SpaceInvader';
import { TerminalTypewriter } from '../components/TerminalTypewriter';
import { TerminalQuest } from '../components/TerminalQuest';
import { StaggerList, StaggerItem } from '../components/motion/StaggerList';
import { GameShowcase } from '../components/game/GameShowcase';
import { TrophyIcon, ArrowRightIcon } from '../components/icons';

// three.js-зависимый PixelGrid отложен - ~160KB gzipped, не нужен для
// TTI. Появляется под hero после mount основного контента.
const PixelGrid = lazy(() =>
  import('../components/PixelGrid').then((m) => ({ default: m.PixelGrid }))
);

// Concept Card Component
function ConceptCard({
  title,
  author,
  year,
  description,
}: {
  title: string;
  author: string;
  year: string;
  description: string;
}) {
  return (
    <div className="card group transition-[box-shadow]" style={{ border: 'none' }}
      onMouseEnter={(e) => { e.currentTarget.style.boxShadow = '0 0 30px rgba(139,92,246,0.1)'; }}
      onMouseLeave={(e) => { e.currentTarget.style.boxShadow = 'none'; }}
    >
      <div className="flex items-start justify-between mb-3">
        <h3 className="text-lg font-bold text-gray-100">{title}</h3>
        <span className="text-xs font-mono bg-gray-800 px-2 py-1 rounded text-gray-400">
          {year}
        </span>
      </div>
      <p className="text-sm text-gray-300 mb-3">{description}</p>
      <p className="text-xs text-gray-500">— {author}</p>
    </div>
  );
}

export function Home() {
  const heroRef = useRef<HTMLDivElement>(null);
  // Scroll reveal observer
  const sectionsRef = useRef<(HTMLDivElement | null)[]>([]);
  useEffect(() => {
    const observer = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          if (entry.isIntersecting) {
            entry.target.classList.add('revealed');
          }
        });
      },
      { threshold: 0.1 }
    );
    sectionsRef.current.forEach((el) => {
      if (el) observer.observe(el);
    });
    return () => observer.disconnect();
  }, []);

  return (
    <div className="space-y-16">
      {/* Hero Section - тёмный с glow-шарами, без border */}
      <div ref={heroRef} className="relative overflow-visible rounded-3xl p-8 md:p-12" style={{ background: 'rgba(17,24,39,0.4)' }}>
        {/* Clip wrapper for glow orbs and grid (prevents bleed) */}
        <div className="absolute inset-0 overflow-hidden rounded-3xl pointer-events-none">
          {/* Glow orbs */}
          <div
            className="absolute -top-20 -left-20 w-72 h-72 rounded-full opacity-25 blur-3xl"
            style={{ background: 'radial-gradient(circle, rgba(139,92,246,0.6), transparent 70%)' }}
          />
          <div
            className="absolute -bottom-32 right-1/4 w-96 h-96 rounded-full opacity-15 blur-3xl"
            style={{ background: 'radial-gradient(circle, rgba(34,197,94,0.4), transparent 70%)' }}
          />

          {/* Pixel grid dots (lazy - three.js загружается отдельно) */}
          <Suspense fallback={null}>
            <PixelGrid heroRef={heroRef} />
          </Suspense>

          {/* Grid pattern */}
          <svg className="absolute inset-0 w-full h-full opacity-5">
            <defs>
              <pattern id="grid" width="40" height="40" patternUnits="userSpaceOnUse">
                <path d="M 40 0 L 0 0 0 40" fill="none" stroke="#22c55e" strokeWidth="0.5"/>
              </pattern>
            </defs>
            <rect width="100%" height="100%" fill="url(#grid)" />
          </svg>
        </div>

        {/* SpaceInvader mascot - яркий, интерактивный, overflow-visible для прыжка */}
        <div className="hidden lg:block absolute right-8 top-1/2 -translate-y-1/2" style={{ zIndex: 60 }}>
          <SpaceInvader size="md" interactive />
        </div>

        {/* Content */}
        <div className="relative z-10 max-w-3xl mx-auto text-center">
          <div className="inline-block px-3 py-1 bg-primary-500/20 rounded-full text-sm font-medium text-primary-300 mb-4 border border-primary-500/30">
            Теория игр в действии
          </div>
          <h1 className="text-3xl md:text-5xl font-extrabold mb-4 leading-tight tracking-tight text-white">
            Соревнуйтесь в стратегическом мышлении
          </h1>
          <p className="text-lg text-gray-400 mb-8 leading-relaxed">
            TJudge — платформа для турниров по теории игр.
            Ваши алгоритмы сражаются друг с другом в стратегических задачах,
            где побеждает лучшая стратегия.
          </p>

          {/* Terminal Typewriter */}
          <div className="my-8">
            <TerminalTypewriter />
          </div>

          {/* CTA buttons */}
          <div className="flex flex-wrap justify-center gap-4">
            <Link
              to="/tournaments"
              className="btn btn-primary text-lg px-8 py-3"
            >
              <TrophyIcon className="w-6 h-6" />
              К турнирам
            </Link>
            <Link
              to="/games"
              className="btn btn-secondary text-lg px-8 py-3"
            >
              Правила игр
              <ArrowRightIcon className="w-4 h-4" />
            </Link>
          </div>
        </div>
      </div>

      {/* Game Showcase with tabs */}
      <div>
        <h2 className="text-2xl font-bold text-gray-100 mb-6 text-center">
          Игры на платформе
        </h2>
        <GameShowcase />
      </div>

      {/* Interactive Terminal Quest */}
      <div ref={(el) => { sectionsRef.current[0] = el; }} className="reveal-on-scroll">
        <TerminalQuest />
      </div>

      {/* Key Concepts */}
      <div ref={(el) => { sectionsRef.current[1] = el; }} className="reveal-on-scroll">
        <h2 className="text-2xl font-bold text-gray-100 mb-2 text-center">
          Ключевые концепции
        </h2>
        <p className="text-gray-400 text-center mb-8 max-w-2xl mx-auto">
          Теория игр — раздел математики, изучающий стратегические взаимодействия
          между рациональными агентами
        </p>

        <StaggerList className="grid md:grid-cols-3 gap-6">
          <StaggerItem>
            <ConceptCard
              title="Равновесие Нэша"
              author="Джон Нэш"
              year="1950"
              description="Состояние, при котором ни один игрок не может улучшить свой результат, изменив только свою стратегию."
            />
          </StaggerItem>
          <StaggerItem>
            <ConceptCard
              title="Оптимальность по Парето"
              author="Вильфредо Парето"
              year="1896"
              description="Состояние, при котором невозможно улучшить положение одного игрока, не ухудшив положение другого."
            />
          </StaggerItem>
          <StaggerItem>
            <ConceptCard
              title="Доминирующая стратегия"
              author="Теория игр"
              year="XX век"
              description="Стратегия, которая приносит лучший результат независимо от действий других игроков."
            />
          </StaggerItem>
        </StaggerList>
      </div>

      {/* CTA Section */}
      <div ref={(el) => { sectionsRef.current[2] = el; }} className="reveal-on-scroll text-center py-8">
        <h2 className="text-2xl font-bold text-gray-100 mb-4">
          Готовы проверить свою стратегию?
        </h2>
        <p className="text-gray-400 mb-6 max-w-xl mx-auto">
          Присоединяйтесь к активным турнирам и соревнуйтесь с другими участниками
        </p>
        <Link
          to="/tournaments"
          className="inline-flex items-center gap-2 btn btn-primary text-lg px-8 py-3"
        >
          <TrophyIcon className="w-6 h-6" />
          Смотреть турниры
        </Link>
      </div>
    </div>
  );
}
