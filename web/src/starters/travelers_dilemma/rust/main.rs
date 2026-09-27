// Дилемма путешественника: стратегия «заявка 90».
// Каждую итерацию одна и та же высокая заявка, но внутри границ [L, U].
// Протокол: ← строка от судьи (stdin), → ваша строка (stdout).
// Отладку печатайте через eprintln!, stdout читает судья.
use std::io::{self, BufRead, Write};

fn main() {
    let mut lines = io::stdin().lock().lines().map(|line| line.unwrap());
    let mut next = || -> i64 { lines.next().unwrap().trim().parse().unwrap() };
    let low = next(); // ← нижняя граница L
    let high = next(); // ← верхняя граница U
    let _bonus = next(); // ← бонус и штраф R
    let n = next(); // ← число итераций
    let claim = 90.clamp(low, high); // 90, если оно внутри [L, U]
    for _ in 0..n {
        println!("{}", claim); // → заявка: целое число от L до U
        io::stdout().flush().unwrap(); // сброс вывода обязателен
        let _opponent = next(); // ← заявка соперника
    }
}
