-- Nginx request IDs are 32 characters while internally generated IDs are 20.
-- Keep the full upstream value so audited mutations do not fail under Nginx.
ALTER TABLE audit_logs
    MODIFY COLUMN request_id varchar(100) NOT NULL;
