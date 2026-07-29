-- Repair catalogs imported by a mysql client that used latin1 while reading the
-- UTF-8 seed file. The binary collation keeps the detector accent-sensitive, so
-- already-correct rows are left untouched and the migration remains idempotent.

UPDATE lottery_categories
SET name = CONVERT(
    CAST(CONVERT(name USING latin1) AS BINARY)
    USING utf8mb4
)
WHERE name COLLATE utf8mb4_bin REGEXP '[ãäåæçèé]';

UPDATE lottery_games
SET name = CONVERT(
        CAST(CONVERT(name USING latin1) AS BINARY)
        USING utf8mb4
    ),
    config = CONVERT(
        CAST(CONVERT(CAST(config AS CHAR CHARACTER SET utf8mb4) USING latin1) AS BINARY)
        USING utf8mb4
    )
WHERE name COLLATE utf8mb4_bin REGEXP '[ãäåæçèé]';

UPDATE lottery_plays
SET name = CONVERT(
        CAST(CONVERT(name USING latin1) AS BINARY)
        USING utf8mb4
    ),
    config = CONVERT(
        CAST(CONVERT(CAST(config AS CHAR CHARACTER SET utf8mb4) USING latin1) AS BINARY)
        USING utf8mb4
    )
WHERE name COLLATE utf8mb4_bin REGEXP '[ãäåæçèé]';

UPDATE lottery_options
SET name = CONVERT(
        CAST(CONVERT(name USING latin1) AS BINARY)
        USING utf8mb4
    ),
    config = CASE
        WHEN config IS NULL THEN NULL
        ELSE CONVERT(
            CAST(CONVERT(CAST(config AS CHAR CHARACTER SET utf8mb4) USING latin1) AS BINARY)
            USING utf8mb4
        )
    END
WHERE name COLLATE utf8mb4_bin REGEXP '[ãäåæçèé]';
