// Аукцион двойной цены: стратегия «пас после порога».
// Перебивать соперника на 1, пока ставка не выше половины приза, дальше пас.
// Протокол: ← строка от судьи (stdin), → ваша строка (stdout).
// Ходы поочерёдные: сначала ставка соперника, потом ваш ответ. О пасе соперника
// судья не сообщает: ввод просто заканчивается.
// Отладку печатайте в System.err, stdout читает судья.
import java.io.BufferedReader;
import java.io.IOException;
import java.io.InputStreamReader;

public class Main {
    public static void main(String[] args) throws IOException {
        BufferedReader in = new BufferedReader(new InputStreamReader(System.in));
        int prize = Integer.parseInt(in.readLine().trim());     // ← приз P
        int maxIters = Integer.parseInt(in.readLine().trim());  // ← максимум итераций
        int limit = prize / 2; // дороже половины приза не торговаться
        String line;
        // ← ставка соперника, 0 в начале торгов
        while ((line = in.readLine()) != null) {
            int bid = Integer.parseInt(line.trim()) + 1; // перебить на 1
            if (bid > limit) bid = 0;           // 0 = пас, торги заканчиваются
            System.out.println(bid);            // → ставка выше соперника или 0
            System.out.flush();                 // сброс вывода обязателен
        }
    }
}
