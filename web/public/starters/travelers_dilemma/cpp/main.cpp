// Дилемма путешественника: стратегия «заявка 90».
// Каждую итерацию одна и та же высокая заявка, но внутри границ [L, U].
// Протокол: ← строка от судьи (stdin), → ваша строка (stdout).
// Отладку печатайте в std::cerr, stdout читает судья.
#include <algorithm>
#include <iostream>

int main() {
    int low, high, bonus, n, opponent;
    std::cin >> low >> high >> bonus >> n;    // ← нижняя граница L, верхняя U, бонус и штраф R, число итераций
    int claim = std::clamp(90, low, high);    // 90, если оно внутри [L, U]
    for (int i = 0; i < n; i++) {
        std::cout << claim << std::endl;      // → заявка: целое число от L до U; std::endl сбрасывает вывод
        std::cin >> opponent;                 // ← заявка соперника
    }
}
