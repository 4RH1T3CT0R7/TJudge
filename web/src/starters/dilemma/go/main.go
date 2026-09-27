// Дилемма заключённого: стратегия «око за око» (tit for tat).
// Первый ход - сотрудничество, дальше повтор прошлого хода соперника.
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
	var n int
	if _, err := fmt.Fscan(in, &n); err != nil { // ← число итераций
		return
	}
	move := "COOPERATE"
	for i := 0; i < n; i++ {
		// os.Stdout без буфера: Println отправляет строку сразу
		fmt.Println(move)                               // → ваш ход
		if _, err := fmt.Fscan(in, &move); err != nil { // ← ход соперника
			return
		}
	}
}
