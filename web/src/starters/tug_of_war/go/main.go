// Перетягивание каната: стратегия «равная трата».
// Остаток энергии делится поровну на оставшиеся итерации, к концу тратится весь.
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
	var energy, n, opponent int
	// ← начальная энергия, затем число итераций
	if _, err := fmt.Fscan(in, &energy, &n); err != nil {
		return
	}
	for i := 0; i < n; i++ {
		spend := energy / (n - i) // поровну на оставшиеся итерации
		energy -= spend
		// os.Stdout без буфера: Println отправляет строку сразу
		fmt.Println(spend)                                  // → трата: от 0 до остатка
		if _, err := fmt.Fscan(in, &opponent); err != nil { // ← трата соперника
			return
		}
	}
}
