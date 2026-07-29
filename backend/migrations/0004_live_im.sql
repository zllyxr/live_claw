-- Backend v2: Douyin-only live rooms and native Go IM.

CREATE TABLE IF NOT EXISTS live_rooms (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    room_no char(26) NOT NULL,
    host_user_id bigint unsigned NOT NULL,
    title varchar(300) NOT NULL DEFAULT '',
    category varchar(60) NOT NULL DEFAULT '',
    cover_asset_id bigint unsigned NOT NULL DEFAULT 0,
    provider varchar(20) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'douyin',
    provider_room_id varchar(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    provider_page varchar(1000) NOT NULL,
    status tinyint unsigned NOT NULL DEFAULT 0 COMMENT '0 offline, 1 online, 2 disabled',
    sort_order int NOT NULL DEFAULT 0,
    last_seen_at datetime(3) NULL,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_live_room_no (room_no),
    UNIQUE KEY uk_live_provider_room (provider, provider_room_id),
    KEY idx_live_rooms_home (status, sort_order, last_seen_at),
    CHECK (provider = 'douyin')
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS douyin_room_profiles (
    live_room_id bigint unsigned NOT NULL,
    nickname varchar(200) NOT NULL DEFAULT '',
    unique_id varchar(200) NOT NULL DEFAULT '',
    avatar_url varchar(1000) NOT NULL DEFAULT '',
    cover_url varchar(1000) NOT NULL DEFAULT '',
    resolution varchar(30) NOT NULL DEFAULT '',
    stream_format varchar(20) NOT NULL DEFAULT 'hls',
    verify_status tinyint unsigned NOT NULL DEFAULT 0 COMMENT '0 pending, 1 approved, 2 rejected',
    verified_by bigint unsigned NOT NULL DEFAULT 0,
    verified_at datetime(3) NULL,
    last_resolve_status tinyint unsigned NOT NULL DEFAULT 0,
    last_resolve_error varchar(500) NOT NULL DEFAULT '',
    last_resolved_at datetime(3) NULL,
    updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (live_room_id),
    KEY idx_douyin_verify (verify_status, updated_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS live_room_managers (
    live_room_id bigint unsigned NOT NULL,
    user_id bigint unsigned NOT NULL,
    created_by bigint unsigned NOT NULL DEFAULT 0,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (live_room_id, user_id),
    KEY idx_live_managers_user (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS live_moderation_actions (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    live_room_id bigint unsigned NOT NULL,
    target_user_id bigint unsigned NOT NULL,
    action_type varchar(30) NOT NULL COMMENT 'mute, unmute, kick, ban, unban',
    reason varchar(500) NOT NULL DEFAULT '',
    expires_at datetime(3) NULL,
    actor_type tinyint unsigned NOT NULL COMMENT '1 admin, 2 room manager, 3 system',
    actor_id bigint unsigned NOT NULL DEFAULT 0,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    KEY idx_live_moderation_target (live_room_id, target_user_id, created_at),
    KEY idx_live_moderation_expiry (action_type, expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS live_gifts (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    gift_key varchar(60) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    name varchar(100) NOT NULL,
    icon_asset_id bigint unsigned NOT NULL DEFAULT 0,
    price_coin bigint unsigned NOT NULL,
    status tinyint unsigned NOT NULL DEFAULT 1,
    sort_order int NOT NULL DEFAULT 0,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_live_gift_key (gift_key),
    KEY idx_live_gifts_status (status, sort_order)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS live_gift_orders (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    order_no char(26) NOT NULL,
    live_room_id bigint unsigned NOT NULL,
    sender_user_id bigint unsigned NOT NULL,
    receiver_user_id bigint unsigned NOT NULL,
    gift_id bigint unsigned NOT NULL,
    gift_count int unsigned NOT NULL,
    unit_price_coin bigint unsigned NOT NULL,
    total_coin bigint unsigned NOT NULL,
    ledger_entry_no char(26) NULL,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_live_gift_order_no (order_no),
    UNIQUE KEY uk_live_gift_ledger (ledger_entry_no),
    KEY idx_live_gift_room_time (live_room_id, created_at),
    KEY idx_live_gift_sender_time (sender_user_id, created_at),
    KEY idx_live_gift_receiver_time (receiver_user_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS live_red_packets (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    packet_no char(26) NOT NULL,
    live_room_id bigint unsigned NOT NULL,
    sender_user_id bigint unsigned NOT NULL,
    total_coin bigint unsigned NOT NULL,
    packet_count int unsigned NOT NULL,
    claimed_coin bigint unsigned NOT NULL DEFAULT 0,
    claimed_count int unsigned NOT NULL DEFAULT 0,
    hold_no char(26) NOT NULL,
    status tinyint unsigned NOT NULL DEFAULT 0 COMMENT '0 active, 1 finished, 2 expired, 3 cancelled',
    expires_at datetime(3) NOT NULL,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_live_red_packet_no (packet_no),
    UNIQUE KEY uk_live_red_packet_hold (hold_no),
    KEY idx_live_red_packets_room (live_room_id, status, created_at),
    KEY idx_live_red_packets_expiry (status, expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS live_red_packet_claims (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    packet_id bigint unsigned NOT NULL,
    user_id bigint unsigned NOT NULL,
    amount_coin bigint unsigned NOT NULL,
    ledger_entry_no char(26) NOT NULL,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_live_red_packet_user (packet_id, user_id),
    UNIQUE KEY uk_live_red_packet_ledger (ledger_entry_no),
    KEY idx_live_red_claims_user (user_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS im_conversations (
    id char(26) NOT NULL,
    conversation_type tinyint unsigned NOT NULL COMMENT '1 single, 2 group, 3 live',
    direct_key varchar(100) CHARACTER SET ascii COLLATE ascii_bin NULL,
    title varchar(200) NOT NULL DEFAULT '',
    avatar_asset_id bigint unsigned NOT NULL DEFAULT 0,
    message_seq bigint unsigned NOT NULL DEFAULT 0,
    status tinyint unsigned NOT NULL DEFAULT 1,
    created_by bigint unsigned NOT NULL DEFAULT 0,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_im_direct_key (direct_key),
    KEY idx_im_conversations_type (conversation_type, status, updated_at),
    CHECK (conversation_type IN (1, 2, 3))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS im_conversation_members (
    conversation_id char(26) NOT NULL,
    user_id bigint unsigned NOT NULL,
    role tinyint unsigned NOT NULL DEFAULT 10 COMMENT '10 member, 60 admin, 100 owner',
    member_status tinyint unsigned NOT NULL DEFAULT 1 COMMENT '0 pending, 1 active, 2 left, 3 removed',
    mute_until datetime(3) NULL,
    last_read_seq bigint unsigned NOT NULL DEFAULT 0,
    joined_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    left_at datetime(3) NULL,
    PRIMARY KEY (conversation_id, user_id),
    KEY idx_im_members_user (user_id, member_status, conversation_id),
    KEY idx_im_members_mute (conversation_id, mute_until)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS im_groups (
    conversation_id char(26) NOT NULL,
    group_no char(26) NOT NULL,
    owner_user_id bigint unsigned NOT NULL,
    introduction varchar(1000) NOT NULL DEFAULT '',
    announcement varchar(2000) NOT NULL DEFAULT '',
    join_policy tinyint unsigned NOT NULL DEFAULT 1 COMMENT '1 approval, 2 open, 3 invite only',
    all_muted tinyint unsigned NOT NULL DEFAULT 0,
    max_members int unsigned NOT NULL DEFAULT 500,
    member_count int unsigned NOT NULL DEFAULT 0,
    dissolved_at datetime(3) NULL,
    updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (conversation_id),
    UNIQUE KEY uk_im_group_no (group_no)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS im_messages (
    id char(26) NOT NULL,
    conversation_id char(26) NOT NULL,
    sequence bigint unsigned NOT NULL,
    client_message_id varchar(100) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    sender_user_id bigint unsigned NOT NULL,
    message_type tinyint unsigned NOT NULL COMMENT '1 text, 2 image, 3 voice, 4 video, 5 file, 100 system',
    text_content varchar(5000) NOT NULL DEFAULT '',
    asset_id bigint unsigned NOT NULL DEFAULT 0,
    metadata json NULL,
    status tinyint unsigned NOT NULL DEFAULT 1 COMMENT '1 normal, 2 revoked, 3 deleted by admin',
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    revoked_at datetime(3) NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_im_message_seq (conversation_id, sequence),
    UNIQUE KEY uk_im_client_message (sender_user_id, client_message_id),
    KEY idx_im_messages_sender (sender_user_id, created_at),
    KEY idx_im_messages_created (created_at, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS im_group_applications (
    id char(26) NOT NULL,
    conversation_id char(26) NOT NULL,
    applicant_user_id bigint unsigned NOT NULL,
    request_message varchar(500) NOT NULL DEFAULT '',
    status tinyint unsigned NOT NULL DEFAULT 0 COMMENT '0 pending, 1 accepted, 2 rejected, 3 cancelled',
    handled_by bigint unsigned NOT NULL DEFAULT 0,
    handle_message varchar(500) NOT NULL DEFAULT '',
    handled_at datetime(3) NULL,
    pending_key varchar(100) CHARACTER SET ascii COLLATE ascii_bin
        GENERATED ALWAYS AS (
            CASE WHEN status = 0 THEN CONCAT(conversation_id, ':', applicant_user_id) ELSE NULL END
        ) STORED,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_im_group_pending_application (pending_key),
    KEY idx_im_group_applications_pick (conversation_id, status, created_at),
    KEY idx_im_group_applications_user (applicant_user_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS im_moderation_actions (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    conversation_id char(26) NOT NULL,
    target_user_id bigint unsigned NOT NULL DEFAULT 0,
    message_id char(26) NOT NULL DEFAULT '',
    action_type varchar(30) NOT NULL,
    reason varchar(500) NOT NULL DEFAULT '',
    actor_type tinyint unsigned NOT NULL,
    actor_id bigint unsigned NOT NULL DEFAULT 0,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    KEY idx_im_moderation_conversation (conversation_id, created_at),
    KEY idx_im_moderation_target (target_user_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
