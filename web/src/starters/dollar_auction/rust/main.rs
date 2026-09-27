// Аукцион двойной цены: стратегия «пас после порога».
// Перебивать соперника на 1, пока ставка не выше половины приза, дальше пас.
// Протокол: ← строка от судьи (stdin), → ваша строка (stdout).
// Ходы поочерёдные: сначала ставка соперника, потом ваш ответ. О пасе соперника
// судья не сообщает: ввод просто заканчивается.
// Отладку печатайте через eprintln!, stdout читает судья.
use std::io::{self, BufRead, Write};

fn main() {
    let mut lines = io::stdin().lock().lines().map(|line| line.unwrap());
    let mut next = || -> Option<i64> { Some(lines.next()?.trim().parse().unwrap()) };
    let prize = next().unwrap(); // ← приз P
    let _max_iters = next().unwrap(); // ← максимум итераций
    let limit = prize / 2; // дороже половины приза не торговаться
    while let Some(opponent) = next() {
        // ← ставка соперника, 0 в начале торгов
        let mut bid = opponent + 1; // перебить на 1
        if bid > limit {
            bid = 0; // 0 = пас, торги заканчиваются
        }
        println!("{}", bid); // → ставка выше соперника или 0
        io::stdout().flush().unwrap(); // сброс вывода обязателен
    }
}
