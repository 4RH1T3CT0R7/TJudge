// Дилемма путешественника: стратегия «заявка 90».
// Каждую итерацию одна и та же высокая заявка, но внутри границ [L, U].
// Протокол: ← строка от судьи (stdin), → ваша строка (stdout).
// Отладку печатайте в stderr: fprintf(stderr, ...), stdout читает судья.
#include <stdio.h>

int main(void) {
    int low, high, bonus, n, opponent;
    // ← нижняя граница L, верхняя граница U, бонус и штраф R, число итераций
    if (scanf("%d %d %d %d", &low, &high, &bonus, &n) != 4) return 1;
    int claim = 90;                                     // 90, если оно внутри [L, U]
    if (claim < low) claim = low;
    if (claim > high) claim = high;
    for (int i = 0; i < n; i++) {
        printf("%d\n", claim);                          // → заявка: целое число от L до U
        fflush(stdout);                                 // сброс вывода обязателен
        if (scanf("%d", &opponent) != 1) return 1;      // ← заявка соперника
    }
    return 0;
}
