import { useEffect, useState } from 'react';

// QR-код ссылки. Кодировщик грузится лениво: нужен только табло.
// Чёрное на белом с полями: тёмная инверсия читается не всеми камерами.
export function QrCode({ text, className = '' }: { text: string; className?: string }) {
  const [qr, setQr] = useState<{ d: string; size: number } | null>(null);

  useEffect(() => {
    let cancelled = false;
    void import('uqr').then(({ encode }) => {
      if (cancelled) return;
      const { data, size } = encode(text, { ecc: 'M', border: 2 });
      let d = '';
      data.forEach((row, y) => row.forEach((on, x) => {
        if (on) d += `M${x} ${y}h1v1h-1z`;
      }));
      setQr({ d, size });
    });
    return () => {
      cancelled = true;
    };
  }, [text]);

  if (!qr) return <div className={className} aria-hidden="true" />;
  return (
    <svg viewBox={`0 0 ${qr.size} ${qr.size}`} className={className} role="img" aria-label={`QR-код ссылки ${text}`} shapeRendering="crispEdges">
      <rect width={qr.size} height={qr.size} fill="#fff" />
      <path d={qr.d} fill="#000" />
    </svg>
  );
}
