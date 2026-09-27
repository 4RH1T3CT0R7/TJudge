/* Дилемма заключённого: стратегия «око за око» (tit for tat).
   Первый ход - сотрудничество, дальше повтор прошлого хода соперника.
   Протокол: ← строка от судьи (stdin), → ваша строка (stdout).
   Отладку печатайте в stderr: fprintf(stderr, ...), stdout читает судья. */
#include <stdio.h>

int main(void) {
    int n;
    char move[16] = "COOPERATE";
    if (scanf("%d", &n) != 1) return 1;          /* ← число итераций */
    for (int i = 0; i < n; i++) {
        printf("%s\n", move);                     /* → ваш ход */
        fflush(stdout);                           /* сброс вывода обязателен */
        if (scanf("%15s", move) != 1) return 1;   /* ← ход соперника: в следующий раз ответить тем же */
    }
    return 0;
}
