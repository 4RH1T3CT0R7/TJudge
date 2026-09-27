// Дилемма заключённого: стратегия «око за око» (tit for tat).
// Первый ход - сотрудничество, дальше повтор прошлого хода соперника.
// Протокол: ← строка от судьи (stdin), → ваша строка (stdout).
// Отладку печатайте через eprintln!, stdout читает судья.
use std::io::{self, BufRead, Write};

fn main() {
    let mut lines = io::stdin().lock().lines().map(|line| line.unwrap());
    let n: usize = lines.next().unwrap().trim().parse().unwrap(); // ← число итераций
    let mut choice = String::from("COOPERATE");
    for _ in 0..n {
        println!("{}", choice); // → ваш ход
        io::stdout().flush().unwrap(); // сброс вывода обязателен
        choice = lines.next().unwrap().trim().to_string(); // ← ход соперника
    }
}
