/* Общественное благо: стратегия «условный вклад».
   Сначала вложить всё, дальше столько же, сколько соперник вложил в прошлый раз.
   Протокол: ← строка от судьи (stdin), → ваша строка (stdout).
   Отладку печатайте в stderr: fprintf(stderr, ...), stdout читает судья. */
#include <stdio.h>

int main(void) {
    int capital, n;
    double multiplier;
    /* ← капитал на итерацию E, множитель пула m (дробное число), число итераций */
    if (scanf("%d %lf %d", &capital, &multiplier, &n) != 3) return 1;
    int give = capital;                             /* первый вклад - весь капитал */
    for (int i = 0; i < n; i++) {
        printf("%d\n", give);                       /* → вклад: целое число от 0 до E */
        fflush(stdout);                             /* сброс вывода обязателен */
        if (scanf("%d", &give) != 1) return 1;      /* ← вклад соперника: в следующий раз вложить столько же */
    }
    return 0;
}
