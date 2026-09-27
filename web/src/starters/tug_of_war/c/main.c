// Перетягивание каната: стратегия «равная трата».
// Остаток энергии делится поровну на оставшиеся итерации, к концу тратится весь.
// Протокол: ← строка от судьи (stdin), → ваша строка (stdout).
// Отладку печатайте в stderr: fprintf(stderr, ...), stdout читает судья.
#include <stdio.h>

int main(void) {
    int energy, n, opponent;
    // ← начальная энергия, затем число итераций
    if (scanf("%d %d", &energy, &n) != 2) return 1;
    for (int i = 0; i < n; i++) {
        int spend = energy / (n - i);               // поровну на оставшиеся итерации
        energy -= spend;
        printf("%d\n", spend);                      // → трата: от 0 до остатка
        fflush(stdout);                             // сброс вывода обязателен
        if (scanf("%d", &opponent) != 1) return 1;  // ← трата соперника
    }
    return 0;
}
