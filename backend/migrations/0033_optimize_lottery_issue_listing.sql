-- Serve the admin's newest-issue pagination directly from an ordered index.
-- LOCK=NONE keeps issue creation and settlement available while MySQL builds
-- the secondary index on the production table.

ALTER TABLE lottery_issues
    ADD INDEX idx_lottery_issues_recent (draw_at DESC, id DESC),
    ALGORITHM=INPLACE,
    LOCK=NONE;
