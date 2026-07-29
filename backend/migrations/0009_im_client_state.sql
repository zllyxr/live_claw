-- Backend v2: per-member client state used by the native Go IM adapter.

ALTER TABLE im_conversation_members
    ADD COLUMN is_hidden tinyint unsigned NOT NULL DEFAULT 0
        AFTER last_read_seq,
    ADD KEY idx_im_members_inbox (user_id, member_status, is_hidden, conversation_id);
