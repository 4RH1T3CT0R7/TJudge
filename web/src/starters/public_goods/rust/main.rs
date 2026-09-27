// Общественное благо: стратегия «условный вклад».
// Сначала вложить всё, дальше столько же, сколько соперник вложил в прошлый раз.
// Протокол: ← строка от судьи (stdin), → ваша строка (stdout).
// Отладку печатайте через eprintln!, stdout читает судья.
use std::io::{self, BufRead, Write};

fn main() {
    let mut lines = io::stdin().lock().lines().map(|line| line.unwrap());
    let capital: u32 = lines.next().unwrap().trim().parse().unwrap(); // ← капитал E
    // ← множитель пула m, дробное число
    let _multiplier: f64 = lines.next().unwrap().trim().parse().unwrap();
    let n: u32 = lines.next().unwrap().trim().parse().unwrap(); // ← число итераций
    let mut give = capital; // первый вклад - весь капитал
    for _ in 0..n {
        println!("{}", give); // → вклад: целое число от 0 до E
        io::stdout().flush().unwrap(); // сброс вывода обязателен
        give = lines.next().unwrap().trim().parse().unwrap(); // ← вклад соперника
    }
}
