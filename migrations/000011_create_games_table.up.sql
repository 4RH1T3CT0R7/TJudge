-- Create games table for storing game configurations
CREATE TABLE IF NOT EXISTS games (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(50) NOT NULL UNIQUE,
    display_name VARCHAR(255) NOT NULL,
    rules TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    CONSTRAINT valid_game_name CHECK (name ~ '^[a-z0-9_]+$')
);

-- Create indexes
CREATE INDEX idx_games_name ON games(name);

-- Create trigger for updated_at
CREATE TRIGGER update_games_updated_at
    BEFORE UPDATE ON games
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- Insert default games
INSERT INTO games (name, display_name, rules) VALUES
('prisoners_dilemma', 'Дилемма заключённого', E'# Дилемма заключённого\n\nКлассическая игра теории игр.\n\n## Правила\n\nКаждый игрок выбирает одно из двух действий:\n- **C** (Cooperate) — сотрудничать\n- **D** (Defect) — предать\n\n## Таблица выплат\n\n| A \\ B | C | D |\n|-------|-------|-------|\n| C | 5 \\ 5 | 0 \\ 10 |\n| D | 10 \\ 0 | 1 \\ 1 |\n\n## Формат ввода-вывода\n\n1. Программа выводит C или D в stdout\n2. Получает выбор соперника через stdin\n3. Повторяет N итераций'),
('tictactoe', 'Крестики-нолики', E'# Крестики-нолики\n\nКлассическая игра 3x3.\n\n## Правила\n\nИгроки по очереди ставят X и O на поле 3x3. Побеждает тот, кто первым соберёт 3 в ряд.\n\n## Формат ввода-вывода\n\nПозиции нумеруются 0-8:\n```\n0 | 1 | 2\n---------\n3 | 4 | 5\n---------\n6 | 7 | 8\n```\n\n1. Программа выводит номер клетки (0-8)\n2. Получает ход соперника через stdin');
