// Дилемма путешественника: стратегия «заявка 90».
// Каждую итерацию одна и та же высокая заявка, но внутри границ [L, U].
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
	var low, high, bonus, n, opponent int
	// ← нижняя граница L, верхняя граница U, бонус и штраф R, число итераций
	if _, err := fmt.Fscan(in, &low, &high, &bonus, &n); err != nil {
		return
	}
	claim := min(max(90, low), high) // 90, если оно внутри [L, U]
	for i := 0; i < n; i++ {
		fmt.Println(claim)                                  // → заявка: целое число от L до U; os.Stdout без буфера
		if _, err := fmt.Fscan(in, &opponent); err != nil { // ← заявка соперника
			return
		}
	}
}
