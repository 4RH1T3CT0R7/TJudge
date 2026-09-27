// Аукцион двойной цены: стратегия «пас после порога».
// Перебивать ставку соперника на 1, пока своя ставка не выше половины приза, дальше пас.
// Протокол: ← строка от судьи (stdin), → ваша строка (stdout).
// Здесь ходы поочерёдные: сначала приходит ставка соперника, потом ваш ответ.
// Отладку печатайте через eprintln!, stdout читает судья.
use std::io::{self, BufRead, Write};

fn main() {
    let mut lines = io::stdin().lock().lines().map(|line| line.unwrap());
    let prize: i64 = lines.next().unwrap().trim().parse().unwrap(); // ← стоимость приза P
    let _rounds: i64 = lines.next().unwrap().trim().parse().unwrap(); // ← максимум раундов торгов
    let limit = prize / 2; // дороже половины приза не торговаться
    for line in lines {
        let opponent: i64 = line.trim().parse().unwrap(); // ← ставка соперника (0 в начале торгов)
        let mut bid = opponent + 1; // перебить на 1
        if bid > limit {
            bid = 0; // 0 = пас, торги заканчиваются
        }
        println!("{}", bid); // → ставка: больше ставки соперника или 0
        io::stdout().flush().unwrap(); // сброс вывода обязателен
    }
}
