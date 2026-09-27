// Общественное благо: стратегия «условный вклад».
// Сначала вложить всё, дальше столько же, сколько соперник вложил в прошлый раз.
// Протокол: ← строка от судьи (stdin), → ваша строка (stdout).
// Отладку печатайте в os.Stderr, stdout читает судья.
package main

import (
	"bufio"
	"fmt"
	"os"
)

func main() {
	in := bufio.NewReader(os.Stdin)
	var capital, n int
	var multiplier float64
	// ← капитал на итерацию E, множитель пула m (дробное число), число итераций
	if _, err := fmt.Fscan(in, &capital, &multiplier, &n); err != nil {
		return
	}
	give := capital // первый вклад - весь капитал
	for i := 0; i < n; i++ {
		fmt.Println(give)                               // → вклад: целое число от 0 до E; os.Stdout без буфера
		if _, err := fmt.Fscan(in, &give); err != nil { // ← вклад соперника: в следующий раз вложить столько же
			return
		}
	}
}
