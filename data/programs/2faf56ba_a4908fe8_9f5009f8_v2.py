#!/usr/bin/env python3
'''
Лыткин Артём Витальевич ИУ7-16Б
ЛР №14: бинарная бд

программа для работы с бинарной базой данных
'''

import os
import struct

folder = "."

# Формат одной записи БД (фиксированный размер)
# name: 20 bytes, count: int, price: float, category: 15 bytes
FORMAT = "20s i f 15s"
RECORD_SIZE = struct.calcsize(FORMAT)


def pack_record(name: str, count: int, price: float, category: str) -> bytes:
    return struct.pack(
        FORMAT,
        name.encode("utf-8")[:20].ljust(20, b"\x00"),
        int(count),
        float(price),
        category.encode("utf-8")[:15].ljust(15, b"\x00"),
    )


def unpack_record(data: bytes):
    name_b, count, price, category_b = struct.unpack(FORMAT, data)
    name = name_b.decode("utf-8", errors="ignore").rstrip("\x00")
    category = category_b.decode("utf-8", errors="ignore").rstrip("\x00")
    return name, count, price, category

def list_files():
    return [f for f in os.listdir(folder) if os.path.isfile(os.path.join(folder, f))]

def generate_record(index: int):
    # Возвращает одну запись по индексу (0..), либо None если записей больше нет
    if index == 0:
        return ("яблоко", 10, 3.5, "фрукт")
    if index == 1:
        return ("книга", 2, 799.0, "товар")
    if index == 2:
        return ("ноутбук", 1, 89999.99, "техника")
    return None

def init_db_binary(filename):
    # Инициализация БД: создать/перезаписать файл и заполнить бинарными записями фиксированного размера
    count_records = 0

    with open(filename, "wb") as f:
        i = 0
        while True:
            rec = generate_record(i)
            if rec is None:
                break
            f.write(pack_record(*rec))
            count_records += 1
            i += 1

    print(
        f"Файл '{filename}' инициализирован бинарной БД ({count_records} записей, размер записи {RECORD_SIZE} байт)"
    )


def choose_any_file():
    """
    Выбор произвольного файла на диске.
    Если файл существует — он будет перезаписан и инициализирован базой.
    Если файла нет — по запросу создаётся новый и инициализируется базой.
    """
    while True:
        path = input("Введите полный путь к файлу БД (или оставьте пустым для отмены): ").strip()

        if not path:
            print("Выбор файла отменён.")
            return None

        # Раскрываем ~ и относительные пути
        path = os.path.expanduser(path)
        path = os.path.abspath(path)

        # Если указали папку вместо файла
        if os.path.isdir(path):
            print("Указан каталог, а не файл. Введите путь именно к файлу (с именем файла).")
            continue

        # Если файл уже существует — используем его и инициализируем базой
        if os.path.exists(path):
            print(f"Выбран существующий файл: {path}")
            init_db_binary(path)
            return path

        # Файл не существует — предложим создать и инициализировать
        ans = input(f"Файл '{path}' не найден. Создать новый и инициализировать БД? (y/n): ").strip().lower()
        if ans == "y":
            init_db_binary(path)
            return path
        else:
            print("Попробуйте ввести путь ещё раз.")

def choose_file():
    print("Выбор файла базы данных:")
    print("1. Выбрать файл из текущего каталога")
    print("2. Ввести полный путь к файлу")

    try:
        mode = int(input("Выберите способ выбора файла (1-2): "))
    except ValueError:
        print("Ошибка: введите 1 или 2.")
        return None

    if mode == 1:
        files = list_files()

        print("\nСписок файлов:")
        for i, f in enumerate(files, start=1):
            print(f"{i}. {f}")
        print(f"{len(files) + 1}. Создать новый файл")

        choice = int(input("\nВыберите пункт: "))

        # Выбор существующего
        if 1 <= choice <= len(files):
            selected = files[choice - 1]
            print(f"Вы выбрали существующий файл: {selected}")

            init_db_binary(selected)
            return selected

        # Создание нового
        elif choice == len(files) + 1:
            filename = input("Введите имя нового файла: ")

            full_path = os.path.join(folder, filename)

            if os.path.exists(full_path):
                print("Файл существует. Он будет перезаписан.")

            init_db_binary(full_path)
            return full_path

        else:
            print("Некорректный выбор.")
            return None

    elif mode == 2:
        return choose_any_file()

    else:
        print("Некорректный выбор.")
        return None

def print_db_from_file(filename):
    print(f"\nСодержимое БД '{filename}':")
    if not os.path.exists(filename):
        print("Файл не найден")
        return

    size = os.path.getsize(filename)
    if size % RECORD_SIZE != 0:
        print(f"Предупреждение: размер файла {size} байт не кратен размеру записи {RECORD_SIZE} байт")

    with open(filename, "rb") as f:
        idx = 1
        while True:
            data = f.read(RECORD_SIZE)
            if not data:
                break
            if len(data) != RECORD_SIZE:
                print(f"[{idx}] Повреждённая/неполная запись ({len(data)} байт)")
                break
            name, count, price, category = unpack_record(data)
            print(f"{idx}. {name}, {count}, {price}, {category}")
            idx += 1


def insert_record_at(filename, position_1based, name, count, price, category):
    # position_1based: 1..n+1
    if not os.path.exists(filename):
        print("Файл не найден")
        return

    pos = int(position_1based)
    if pos < 1:
        print("Некорректная позиция")
        return

    file_size = os.path.getsize(filename)
    n = file_size // RECORD_SIZE
    if file_size % RECORD_SIZE != 0:
        print("Файл имеет некорректный размер (не кратен размеру записи). Вставка отменена.")
        return

    if pos > n + 1:
        print(f"Некорректная позиция. Допустимо 1..{n + 1}")
        return

    new_data = pack_record(name, count, price, category)

    with open(filename, "rb+") as f:
        # Если вставка в конец — просто дописываем
        if pos == n + 1:
            f.seek(0, 2)
            f.write(new_data)
            print("Запись добавлена в конец")
            return

        # Сдвигаем хвост на 1 запись вправо, начиная с конца
        for i in range(n - 1, pos - 2, -1):
            f.seek(i * RECORD_SIZE)
            chunk = f.read(RECORD_SIZE)
            f.seek((i + 1) * RECORD_SIZE)
            f.write(chunk)

        # Пишем новую запись на нужную позицию
        f.seek((pos - 1) * RECORD_SIZE)
        f.write(new_data)

    print(f"Запись вставлена на позицию {pos}")

def delete_record(filename, position_1based):
    if not os.path.exists(filename):
        print("Файл не найден")
        return

    try:
        pos = int(position_1based)
    except ValueError:
        print("Некорректная позиция")
        return

    if pos < 1:
        print("Некорректная позиция")
        return

    file_size = os.path.getsize(filename)
    if file_size % RECORD_SIZE != 0:
        print("Файл имеет некорректный размер (не кратен размеру записи). Удаление отменено.")
        return

    n = file_size // RECORD_SIZE
    if n == 0:
        print("База пуста")
        return

    # Удалять можно только существующую запись: 1..n
    if pos > n:
        print(f"Некорректная позиция. Допустимо 1..{n}")
        return

    with open(filename, "rb+") as f:
        # Сдвигаем все записи после удаляемой на одну позицию влево
        for i in range(pos, n):
            f.seek(i * RECORD_SIZE)
            chunk = f.read(RECORD_SIZE)
            f.seek((i - 1) * RECORD_SIZE)
            f.write(chunk)

        # Обрезаем файл на одну запись
        f.truncate((n - 1) * RECORD_SIZE)

    print(f"Запись удалена с позиции {pos}")

def one_field_search(filename, field_num, value):
    """Поиск по одному полю без хранения БД в памяти.

    field_num:
        1 - name
        2 - count
        3 - price
        4 - category
    value: искомое значение (строка; для чисел будет преобразовано)
    """
    if not os.path.exists(filename):
        print("Файл не найден")
        return

    file_size = os.path.getsize(filename)
    if file_size % RECORD_SIZE != 0:
        print("Файл имеет некорректный размер (не кратен размеру записи). Поиск отменён.")
        return

    n = file_size // RECORD_SIZE
    if n == 0:
        print("База пуста")
        return

    try:
        field_num = int(field_num)
    except ValueError:
        print("Номер поля должен быть числом 1-4")
        return

    if field_num not in (1, 2, 3, 4):
        print("Некорректный номер поля. Допустимо 1-4")
        return

    found = False

    # Важно: открываем только для чтения (rb), потому что поиск не меняет файл
    with open(filename, "rb") as f:
        idx = 1
        while True:
            data = f.read(RECORD_SIZE)
            if not data:
                break
            if len(data) != RECORD_SIZE:
                print(f"[{idx}] Повреждённая/неполная запись ({len(data)} байт)")
                break

            name, count, price, category = unpack_record(data)

            # Сравнение строго по выбранному полю
            if field_num == 1:
                ok = (name == value)
            elif field_num == 2:
                try:
                    ok = (count == int(value))
                except ValueError:
                    ok = False
            elif field_num == 3:
                try:
                    ok = (price == float(value))
                except ValueError:
                    ok = False
            else:  # field_num == 4
                ok = (category == value)

            if ok:
                print(f"{idx}. {name}, {count}, {price}, {category}")
                found = True

            idx += 1

    if not found:
        print("Совпадений не найдено")

def two_fields_search(filename, field1_num, value1, field2_num, value2):
    if not os.path.exists(filename):
        print("Файл не найден")
        return

    try:
        field1_num = int(field1_num)
        field2_num = int(field2_num)
    except ValueError:
        print("Номера полей должны быть числами 1-4")
        return

    if field1_num == field2_num:
        print("Номера полей должны быть разными")
        return

    if field1_num not in (1, 2, 3, 4) or field2_num not in (1, 2, 3, 4):
        print("Некорректный номер поля. Допустимо 1-4")
        return

    file_size = os.path.getsize(filename)
    if file_size % RECORD_SIZE != 0:
        print("Файл имеет некорректный размер (не кратен размеру записи). Поиск отменён.")
        return

    n = file_size // RECORD_SIZE
    if n == 0:
        print("База пуста")
        return

    found = False

    with open(filename, "rb") as f:
        idx = 1
        while True:
            data = f.read(RECORD_SIZE)
            if not data:
                break
            if len(data) != RECORD_SIZE:
                print(f"[{idx}] Повреждённая/неполная запись ({len(data)} байт)")
                break

            name, count, price, category = unpack_record(data)

            # Проверка по 1-му полю
            if field1_num == 1:
                ok1 = (name == value1)
            elif field1_num == 2:
                try:
                    ok1 = (count == int(value1))
                except ValueError:
                    ok1 = False
            elif field1_num == 3:
                try:
                    ok1 = (price == float(value1))
                except ValueError:
                    ok1 = False
            else:
                ok1 = (category == value1)

            # Проверка по 2-му полю
            if field2_num == 1:
                ok2 = (name == value2)
            elif field2_num == 2:
                try:
                    ok2 = (count == int(value2))
                except ValueError:
                    ok2 = False
            elif field2_num == 3:
                try:
                    ok2 = (price == float(value2))
                except ValueError:
                    ok2 = False
            else:
                ok2 = (category == value2)

            if ok1 and ok2:
                print(f"{idx}. {name}, {count}, {price}, {category}")
                found = True

            idx += 1

    if not found:
        print("Совпадений не найдено")


def digit_search(filename, condition):
    if not os.path.exists(filename):
        print("Файл не найден")
        return

    condition = condition.strip()
    if not condition:
        print("Значение переменной для поиска не может быть равно нулю")
        return

    op = condition[0]
    if op not in (">", "<", "="):
        print("Некорректный оператор. Ожидается один из: >, <, =")
        return

    num_str = condition[1:].strip()
    if not num_str:
        print("Число для сравнения не указано")
        return

    # Определяем тип поиска: целое -> по count, дробное -> по price
    try:
        if "." in num_str:
            target = float(num_str)
            numeric_is_float = True
        else:
            target = int(num_str)
            numeric_is_float = False
    except ValueError:
        print("Некорректное число для сравнения")
        return

    file_size = os.path.getsize(filename)
    if file_size % RECORD_SIZE != 0:
        print("Файл имеет некорректный размер (не кратен размеру записи). Поиск отменён.")
        return

    n = file_size // RECORD_SIZE
    if n == 0:
        print("База пуста")
        return

    found = False
    with open(filename, "rb") as f:
        idx = 1
        while True:
            data = f.read(RECORD_SIZE)
            if not data:
                break
            if len(data) != RECORD_SIZE:
                print(f"[{idx}] Повреждённая/неполная запись ({len(data)} байт)")
                break

            name, count, price, category = unpack_record(data)

            if numeric_is_float:
                if op == ">" and price > target:
                    match = True
                elif op == "<" and price < target:
                    match = True
                elif op == "=" and price == target:
                    match = True
                else:
                    match = False
            else:
                if op == ">" and count > target:
                    match = True
                elif op == "<" and count < target:
                    match = True
                elif op == "=" and count == target:
                    match = True
                else:
                    match = False

            if match:
                print(f"{idx}. {name}, {count}, {price}, {category}")
                found = True

            idx += 1

    if not found:
        print("Совпадений не найдено")



# Меню и главный цикл
def menu():
    print("\nМеню:")
    print("1. Выбрать файл для работы")
    print("2. Инициализировать базу данных")
    print("3. Вывести содержимое базы данных")
    print("4. Добавить запись в произвольное место")
    print("5. Удалить запись")
    print("6. Поиск по одному полю")
    print("7. Поиск по двум полям")
    print("8. Поиск по числу")
    print("9. Выход")

    try:
        return int(input("Выберите пункт меню: "))
    except ValueError:
        return -1


def main():
    db_file = None

    while True:
        choice = menu()

        if choice == 1:
            db_file = choose_file()

        elif choice == 2:
            if not db_file:
                print("Сначала выберите файл БД")
            else:
                init_db_binary(db_file)

        elif choice == 3:
            if not db_file:
                print("Сначала выберите файл БД")
            else:
                print_db_from_file(db_file)

        elif choice == 4:
            if not db_file:
                print("Сначала выберите файл БД")
            else:
                try:
                    pos = int(input("Введите позицию для вставки (начиная с 1): "))
                    name = input("Введите имя: ")
                    count = int(input("Введите количество: "))
                    price = float(input("Введите цену: "))
                    category = input("Введите категорию: ")
                    insert_record_at(db_file, pos, name, count, price, category)
                except ValueError:
                    print("Ошибка ввода данных")

        elif choice == 5:
            if not db_file:
                print("Сначала выберите файл БД")
            else:
                try:
                    pos = int(input("Введите номер записи для удаления (начиная с 1): "))
                    delete_record(db_file, pos)
                except ValueError:
                    print("Ошибка ввода данных")

        elif choice == 6:
            if not db_file:
                print("Сначала выберите файл БД")
            else:
                print("Поиск по одному полю:")
                print("1 - name")
                print("2 - count")
                print("3 - price")
                print("4 - category")
                field_num = input("Введите номер поля (1-4): ").strip()
                value = input("Введите значение для поиска: ")
                one_field_search(db_file, field_num, value)

        elif choice == 7:
            if not db_file:
                print("Сначала выберите файл БД")
            else:
                print("Поиск по двум полям:")
                print("1 - name")
                print("2 - count")
                print("3 - price")
                print("4 - category")
                field1 = input("Введите номер 1-го поля (1-4): ").strip()
                value1 = input("Введите значение 1-го поля: ")
                field2 = input("Введите номер 2-го поля (1-4): ").strip()
                value2 = input("Введите значение 2-го поля: ")
                two_fields_search(db_file, field1, value1, field2, value2)

        elif choice == 8:
            if not db_file:
                print("Сначала выберите файл с БД.")
            else:
                print("Поиск по числу:")
                range = input("Введите условие поиска (например, >5, <10, =3.5): ").strip()
                digit_search(db_file, range)

        elif choice == 9:
            print("Выход из программы")
            break

        else:
            print("Некорректный пункт меню")

# Точка входа
if __name__ == "__main__":
    main()
