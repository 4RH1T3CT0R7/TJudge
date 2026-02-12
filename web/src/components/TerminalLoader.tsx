import { useState, useEffect, useRef } from 'react';

const messages = [
  '> Загрузка данных...',
  '> Подключение к серверу...',
  '> Получение ответа...',
  '> Обработка...',
  '> Почти готово...',
];

export function TerminalLoader() {
  const [lines, setLines] = useState<string[]>([]);
  const [currentText, setCurrentText] = useState('');
  const [cursorVisible, setCursorVisible] = useState(true);
  const msgIndexRef = useRef(0);
  const charIndexRef = useRef(0);
  const lineCountRef = useRef(0);

  // Blinking cursor
  useEffect(() => {
    const interval = setInterval(() => setCursorVisible((v) => !v), 530);
    return () => clearInterval(interval);
  }, []);

  // Typing effect
  useEffect(() => {
    const msg = messages[msgIndexRef.current % messages.length];

    const type = () => {
      if (charIndexRef.current <= msg.length) {
        setCurrentText(msg.slice(0, charIndexRef.current));
        charIndexRef.current++;
        return 30 + Math.random() * 50;
      }
      // Finished typing current line — pause then start next
      setLines((prev) => [...prev.slice(-2), msg]); // keep max 3 completed lines
      setCurrentText('');
      charIndexRef.current = 0;
      msgIndexRef.current++;
      lineCountRef.current++;
      return 1500;
    };

    let timeout: ReturnType<typeof setTimeout>;
    const tick = () => {
      const delay = type();
      timeout = setTimeout(tick, delay);
    };
    timeout = setTimeout(tick, 400);

    return () => clearTimeout(timeout);
  }, []);

  return (
    <div className="flex items-center justify-center py-12">
      <div className="font-mono text-sm min-h-[6rem] w-full max-w-md">
        {lines.map((line, i) => (
          <div key={i} className="text-green-500/60 leading-relaxed">
            {line}
          </div>
        ))}
        <div className="text-green-400 leading-relaxed">
          {currentText}
          <span
            className={`inline-block w-2 h-4 ml-0.5 align-middle bg-green-400 ${
              cursorVisible ? 'opacity-100' : 'opacity-0'
            }`}
            style={{ transition: 'opacity 0.1s' }}
          />
        </div>
      </div>
    </div>
  );
}
