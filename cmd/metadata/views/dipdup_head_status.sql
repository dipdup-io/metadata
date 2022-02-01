CREATE OR REPLACE VIEW dipdup_head_status AS
SELECT
    index_name,
    CASE
        WHEN timestamp < NOW() - interval '3 minutes' THEN 'OUTDATED'
        ELSE 'OK'
    END AS status,
    created_at,
    updated_at
FROM
    dipdup_state;
