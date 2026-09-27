// Перетягивание каната: стратегия «равная трата».
// Остаток энергии делится поровну на оставшиеся итерации, к концу тратится весь.
// Протокол: ← строка от судьи (stdin), → ваша строка (stdout).
// Отладку печатайте в std::cerr, stdout читает судья.
#include <iostream>

int main() {
    int energy, n, opponent;
    std::cin >> energy >> n;                  // ← начальная энергия и число итераций
    for (int i = 0; i < n; i++) {
        int spend = energy / (n - i);         // поровну на оставшиеся итерации
        energy -= spend;
        std::cout << spend << std::endl;      // → трата; std::endl сбрасывает вывод
        std::cin >> opponent;                 // ← трата соперника
    }
}
