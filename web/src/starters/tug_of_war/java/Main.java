// Перетягивание каната: стратегия «равная трата».
// Остаток энергии делится поровну на оставшиеся итерации, к концу тратится весь.
// Протокол: ← строка от судьи (stdin), → ваша строка (stdout).
// Отладку печатайте в System.err, stdout читает судья.
import java.io.BufferedReader;
import java.io.IOException;
import java.io.InputStreamReader;

public class Main {
    public static void main(String[] args) throws IOException {
        BufferedReader in = new BufferedReader(new InputStreamReader(System.in));
        int energy = Integer.parseInt(in.readLine().trim());      // ← начальная энергия
        int n = Integer.parseInt(in.readLine().trim());           // ← число итераций
        for (int i = 0; i < n; i++) {
            int spend = energy / (n - i);                         // поровну на оставшиеся итерации
            energy -= spend;
            System.out.println(spend);                            // → сколько потратить: от 0 до остатка
            System.out.flush();                                   // сброс вывода обязателен
            int opponent = Integer.parseInt(in.readLine().trim()); // ← сколько потратил соперник
        }
    }
}
