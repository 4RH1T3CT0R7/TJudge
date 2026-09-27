// Дилемма путешественника: стратегия «заявка 90».
// Каждую итерацию одна и та же высокая заявка, но внутри границ [L, U].
// Протокол: ← строка от судьи (stdin), → ваша строка (stdout).
// Отладку печатайте в System.err, stdout читает судья.
import java.io.BufferedReader;
import java.io.IOException;
import java.io.InputStreamReader;

public class Main {
    public static void main(String[] args) throws IOException {
        BufferedReader in = new BufferedReader(new InputStreamReader(System.in));
        int low = Integer.parseInt(in.readLine().trim());         // ← нижняя граница L
        int high = Integer.parseInt(in.readLine().trim());        // ← верхняя граница U
        int bonus = Integer.parseInt(in.readLine().trim());       // ← бонус и штраф R
        int n = Integer.parseInt(in.readLine().trim());           // ← число итераций
        int claim = Math.min(Math.max(90, low), high);            // 90, если оно внутри [L, U]
        for (int i = 0; i < n; i++) {
            System.out.println(claim);                            // → заявка: целое число от L до U
            System.out.flush();                                   // сброс вывода обязателен
            int opponent = Integer.parseInt(in.readLine().trim()); // ← заявка соперника
        }
    }
}
