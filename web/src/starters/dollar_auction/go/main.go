// Аукцион двойной цены: стратегия «пас после порога».
// Перебивать соперника на 1, пока ставка не выше половины приза, дальше пас.
// Протокол: ← строка от судьи (stdin), → ваша строка (stdout).
// Ходы поочерёдные: сначала ставка соперника, потом ваш ответ. О пасе соперника
// судья не сообщает: ввод просто заканчивается.
// Отладку печатайте в os.Stderr, stdout читает судья.
package main

import (
	"bufio"
	"fmt"
	"os"
)

func main() {
	in := bufio.NewReader(os.Stdin)
	var prize, maxIters, opponent int
	// ← приз P и максимум итераций
	if _, err := fmt.Fscan(in, &prize, &maxIters); err != nil {
		return
	}
	limit := prize / 2 // дороже половины приза не торговаться
	for {
		// ← ставка соперника, 0 в начале торгов
		if _, err := fmt.Fscan(in, &opponent); err != nil {
			return
		}
		bid := opponent + 1 // перебить на 1
		if bid > limit {
			bid = 0 // 0 = пас, торги заканчиваются
		}
		// os.Stdout без буфера: Println отправляет строку сразу
		fmt.Println(bid) // → ставка выше соперника или 0
	}
}
