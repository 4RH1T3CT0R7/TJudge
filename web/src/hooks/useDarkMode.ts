import { useEffect } from 'react';

export function useDarkMode() {
  useEffect(() => {
    document.documentElement.classList.add('dark');
  }, []);

  return { isDark: true };
}
