-- Sports data may only come from API-Football or an explicit administrator entry.
-- Purge every legacy/fallback provider and block it from being inserted again.

CREATE TEMPORARY TABLE purge_sports_matches (
    id bigint unsigned NOT NULL,
    PRIMARY KEY (id)
) ENGINE=MEMORY;

INSERT INTO purge_sports_matches(id)
SELECT id
FROM sports_matches
WHERE source NOT IN ('api-football','manual_admin');

CREATE TEMPORARY TABLE purge_sports_markets (
    id bigint unsigned NOT NULL,
    PRIMARY KEY (id)
) ENGINE=MEMORY;

INSERT INTO purge_sports_markets(id)
SELECT market.id
FROM sports_markets market
JOIN purge_sports_matches match_row ON match_row.id=market.match_id;

CREATE TEMPORARY TABLE purge_sports_orders (
    id bigint unsigned NOT NULL,
    PRIMARY KEY (id)
) ENGINE=MEMORY;

INSERT INTO purge_sports_orders(id)
SELECT bet_order.id
FROM sports_bet_orders bet_order
JOIN purge_sports_matches match_row ON match_row.id=bet_order.match_id;

DELETE bet_item
FROM sports_bet_items bet_item
LEFT JOIN purge_sports_orders bet_order ON bet_order.id=bet_item.order_id
LEFT JOIN purge_sports_markets market ON market.id=bet_item.market_id
WHERE bet_order.id IS NOT NULL OR market.id IS NOT NULL;

DELETE bet_order
FROM sports_bet_orders bet_order
JOIN purge_sports_orders target ON target.id=bet_order.id;

DELETE settlement
FROM sports_settlement_runs settlement
JOIN purge_sports_matches match_row ON match_row.id=settlement.match_id;

DELETE score_event
FROM sports_score_events score_event
JOIN purge_sports_matches match_row ON match_row.id=score_event.match_id;

DELETE option_row
FROM sports_market_options option_row
JOIN purge_sports_markets market ON market.id=option_row.market_id;

DELETE market
FROM sports_markets market
JOIN purge_sports_markets target ON target.id=market.id;

DELETE match_row
FROM sports_matches match_row
JOIN purge_sports_matches target ON target.id=match_row.id;

DELETE FROM legacy_id_map
WHERE entity_type IN ('sports_match','sports_score_event','sports_settlement');

DELETE FROM legacy_entity_snapshots
WHERE entity_type IN ('sports_match','sports_score_event','sports_settlement');

DELETE FROM sports_sync_logs
WHERE source<>'api-football';

ALTER TABLE sports_matches
    ADD CONSTRAINT chk_sports_matches_source
    CHECK (source IN ('api-football','manual_admin'));

ALTER TABLE sports_sync_logs
    ADD CONSTRAINT chk_sports_sync_logs_source
    CHECK (source='api-football');

DROP TEMPORARY TABLE purge_sports_orders;
DROP TEMPORARY TABLE purge_sports_markets;
DROP TEMPORARY TABLE purge_sports_matches;
