// Дилемма заключённого: стратегия «око за око» (tit for tat).
// Первый ход - сотрудничество, дальше повтор прошлого хода соперника.
// Протокол: ← строка от судьи (stdin), → ваша строка (stdout).
// Отладку печатайте в System.err, stdout читает судья.
import java.io.BufferedReader;
import java.io.IOException;
import java.io.InputStreamReader;

public class Main {
    public static void main(String[] args) throws IOException {
        BufferedReader in = new BufferedReader(new InputStreamReader(System.in));
        int n = Integer.parseInt(in.readLine().trim());   // ← число итераций
        String move = "COOPERATE";
        for (int i = 0; i < n; i++) {
            System.out.println(move);                     // → ваш ход
            System.out.flush();                           // сброс вывода обязателен
            move = in.readLine().trim();                  // ← ход соперника: в следующий раз ответить тем же
        }
    }
}
