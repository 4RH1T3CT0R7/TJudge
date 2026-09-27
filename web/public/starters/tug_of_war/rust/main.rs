// Перетягивание каната: стратегия «равная трата».
// Остаток энергии делится поровну на оставшиеся итерации, к концу тратится весь.
// Протокол: ← строка от судьи (stdin), → ваша строка (stdout).
// Отладку печатайте через eprintln!, stdout читает судья.
use std::io::{self, BufRead, Write};

fn main() {
    let mut lines = io::stdin().lock().lines().map(|line| line.unwrap());
    let mut energy: u32 = lines.next().unwrap().trim().parse().unwrap(); // ← начальная энергия
    let n: u32 = lines.next().unwrap().trim().parse().unwrap(); // ← число итераций
    for i in 0..n {
        let spend = energy / (n - i); // поровну на оставшиеся итерации
        energy -= spend;
        println!("{}", spend); // → сколько потратить: от 0 до остатка
        io::stdout().flush().unwrap(); // сброс вывода обязателен
        let _opponent: u32 = lines.next().unwrap().trim().parse().unwrap(); // ← сколько потратил соперник
    }
}
