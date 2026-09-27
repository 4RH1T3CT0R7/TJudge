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
		fmt.Println(move)                               // → ваш ход; os.Stdout без буфера, сброс не нужен
		if _, err := fmt.Fscan(in, &move); err != nil { // ← ход соперника: в следующий раз ответить тем же
			return
		}
	}
}
