-- Create function to automatically create monthly partitions for the rating_history table.
-- This prevents INSERT failures when the current hardcoded partitions expire.
-- Same pattern as create_matches_partition_if_needed() from migration 24.

CREATE OR REPLACE FUNCTION create_rating_history_partition_if_needed()
RETURNS void AS $$
DECLARE
    current_month DATE := date_trunc('month', now());
    next_month DATE := date_trunc('month', now() + interval '1 month');
    month_after DATE := date_trunc('month', now() + interval '2 months');
    current_partition TEXT;
    next_partition TEXT;
BEGIN
    -- Partition for current month
    current_partition := 'rating_history_' || to_char(current_month, 'YYYY_MM');
    IF NOT EXISTS (
        SELECT 1 FROM pg_class WHERE relname = current_partition
    ) THEN
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS %I PARTITION OF rating_history FOR VALUES FROM (%L) TO (%L)',
            current_partition, current_month, next_month
        );
        RAISE NOTICE 'Created partition: %', current_partition;
    END IF;

    -- Partition for next month (always stay one month ahead)
    next_partition := 'rating_history_' || to_char(next_month, 'YYYY_MM');
    IF NOT EXISTS (
        SELECT 1 FROM pg_class WHERE relname = next_partition
    ) THEN
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS %I PARTITION OF rating_history FOR VALUES FROM (%L) TO (%L)',
            next_partition, next_month, month_after
        );
        RAISE NOTICE 'Created partition: %', next_partition;
    END IF;
END;
$$ LANGUAGE plpgsql;

-- Create partitions for the current and next months immediately
SELECT create_rating_history_partition_if_needed();
