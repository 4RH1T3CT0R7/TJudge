import { useState, useEffect } from 'react';
import { SpaceInvader } from './SpaceInvader';

export function PageLoader() {
  const [visible, setVisible] = useState(false);

  useEffect(() => {
    const timer = setTimeout(() => setVisible(true), 1000);
    return () => clearTimeout(timer);
  }, []);

  if (!visible) return null;

  return (
    <div className="flex flex-col items-center justify-center min-h-[60vh] gap-6 animate-fade-in">
      <SpaceInvader size="sm" />
      <div className="flex items-center gap-2 text-gray-500 font-mono text-sm">
        <span className="text-primary-400">$</span>
        <span>loading</span>
        <span className="terminal-cursor">_</span>
      </div>
    </div>
  );
}
