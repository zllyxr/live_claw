ALTER TABLE lottery_issues
    DROP INDEX idx_lottery_issues_recent,
    ALGORITHM=INPLACE,
    LOCK=NONE;

DELETE FROM schema_migrations
WHERE version='0033_optimize_lottery_issue_listing.sql';
