-- ============================================================
-- 小游戏平台钱包流水
-- 所有游戏资金变化统一经 core 服务写入，游戏容器不得直连用户表。
-- ============================================================

SET NAMES utf8mb4;

CREATE TABLE IF NOT EXISTS `cmf_minigame_wallet_order` (
    `id`              bigint unsigned NOT NULL AUTO_INCREMENT,
    `order_no`        varchar(80)  NOT NULL COMMENT '游戏侧幂等订单号',
    `uid`             bigint unsigned NOT NULL,
    `game_code`       varchar(60)  NOT NULL,
    `table_no`        smallint unsigned NOT NULL COMMENT '1..1000',
    `round_no`        varchar(80)  NOT NULL DEFAULT '',
    `reason`          varchar(80)  NOT NULL DEFAULT '',
    `amount`          bigint NOT NULL COMMENT '正数派彩，负数扣款',
    `balance_before`  bigint NOT NULL,
    `balance_after`   bigint NOT NULL,
    `create_time`     int unsigned NOT NULL,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_order_no` (`order_no`),
    KEY `idx_uid_time` (`uid`, `create_time`),
    KEY `idx_game_round` (`game_code`, `round_no`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='小游戏平台钱包幂等流水';

-- 已注册游戏全部切换为平台钱包、登录后实时匹配。
UPDATE `cmf_minigame`
SET `need_login`=1,
    `use_wallet`=1,
    `play_mode`='realtime',
    `orientation`=CASE WHEN `code` IN ('ddz','mahjong','deepsea_hunter') THEN 'landscape' ELSE `orientation` END,
    `remark`=CASE
        WHEN `code`='deepsea_hunter' THEN '四座实时街机捕鱼，默认匹配 1000 桌，统一平台钱包结算'
        ELSE `remark`
    END,
    `update_time`=UNIX_TIMESTAMP();
