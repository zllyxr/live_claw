-- Every authoritative fishing shot has a client command id so reconnects and
-- retries cannot charge the same cannon shot twice.

ALTER TABLE fishing_checkpoints
    ADD COLUMN client_command_id varchar(100) CHARACTER SET ascii COLLATE ascii_bin NULL
        AFTER event_seq,
    ADD UNIQUE KEY uk_fishing_checkpoint_command (session_id, client_command_id);
