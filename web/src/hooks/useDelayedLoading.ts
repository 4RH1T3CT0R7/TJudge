import { useState, useEffect } from 'react';

/**
 * Returns true only if isLoading has been true for at least `delay` ms.
 * This prevents flickering loaders on fast responses.
 */
export function useDelayedLoading(isLoading: boolean, delay = 1000): boolean {
  const [show, setShow] = useState(false);

  /* eslint-disable react-hooks/set-state-in-effect */
  useEffect(() => {
    if (!isLoading) {
      setShow(false);
      return;
    }
    const timer = setTimeout(() => setShow(true), delay);
    return () => clearTimeout(timer);
  }, [isLoading, delay]);
  /* eslint-enable react-hooks/set-state-in-effect */

  return show;
}
