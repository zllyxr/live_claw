-- Backend v2: lossless audit storage for the local claw_live -> claw_v2 sync.

CREATE TABLE IF NOT EXISTS legacy_entity_snapshots (
    source_name varchar(80) NOT NULL,
    entity_type varchar(50) NOT NULL,
    legacy_id varchar(100) NOT NULL,
    payload json NOT NULL,
    payload_hash char(64) NOT NULL,
    synced_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (source_name, entity_type, legacy_id),
    KEY idx_legacy_snapshot_hash (entity_type, payload_hash)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS sports_score_events (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    source varchar(40) NOT NULL,
    source_event_id varchar(100) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    match_id bigint unsigned NOT NULL,
    source_match_id varchar(100) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
    old_home_score smallint NOT NULL DEFAULT -1,
    old_away_score smallint NOT NULL DEFAULT -1,
    new_home_score smallint NOT NULL DEFAULT -1,
    new_away_score smallint NOT NULL DEFAULT -1,
    old_status varchar(30) NOT NULL DEFAULT '',
    new_status varchar(30) NOT NULL DEFAULT '',
    accepted tinyint unsigned NOT NULL DEFAULT 1,
    reason varchar(100) NOT NULL DEFAULT '',
    raw_payload json NULL,
    occurred_at datetime(3) NOT NULL,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_sports_score_source_event (source, source_event_id),
    KEY idx_sports_score_match_time (match_id, occurred_at),
    CHECK (accepted IN (0, 1))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
