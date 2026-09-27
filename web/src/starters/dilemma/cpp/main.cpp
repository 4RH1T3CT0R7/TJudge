// Дилемма заключённого: стратегия «око за око» (tit for tat).
// Первый ход - сотрудничество, дальше повтор прошлого хода соперника.
// Протокол: ← строка от судьи (stdin), → ваша строка (stdout).
// Отладку печатайте в std::cerr, stdout читает судья.
#include <iostream>
#include <string>

int main() {
    int n;
    std::cin >> n;                        // ← число итераций
    std::string move = "COOPERATE";
    for (int i = 0; i < n; i++) {
        std::cout << move << std::endl;   // → ваш ход; std::endl сбрасывает вывод
        std::cin >> move;                 // ← ход соперника
    }
}
