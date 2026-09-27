// Перетягивание каната: стратегия «равная трата».
// Остаток энергии делится поровну на оставшиеся итерации, к концу тратится весь.
// Протокол: ← строка от судьи (stdin), → ваша строка (stdout).
// Отладку печатайте через eprintln!, stdout читает судья.
use std::io::{self, BufRead, Write};

fn main() {
    let mut lines = io::stdin().lock().lines().map(|line| line.unwrap());
    let mut next = || -> u32 { lines.next().unwrap().trim().parse().unwrap() };
    let mut energy = next(); // ← начальная энергия
    let n = next(); // ← число итераций
    for i in 0..n {
        let spend = energy / (n - i); // поровну на оставшиеся итерации
        energy -= spend;
        println!("{}", spend); // → трата: от 0 до остатка
        io::stdout().flush().unwrap(); // сброс вывода обязателен
        let _opponent = next(); // ← трата соперника
    }
}
