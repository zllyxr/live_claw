-- Claw core rebuild: local lottery engine, database-only sports reads, OpenIM migration.
-- This migration is intentionally idempotent for tables and data backfill.

SET @now := UNIX_TIMESTAMP();

-- ThinkCMF permission checks require cmf_role.type for non-super administrators.
SET @add_role_type := IF(
  EXISTS(SELECT 1 FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name='cmf_role')
  AND NOT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='cmf_role' AND column_name='type'),
  'ALTER TABLE `cmf_role` ADD COLUMN `type` varchar(30) NOT NULL DEFAULT ''admin'' AFTER `name`',
  'SELECT 1'
);
PREPARE stmt FROM @add_role_type; EXECUTE stmt; DEALLOCATE PREPARE stmt;

UPDATE `cmf_role` SET `type`='admin' WHERE `type`='';

-- Legacy category icons were collector slugs rather than files and caused
-- requests such as /upload/hot. Empty values intentionally use the UI badge.
UPDATE `cmf_lottery_category`
SET `icon`=''
WHERE `icon` IN ('hot', 'live', 'card', 'fishing', 'arcade', 'sports', 'ol', 'blockchain');

-- Remove decommissioned IM configuration and placeholder public URLs.
UPDATE `cmf_option`
SET `option_value`=JSON_REMOVE(
  `option_value`,
  '$.chatserver',
  '$.tencentIM_area',
  '$.tencentIM_appid',
  '$.tencentIM_appkey'
)
WHERE `option_name`='configpri' AND JSON_VALID(`option_value`);

UPDATE `cmf_option`
SET `option_value`=JSON_SET(`option_value`,'$.site','','$.wx_siteurl','')
WHERE `option_name`='site_info'
  AND JSON_VALID(`option_value`)
  AND JSON_UNQUOTE(JSON_EXTRACT(`option_value`,'$.site')) IN (
    'http://x.com','https://x.com','http://192.168.31.186:8080','https://192.168.31.186:8080'
  );

CREATE TABLE IF NOT EXISTS `cmf_lottery_draw_config` (
  `id` int unsigned NOT NULL AUTO_INCREMENT,
  `game_id` int unsigned NOT NULL,
  `draw_mode` varchar(30) NOT NULL DEFAULT 'local_auto',
  `template_code` varchar(40) NOT NULL DEFAULT 'digit',
  `draw_count` int unsigned NOT NULL DEFAULT 5,
  `number_min` int NOT NULL DEFAULT 0,
  `number_max` int NOT NULL DEFAULT 9,
  `number_unique` tinyint(1) NOT NULL DEFAULT 0,
  `number_pad` tinyint unsigned NOT NULL DEFAULT 0,
  `sum_big_threshold` int unsigned NOT NULL DEFAULT 0,
  `status` tinyint(1) NOT NULL DEFAULT 1,
  `create_time` int unsigned NOT NULL DEFAULT 0,
  `update_time` int unsigned NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_game_id` (`game_id`),
  KEY `idx_status_game` (`status`,`game_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Local lottery draw configuration';

CREATE TABLE IF NOT EXISTS `cmf_lottery_preset_draw` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `game_id` int unsigned NOT NULL,
  `issue_id` bigint unsigned NOT NULL DEFAULT 0,
  `issue_num` varchar(80) NOT NULL,
  `open_code` varchar(100) NOT NULL,
  `status` tinyint(1) NOT NULL DEFAULT 1,
  `remark` varchar(255) NOT NULL DEFAULT '',
  `admin_id` int unsigned NOT NULL DEFAULT 0,
  `use_time` int unsigned NOT NULL DEFAULT 0,
  `create_time` int unsigned NOT NULL DEFAULT 0,
  `update_time` int unsigned NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_game_issue` (`game_id`,`issue_num`),
  KEY `idx_issue_id` (`issue_id`),
  KEY `idx_status_time` (`status`,`create_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Local lottery preset draw';

CREATE TABLE IF NOT EXISTS `cmf_lottery_draw_audit` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `issue_id` bigint unsigned NOT NULL,
  `game_id` int unsigned NOT NULL,
  `issue_num` varchar(80) NOT NULL,
  `draw_source` varchar(30) NOT NULL DEFAULT 'crypto_random',
  `open_code` varchar(100) NOT NULL,
  `entropy_hash` char(64) NOT NULL,
  `engine_version` varchar(30) NOT NULL,
  `create_time` int unsigned NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_issue` (`issue_id`),
  KEY `idx_game_time` (`game_id`,`create_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Immutable lottery draw audit';

CREATE TABLE IF NOT EXISTS `cmf_sports_snapshot` (
  `match_id` int unsigned NOT NULL,
  `source_match_id` varchar(80) NOT NULL,
  `payload` json NOT NULL,
  `collected_at` int unsigned NOT NULL,
  PRIMARY KEY (`match_id`),
  KEY `idx_source_match` (`source_match_id`),
  KEY `idx_collected_at` (`collected_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Collected upstream fixture snapshot';

INSERT INTO `cmf_lottery_draw_config`
  (`game_id`,`template_code`,`draw_count`,`number_min`,`number_max`,`number_unique`,`number_pad`,`sum_big_threshold`,`status`,`create_time`,`update_time`)
SELECT
  g.`id`,
  CASE
    WHEN g.`category_id`=4 OR LOWER(CONCAT(g.`game_code`,' ',g.`game_name`)) REGEXP 'k3|ks|快三' THEN 'k3'
    WHEN LOWER(CONCAT(g.`game_code`,' ',g.`game_name`)) REGEXP 'pk10|飞艇|赛车|racing' THEN 'pk10'
    WHEN g.`category_id`=3 OR LOWER(CONCAT(g.`game_code`,' ',g.`game_name`)) REGEXP 'syxw|11x5|11选5' THEN '11x5'
    WHEN g.`category_id`=5 OR LOWER(CONCAT(g.`game_code`,' ',g.`game_name`)) REGEXP 'klsf|快乐十分|农场' THEN 'klsf'
    WHEN LOWER(CONCAT(g.`game_code`,' ',g.`game_name`)) REGEXP 'kl8|bingo|快乐8' THEN 'kl8'
    WHEN LOWER(CONCAT(g.`game_code`,' ',g.`game_name`)) REGEXP 'lhc|hk6|macau6|六合彩' THEN 'lhc'
    WHEN LOWER(CONCAT(g.`game_code`,' ',g.`game_name`)) REGEXP '28|pcdd' THEN 'pc28'
    WHEN g.`game_code` IN ('SSQ','QLC','DLT','QXC') THEN 'official'
    WHEN LOWER(CONCAT(g.`game_code`,' ',g.`game_name`)) REGEXP 'ssc|ffc|5fc|时时彩|分分彩' THEN 'ssc'
    ELSE 'digit'
  END,
  CASE
    WHEN g.`category_id`=4 OR LOWER(CONCAT(g.`game_code`,' ',g.`game_name`)) REGEXP 'k3|ks|快三' THEN 3
    WHEN LOWER(CONCAT(g.`game_code`,' ',g.`game_name`)) REGEXP 'pk10|飞艇|赛车|racing' THEN 10
    WHEN LOWER(CONCAT(g.`game_code`,' ',g.`game_name`)) REGEXP 'klsf|快乐十分|农场' THEN 8
    WHEN LOWER(CONCAT(g.`game_code`,' ',g.`game_name`)) REGEXP 'kl8|bingo|快乐8' THEN 20
    WHEN LOWER(CONCAT(g.`game_code`,' ',g.`game_name`)) REGEXP 'lhc|hk6|macau6|六合彩' THEN 7
    WHEN LOWER(CONCAT(g.`game_code`,' ',g.`game_name`)) REGEXP '28|pcdd' THEN 3
    WHEN g.`game_code` IN ('SSQ','QLC','DLT','QXC') THEN 7
    ELSE 5
  END,
  CASE
    WHEN g.`category_id`=4 OR LOWER(CONCAT(g.`game_code`,' ',g.`game_name`)) REGEXP 'k3|ks|快三' THEN 1
    WHEN LOWER(CONCAT(g.`game_code`,' ',g.`game_name`)) REGEXP 'pk10|飞艇|赛车|racing|syxw|11x5|11选5|klsf|快乐十分|农场|kl8|bingo|快乐8|lhc|hk6|macau6|六合彩' THEN 1
    WHEN g.`game_code` IN ('SSQ','QLC','DLT','QXC') THEN 1
    ELSE 0
  END,
  CASE
    WHEN g.`category_id`=4 OR LOWER(CONCAT(g.`game_code`,' ',g.`game_name`)) REGEXP 'k3|ks|快三' THEN 6
    WHEN LOWER(CONCAT(g.`game_code`,' ',g.`game_name`)) REGEXP 'pk10|飞艇|赛车|racing' THEN 10
    WHEN g.`category_id`=3 OR LOWER(CONCAT(g.`game_code`,' ',g.`game_name`)) REGEXP 'syxw|11x5|11选5' THEN 11
    WHEN g.`category_id`=5 OR LOWER(CONCAT(g.`game_code`,' ',g.`game_name`)) REGEXP 'klsf|快乐十分|农场' THEN 20
    WHEN LOWER(CONCAT(g.`game_code`,' ',g.`game_name`)) REGEXP 'kl8|bingo|快乐8' THEN 80
    WHEN LOWER(CONCAT(g.`game_code`,' ',g.`game_name`)) REGEXP 'lhc|hk6|macau6|六合彩' THEN 49
    WHEN g.`game_code` IN ('SSQ','QLC','DLT','QXC') THEN 35
    ELSE 9
  END,
  CASE
    WHEN LOWER(CONCAT(g.`game_code`,' ',g.`game_name`)) REGEXP 'pk10|飞艇|赛车|racing|syxw|11x5|11选5|klsf|快乐十分|农场|kl8|bingo|快乐8|lhc|hk6|macau6|六合彩' THEN 1
    WHEN g.`game_code` IN ('SSQ','QLC','DLT','QXC') THEN 1
    ELSE 0
  END,
  CASE
    WHEN LOWER(CONCAT(g.`game_code`,' ',g.`game_name`)) REGEXP 'pk10|飞艇|赛车|racing|syxw|11x5|11选5|klsf|快乐十分|农场|kl8|bingo|快乐8|lhc|hk6|macau6|六合彩' THEN 2
    WHEN g.`game_code` IN ('SSQ','QLC','DLT','QXC') THEN 2
    ELSE 0
  END,
  0,1,@now,@now
FROM `cmf_lottery_game` g
ON DUPLICATE KEY UPDATE `status`=1,`update_time`=VALUES(`update_time`);

UPDATE `cmf_lottery_issue`
SET `status`=4,`update_time`=@now
WHERE `status` IN (0,1) AND `open_code`='' AND `open_time` < @now;

DROP TABLE IF EXISTS `cmf_lottery_source`;
DROP TABLE IF EXISTS `cmf_lottery_sync_log`;
DROP TABLE IF EXISTS `cmf_localim_message`;
DROP TABLE IF EXISTS `cmf_localim_conversation`;

SET @drop_source_id := IF(
  EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='cmf_lottery_game' AND column_name='source_id'),
  'ALTER TABLE `cmf_lottery_game` DROP COLUMN `source_id`',
  'SELECT 1'
);
PREPARE stmt FROM @drop_source_id; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @drop_source_lottery_id := IF(
  EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='cmf_lottery_game' AND column_name='source_lottery_id'),
  'ALTER TABLE `cmf_lottery_game` DROP COLUMN `source_lottery_id`',
  'SELECT 1'
);
PREPARE stmt FROM @drop_source_lottery_id; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @drop_source_e_name := IF(
  EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='cmf_lottery_game' AND column_name='source_e_name'),
  'ALTER TABLE `cmf_lottery_game` DROP COLUMN `source_e_name`',
  'SELECT 1'
);
PREPARE stmt FROM @drop_source_e_name; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @drop_sync_status := IF(
  EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='cmf_lottery_game' AND column_name='sync_status'),
  'ALTER TABLE `cmf_lottery_game` DROP COLUMN `sync_status`',
  'SELECT 1'
);
PREPARE stmt FROM @drop_sync_status; EXECUTE stmt; DEALLOCATE PREPARE stmt;

DELETE FROM `cmf_admin_menu`
WHERE (`controller`='Lotterygame' AND `action` IN ('syncLogs','syncCatalog'))
   OR (`controller`='System')
   OR `name` IN ('彩票同步日志','彩票开奖同步','直播间系统消息');

DELETE FROM `cmf_auth_rule`
WHERE `name` IN ('Admin/Lotterygame/syncLogs','Admin/Lotterygame/syncCatalog')
   OR `name` LIKE 'Admin/System/%';

DELETE FROM `cmf_sports_sync_log`
WHERE `source`<>'api-football';
