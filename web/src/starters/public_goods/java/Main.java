// Общественное благо: стратегия «условный вклад».
// Сначала вложить всё, дальше столько же, сколько соперник вложил в прошлый раз.
// Протокол: ← строка от судьи (stdin), → ваша строка (stdout).
// Отладку печатайте в System.err, stdout читает судья.
import java.io.BufferedReader;
import java.io.IOException;
import java.io.InputStreamReader;

public class Main {
    public static void main(String[] args) throws IOException {
        BufferedReader in = new BufferedReader(new InputStreamReader(System.in));
        int capital = Integer.parseInt(in.readLine().trim());  // ← капитал E
        double multiplier = Double.parseDouble(in.readLine()); // ← множитель пула m
        int n = Integer.parseInt(in.readLine().trim());        // ← число итераций
        int give = capital; // первый вклад - весь капитал
        for (int i = 0; i < n; i++) {
            System.out.println(give);                       // → вклад: целое от 0 до E
            System.out.flush();                             // сброс вывода обязателен
            give = Integer.parseInt(in.readLine().trim());  // ← вклад соперника
        }
    }
}
