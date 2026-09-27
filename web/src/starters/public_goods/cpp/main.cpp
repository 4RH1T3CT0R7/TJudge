// Общественное благо: стратегия «условный вклад».
// Сначала вложить всё, дальше столько же, сколько соперник вложил в прошлый раз.
// Протокол: ← строка от судьи (stdin), → ваша строка (stdout).
// Отладку печатайте в std::cerr, stdout читает судья.
#include <iostream>

int main() {
    int capital, n;
    double multiplier;
    // ← капитал на итерацию E, множитель пула m (дробное число), число итераций
    std::cin >> capital >> multiplier >> n;
    int give = capital;                       // первый вклад - весь капитал
    for (int i = 0; i < n; i++) {
        std::cout << give << std::endl;       // → вклад; std::endl сбрасывает вывод
        std::cin >> give;                     // ← вклад соперника
    }
}
