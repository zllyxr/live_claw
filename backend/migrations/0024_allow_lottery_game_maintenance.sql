-- Allow administrators to place an existing high-frequency lottery game into
-- maintenance and restore it later. Categories, plays and options remain
-- permanently active-only, and migration 0016's pruned low-frequency/TRX data
-- is not recreated.

ALTER TABLE lottery_games
    DROP CHECK chk_lottery_games_active,
    ADD CONSTRAINT chk_lottery_games_status CHECK (status IN (0,1));
