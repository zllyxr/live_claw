-- API-Football fixtures without a usable decimal price are not catalog data.
-- Remove existing empty fixtures and let the scheduler enforce the same rule
-- after every successful odds refresh.

CREATE TEMPORARY TABLE purge_sports_matches_without_odds (
    id bigint unsigned NOT NULL,
    PRIMARY KEY (id)
) ENGINE=MEMORY;

INSERT INTO purge_sports_matches_without_odds(id)
SELECT match_row.id
FROM sports_matches match_row
WHERE match_row.source='api-football'
  AND NOT EXISTS (
      SELECT 1
      FROM sports_markets market
      JOIN sports_market_options option_row
        ON option_row.market_id=market.id
       AND option_row.odds_scaled>1000000
      WHERE market.match_id=match_row.id
  )
  AND NOT EXISTS (
      SELECT 1
      FROM sports_bet_orders bet_order
      WHERE bet_order.match_id=match_row.id
  );

DELETE settlement
FROM sports_settlement_runs settlement
JOIN purge_sports_matches_without_odds target ON target.id=settlement.match_id;

DELETE score_event
FROM sports_score_events score_event
JOIN purge_sports_matches_without_odds target ON target.id=score_event.match_id;

DELETE option_row
FROM sports_market_options option_row
JOIN sports_markets market ON market.id=option_row.market_id
JOIN purge_sports_matches_without_odds target ON target.id=market.match_id;

DELETE market
FROM sports_markets market
JOIN purge_sports_matches_without_odds target ON target.id=market.match_id;

DELETE match_row
FROM sports_matches match_row
JOIN purge_sports_matches_without_odds target ON target.id=match_row.id;

DROP TEMPORARY TABLE purge_sports_matches_without_odds;
