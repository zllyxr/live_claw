-- Backend v2: compatibility state required by the existing mobile frontend.

CREATE TABLE IF NOT EXISTS auth_verification_codes (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    purpose varchar(20) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    target varchar(300) NOT NULL,
    code_hash char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    attempts int unsigned NOT NULL DEFAULT 0,
    expires_at datetime(3) NOT NULL,
    consumed_at datetime(3) NULL,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    KEY idx_auth_code_lookup (purpose, target, consumed_at, expires_at, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

INSERT INTO daily_tasks(task_key,name,description,target_count,reward_coin,status,rules)
VALUES('daily_login','每日登录','每日打开应用即可完成',1,10,1,JSON_OBJECT('event','login'))
ON DUPLICATE KEY UPDATE
    name=VALUES(name),description=VALUES(description),target_count=VALUES(target_count),
    reward_coin=VALUES(reward_coin),status=VALUES(status),rules=VALUES(rules);

CREATE TABLE IF NOT EXISTS live_guards (
    live_room_id bigint unsigned NOT NULL,
    user_id bigint unsigned NOT NULL,
    level tinyint unsigned NOT NULL DEFAULT 1,
    expires_at datetime(3) NOT NULL,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (live_room_id,user_id),
    KEY idx_live_guards_expiry (live_room_id,expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

INSERT INTO live_gifts(gift_key,name,price_coin,status,sort_order)
VALUES
    ('star','星光',10,1,30),
    ('heart','心动',52,1,20),
    ('crown','皇冠',188,1,10)
ON DUPLICATE KEY UPDATE
    name=VALUES(name),price_coin=VALUES(price_coin),status=VALUES(status),sort_order=VALUES(sort_order);
