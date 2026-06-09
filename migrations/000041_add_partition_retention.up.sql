-- Retention помесячных партиций matches и rating_history.
--
-- Создание партиций автоматизировано (000024/000025), но удаления не было -
-- рост неограничен, а старые партиции замедляют запросы по всей таблице.
--
-- drop_old_partitions(parent, retention_months) отсоединяет и удаляет
-- партиции старше retention_months. Вызывается из горутины обслуживания
-- партиций (internal/infrastructure/db/db.go) при PARTITION_RETENTION_MONTHS > 0;
-- по умолчанию retention ВЫКЛЮЧЕН - удаление турнирных данных должно быть
-- осознанным решением оператора.

CREATE OR REPLACE FUNCTION drop_old_partitions(parent_table TEXT, retention_months INT)
RETURNS INT AS $$
DECLARE
    part RECORD;
    cutoff DATE;
    part_month DATE;
    dropped INT := 0;
BEGIN
    IF retention_months <= 0 THEN
        RETURN 0;
    END IF;

    cutoff := date_trunc('month', NOW())::date - make_interval(months => retention_months);

    FOR part IN
        SELECT c.relname AS partition_name
        FROM pg_inherits i
        JOIN pg_class c ON c.oid = i.inhrelid
        JOIN pg_class p ON p.oid = i.inhparent
        WHERE p.relname = parent_table
    LOOP
        -- Имена партиций: <parent>_YYYY_MM (см. create_*_partition_if_needed).
        BEGIN
            part_month := to_date(right(part.partition_name, 7), 'YYYY_MM');
        EXCEPTION WHEN others THEN
            CONTINUE; -- партиция с нестандартным именем - не трогаем
        END;

        IF part_month < cutoff THEN
            EXECUTE format('ALTER TABLE %I DETACH PARTITION %I', parent_table, part.partition_name);
            EXECUTE format('DROP TABLE %I', part.partition_name);
            dropped := dropped + 1;
            RAISE NOTICE 'Dropped old partition %', part.partition_name;
        END IF;
    END LOOP;

    RETURN dropped;
END;
$$ LANGUAGE plpgsql;
