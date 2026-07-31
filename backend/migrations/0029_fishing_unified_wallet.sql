-- Fishing now debits and credits the single COIN wallet for every authoritative
-- shot/result. Historical escrow sessions are closed and reconciled by the
-- scheduler so they can never be resumed as a second balance.

ALTER TABLE game_sessions
    ADD COLUMN wallet_mode tinyint unsigned NOT NULL DEFAULT 1
        COMMENT '0 legacy escrow, 1 direct unified wallet'
        AFTER escrow_hold_no;

-- The migration runs before the direct-wallet binary starts, so every session
-- already present is unambiguously a legacy session even if an older data
-- repair left its hold number empty.
UPDATE game_sessions
SET wallet_mode=0;

UPDATE game_sessions
SET status=4,
    disconnected_at=COALESCE(disconnected_at,CURRENT_TIMESTAMP(3)),
    expires_at=LEAST(expires_at,CURRENT_TIMESTAMP(3))
WHERE wallet_mode=0 AND status IN (1,2);

ALTER TABLE game_sessions
    ADD KEY idx_game_sessions_wallet_mode (wallet_mode,status,id),
    ADD CONSTRAINT chk_game_sessions_wallet_mode
        CHECK (wallet_mode=0 OR (wallet_mode=1 AND escrow_hold_no='')),
    MODIFY COLUMN escrow_balance bigint unsigned NOT NULL DEFAULT 0
        COMMENT 'current wallet balance snapshot; legacy rows contain escrow balance';

ALTER TABLE fishing_checkpoints
    MODIFY COLUMN escrow_balance bigint unsigned NOT NULL
        COMMENT 'wallet balance after this event; legacy rows contain escrow balance';

UPDATE game_venues venue
JOIN games game ON game.id=venue.game_id AND game.game_code='deepsea_hunter'
SET venue.escrow_amount=0;

UPDATE games
SET config=JSON_SET(config,'$.wallet_mode','direct')
WHERE game_code='deepsea_hunter';
