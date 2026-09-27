// Аукцион двойной цены: стратегия «пас после порога».
// Перебивать ставку соперника на 1, пока своя ставка не выше половины приза, дальше пас.
// Протокол: ← строка от судьи (stdin), → ваша строка (stdout).
// Здесь ходы поочерёдные: сначала приходит ставка соперника, потом ваш ответ.
// Отладку печатайте в System.err, stdout читает судья.
import java.io.BufferedReader;
import java.io.IOException;
import java.io.InputStreamReader;

public class Main {
    public static void main(String[] args) throws IOException {
        BufferedReader in = new BufferedReader(new InputStreamReader(System.in));
        int prize = Integer.parseInt(in.readLine().trim());       // ← стоимость приза P
        int rounds = Integer.parseInt(in.readLine().trim());      // ← максимум раундов торгов
        int limit = prize / 2;                                    // дороже половины приза не торговаться
        String line;
        while ((line = in.readLine()) != null) {                  // ← ставка соперника (0 в начале торгов)
            int bid = Integer.parseInt(line.trim()) + 1;          // перебить на 1
            if (bid > limit) bid = 0;                             // 0 = пас, торги заканчиваются
            System.out.println(bid);                              // → ставка: больше ставки соперника или 0
            System.out.flush();                                   // сброс вывода обязателен
        }
    }
}
