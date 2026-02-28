import { useState, useEffect, useCallback, useRef } from 'react';
import { SpaceInvader } from './SpaceInvader';
import type { InvaderPose } from './SpaceInvader';

type CinematicType = 'first_login' | 'tournament_victory' | 'top1_leaderboard';

interface CinematicProps {
  type: CinematicType;
  username?: string;
  teamName?: string;
  onComplete: () => void;
}

// Terminal typing effect
function useTerminalTyping(lines: string[], startDelay: number) {
  const [displayedLines, setDisplayedLines] = useState<string[]>([]);
  const [currentLine, setCurrentLine] = useState('');
  const [lineIndex, setLineIndex] = useState(0);
  const [charIndex, setCharIndex] = useState(0);
  const [started, setStarted] = useState(false);

  useEffect(() => {
    const t = setTimeout(() => setStarted(true), startDelay);
    return () => clearTimeout(t);
  }, [startDelay]);

  useEffect(() => {
    if (!started || lineIndex >= lines.length) return;

    const line = lines[lineIndex];
    if (charIndex < line.length) {
      const t = setTimeout(() => {
        setCurrentLine(prev => prev + line[charIndex]);
        setCharIndex(c => c + 1);
      }, 30 + Math.random() * 40);
      return () => clearTimeout(t);
    } else {
      const t = setTimeout(() => {
        setDisplayedLines(prev => [...prev, currentLine]);
        setCurrentLine('');
        setCharIndex(0);
        setLineIndex(i => i + 1);
      }, 300);
      return () => clearTimeout(t);
    }
  }, [started, lineIndex, charIndex, lines, currentLine]);

  const done = lineIndex >= lines.length;
  return { displayedLines, currentLine, done };
}

// ASCII trophy
const ASCII_TROPHY = [
  '    ___________',
  '   \'._==_==_=_.\'',
  '   .-\\:      /-.',
  '  | (|:.     |) |',
  '   \'-|:.     |-\'',
  '     \\::.    /',
  '      \'::. .\'',
  '        ) (',
  '      _.|  |._',
  '     `"""`""`',
];

export function CinematicOverlay({ type, username, teamName, onComplete }: CinematicProps) {
  const [phase, setPhase] = useState(0);
  const [invaderPose, setInvaderPose] = useState<InvaderPose>('idle');
  const [invaderSpeech, setInvaderSpeech] = useState<string | null>(null);
  const [showTrophy, setShowTrophy] = useState(false);
  const [trophyLines, setTrophyLines] = useState(0);
  const timerRef = useRef<ReturnType<typeof setTimeout>[]>([]);

  const addTimer = useCallback((fn: () => void, ms: number) => {
    timerRef.current.push(setTimeout(fn, ms));
  }, []);

  useEffect(() => {
    const timers = timerRef.current;
    return () => timers.forEach(clearTimeout);
  }, []);

  // First login cinematic sequence
  const firstLoginLines = [
    '> Connecting to tjudge.io...',
    '> Authenticating...',
    '> Access granted.',
  ];
  const { displayedLines, currentLine, done: typingDone } = useTerminalTyping(
    type === 'first_login' ? firstLoginLines : [],
    500
  );

  // First login sequence
  useEffect(() => {
    if (type !== 'first_login') return;
    if (!typingDone || phase > 0) return;

    setPhase(1);
    // Invader teleport in
    addTimer(() => {
      setInvaderPose('teleport');
      setInvaderSpeech(null);
    }, 500);
    // Dance + welcome
    addTimer(() => {
      setInvaderPose('dance');
      setInvaderSpeech(`// добро пожаловать, ${username || 'user'}!`);
    }, 1500);
    // Fade out
    addTimer(() => {
      setPhase(2);
    }, 4500);
    addTimer(() => {
      onComplete();
    }, 5500);
  }, [type, typingDone, phase, username, addTimer, onComplete]);

  // Tournament victory sequence
  useEffect(() => {
    if (type !== 'tournament_victory') return;

    // Phase 0: screen shake + glitch
    addTimer(() => setPhase(1), 500);

    // Phase 1: ASCII trophy typing
    addTimer(() => {
      setShowTrophy(true);
      let line = 0;
      const typeInterval = setInterval(() => {
        line++;
        setTrophyLines(line);
        if (line >= ASCII_TROPHY.length) {
          clearInterval(typeInterval);
        }
      }, 200);
      timerRef.current.push(typeInterval as unknown as ReturnType<typeof setTimeout>);
    }, 1000);

    // Phase 2: Invader flies in
    addTimer(() => {
      setInvaderPose('fly');
      setPhase(2);
    }, 3500);

    // Phase 3: Dance + celebration
    addTimer(() => {
      setInvaderPose('dance');
      setInvaderSpeech(`// ${teamName || 'Команда'} — 1st PLACE!`);
    }, 5000);

    // Fade out
    addTimer(() => setPhase(3), 7000);
    addTimer(() => onComplete(), 8000);
  }, [type, teamName, addTimer, onComplete]);

  // Top 1 leaderboard sequence
  useEffect(() => {
    if (type !== 'top1_leaderboard') return;

    // Phase 1: Flash gold
    addTimer(() => setPhase(1), 300);

    // Phase 2: Invader fly up
    addTimer(() => {
      setInvaderPose('fly');
      setPhase(2);
    }, 1000);

    // Phase 3: Spin + handsUp
    addTimer(() => {
      setInvaderPose('spin');
    }, 2000);
    addTimer(() => {
      setInvaderPose('handsUp');
      setInvaderSpeech('// #1!!!');
    }, 3000);

    // Fade out
    addTimer(() => setPhase(3), 4500);
    addTimer(() => onComplete(), 5500);
  }, [type, addTimer, onComplete]);

  const isFading = (type === 'first_login' && phase >= 2) ||
                   (type === 'tournament_victory' && phase >= 3) ||
                   (type === 'top1_leaderboard' && phase >= 3);

  return (
    <div
      className={`fixed inset-0 z-[100] flex flex-col items-center justify-center transition-opacity duration-1000 ${
        isFading ? 'opacity-0' : 'opacity-100'
      }`}
      style={{ backgroundColor: 'rgba(0,0,0,0.95)' }}
    >
      {/* First Login */}
      {type === 'first_login' && (
        <>
          {/* Terminal text */}
          <div className="font-mono text-sm text-green-400 mb-8 min-h-[120px]">
            {displayedLines.map((line, i) => (
              <div key={i} className="mb-1">{line}</div>
            ))}
            {currentLine && (
              <div>
                {currentLine}
                <span className="animate-pulse">_</span>
              </div>
            )}
          </div>

          {/* Invader (appears after typing) */}
          {phase >= 1 && (
            <div className="transition-transform duration-500" style={{ transform: phase >= 1 ? 'scale(1)' : 'scale(0)' }}>
              <SpaceInvader
                size="lg"
                controlledPose={invaderPose}
                speechBubble={invaderSpeech}
                eyeOverride="wide"
              />
            </div>
          )}

          {/* Light rays effect */}
          {phase >= 1 && (
            <div className="absolute inset-0 pointer-events-none" style={{
              background: 'radial-gradient(circle at 50% 50%, rgba(139,92,246,0.15) 0%, transparent 70%)',
            }} />
          )}
        </>
      )}

      {/* Tournament Victory */}
      {type === 'tournament_victory' && (
        <>
          {/* Screen shake effect via className */}
          <div className={phase === 0 ? 'animate-shake' : ''}>
            {/* ASCII Trophy */}
            {showTrophy && (
              <div className="font-mono text-amber-400 text-center mb-6 text-xs sm:text-sm">
                {ASCII_TROPHY.slice(0, trophyLines).map((line, i) => (
                  <div key={i} style={{ textShadow: '0 0 10px rgba(245,158,11,0.5)' }}>{line}</div>
                ))}
              </div>
            )}

            {/* Invader */}
            {phase >= 2 && (
              <div className="flex justify-center">
                <SpaceInvader
                  size="lg"
                  controlledPose={invaderPose}
                  speechBubble={invaderSpeech}
                  eyeOverride="wide"
                />
              </div>
            )}

            {/* Team name */}
            {phase >= 2 && (
              <div className="text-center mt-4">
                <div className="text-2xl font-bold text-amber-400 font-mono" style={{ textShadow: '0 0 20px rgba(245,158,11,0.5)' }}>
                  1st PLACE
                </div>
                <div className="text-lg text-gray-300 mt-1">{teamName}</div>
              </div>
            )}
          </div>
        </>
      )}

      {/* Top 1 Leaderboard */}
      {type === 'top1_leaderboard' && (
        <>
          {/* Gold flash */}
          {phase >= 1 && (
            <div className="absolute inset-0 pointer-events-none animate-pulse" style={{
              background: 'radial-gradient(circle at 50% 40%, rgba(245,158,11,0.2) 0%, transparent 60%)',
            }} />
          )}

          {/* Invader */}
          <div className="transition-transform duration-700" style={{
            transform: phase >= 2 ? 'translateY(-20px)' : 'translateY(0)',
          }}>
            <SpaceInvader
              size="lg"
              controlledPose={invaderPose}
              speechBubble={invaderSpeech}
              eyeOverride="wide"
            />
          </div>

          {phase >= 2 && (
            <div className="text-center mt-6">
              <div className="text-3xl font-bold text-amber-400 font-mono" style={{ textShadow: '0 0 20px rgba(245,158,11,0.5)' }}>
                #1
              </div>
            </div>
          )}
        </>
      )}
    </div>
  );
}
