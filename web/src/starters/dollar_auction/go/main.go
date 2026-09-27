// Аукцион двойной цены: стратегия «пас после порога».
// Перебивать ставку соперника на 1, пока своя ставка не выше половины приза, дальше пас.
// Протокол: ← строка от судьи (stdin), → ваша строка (stdout).
// Здесь ходы поочерёдные: сначала приходит ставка соперника, потом ваш ответ.
// Отладку печатайте в os.Stderr, stdout читает судья.
package main

import (
	"bufio"
	"fmt"
	"os"
)

func main() {
	in := bufio.NewReader(os.Stdin)
	var prize, rounds, opponent int
	if _, err := fmt.Fscan(in, &prize, &rounds); err != nil { // ← стоимость приза P и максимум раундов торгов
		return
	}
	limit := prize / 2 // дороже половины приза не торговаться
	for {
		if _, err := fmt.Fscan(in, &opponent); err != nil { // ← ставка соперника (0 в начале торгов)
			return
		}
		bid := opponent + 1 // перебить на 1
		if bid > limit {
			bid = 0 // 0 = пас, торги заканчиваются
		}
		fmt.Println(bid) // → ставка: больше ставки соперника или 0; os.Stdout без буфера
	}
}
