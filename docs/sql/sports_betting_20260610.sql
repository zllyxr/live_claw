-- Sports betting foundation.
-- No match, odds, order, or other business sample data is inserted here.

CREATE TABLE IF NOT EXISTS `cmf_sports_match` (
  `id` int unsigned NOT NULL AUTO_INCREMENT,
  `source` varchar(40) NOT NULL DEFAULT 'api-football' COMMENT 'data source',
  `source_match_id` varchar(80) NOT NULL DEFAULT '' COMMENT 'upstream match id',
  `competition` varchar(160) NOT NULL DEFAULT '',
  `competition_type` varchar(160) NOT NULL DEFAULT '',
  `country` varchar(80) NOT NULL DEFAULT '',
  `home_name` varchar(160) NOT NULL DEFAULT '',
  `away_name` varchar(160) NOT NULL DEFAULT '',
  `home_logo` varchar(255) NOT NULL DEFAULT '',
  `away_logo` varchar(255) NOT NULL DEFAULT '',
  `match_date` varchar(20) NOT NULL DEFAULT '',
  `kickoff_time` int unsigned NOT NULL DEFAULT 0,
  `bet_close_time` int unsigned NOT NULL DEFAULT 0,
  `seal_advance_sec` int unsigned NOT NULL DEFAULT 300 COMMENT 'seconds before kickoff to stop betting',
  `home_score` smallint NOT NULL DEFAULT -1,
  `away_score` smallint NOT NULL DEFAULT -1,
  `status` varchar(30) NOT NULL DEFAULT 'NS' COMMENT 'upstream status',
  `status_text` varchar(80) NOT NULL DEFAULT '',
  `raw_status` varchar(60) NOT NULL DEFAULT '',
  `bet_status` tinyint(1) NOT NULL DEFAULT 1 COMMENT '0 disabled, 1 open, 2 closed',
  `settle_status` tinyint(1) NOT NULL DEFAULT 0 COMMENT '0 pending, 1 settled, 2 refunded, 3 blocked',
  `min_bet` bigint unsigned NOT NULL DEFAULT 10,
  `max_bet` bigint unsigned NOT NULL DEFAULT 500000,
  `max_match_bet` bigint unsigned NOT NULL DEFAULT 1000000,
  `sync_time` int unsigned NOT NULL DEFAULT 0,
  `settle_time` int unsigned NOT NULL DEFAULT 0,
  `settle_remark` varchar(255) NOT NULL DEFAULT '',
  `create_time` int unsigned NOT NULL DEFAULT 0,
  `update_time` int unsigned NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_source_match` (`source`, `source_match_id`),
  KEY `idx_time_status` (`kickoff_time`, `status`),
  KEY `idx_bet_status` (`bet_status`, `bet_close_time`),
  KEY `idx_settle_status` (`settle_status`, `status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Sports match';

CREATE TABLE IF NOT EXISTS `cmf_sports_market` (
  `id` int unsigned NOT NULL AUTO_INCREMENT,
  `match_id` int unsigned NOT NULL DEFAULT 0,
  `market_code` varchar(40) NOT NULL DEFAULT '',
  `market_name` varchar(80) NOT NULL DEFAULT '',
  `market_rule` varchar(80) NOT NULL DEFAULT '',
  `sort` int NOT NULL DEFAULT 0,
  `status` tinyint(1) NOT NULL DEFAULT 1,
  `create_time` int unsigned NOT NULL DEFAULT 0,
  `update_time` int unsigned NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_match_market` (`match_id`, `market_code`),
  KEY `idx_match_status` (`match_id`, `status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Sports betting market';

CREATE TABLE IF NOT EXISTS `cmf_sports_option` (
  `id` int unsigned NOT NULL AUTO_INCREMENT,
  `market_id` int unsigned NOT NULL DEFAULT 0,
  `option_code` varchar(40) NOT NULL DEFAULT '',
  `option_name` varchar(80) NOT NULL DEFAULT '',
  `odds` decimal(10,4) NOT NULL DEFAULT 1.0000,
  `sort` int NOT NULL DEFAULT 0,
  `status` tinyint(1) NOT NULL DEFAULT 1,
  `create_time` int unsigned NOT NULL DEFAULT 0,
  `update_time` int unsigned NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_market_option` (`market_id`, `option_code`),
  KEY `idx_market_status` (`market_id`, `status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Sports betting option';

CREATE TABLE IF NOT EXISTS `cmf_sports_bet_order` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `order_no` varchar(64) NOT NULL DEFAULT '',
  `client_trace_id` varchar(80) NOT NULL DEFAULT '',
  `uid` int unsigned NOT NULL DEFAULT 0,
  `match_id` int unsigned NOT NULL DEFAULT 0,
  `source_match_id` varchar(80) NOT NULL DEFAULT '',
  `match_title` varchar(255) NOT NULL DEFAULT '',
  `kickoff_time` int unsigned NOT NULL DEFAULT 0,
  `total_bet` bigint unsigned NOT NULL DEFAULT 0,
  `total_payout` bigint unsigned NOT NULL DEFAULT 0,
  `net_amount` bigint NOT NULL DEFAULT 0,
  `status` tinyint(1) NOT NULL DEFAULT 0 COMMENT '0 pending, 1 win, 2 lose, 3 refund, 4 canceled',
  `bet_time` int unsigned NOT NULL DEFAULT 0,
  `settle_time` int unsigned NOT NULL DEFAULT 0,
  `settle_remark` varchar(255) NOT NULL DEFAULT '',
  `create_time` int unsigned NOT NULL DEFAULT 0,
  `update_time` int unsigned NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_order_no` (`order_no`),
  UNIQUE KEY `uk_uid_trace` (`uid`, `client_trace_id`),
  KEY `idx_uid_time` (`uid`, `bet_time`),
  KEY `idx_match_status` (`match_id`, `status`),
  KEY `idx_status_time` (`status`, `bet_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Sports bet order';

CREATE TABLE IF NOT EXISTS `cmf_sports_bet_item` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `order_id` bigint unsigned NOT NULL DEFAULT 0,
  `uid` int unsigned NOT NULL DEFAULT 0,
  `match_id` int unsigned NOT NULL DEFAULT 0,
  `market_id` int unsigned NOT NULL DEFAULT 0,
  `market_code` varchar(40) NOT NULL DEFAULT '',
  `market_name` varchar(80) NOT NULL DEFAULT '',
  `option_id` int unsigned NOT NULL DEFAULT 0,
  `option_code` varchar(40) NOT NULL DEFAULT '',
  `option_name` varchar(80) NOT NULL DEFAULT '',
  `odds` decimal(10,4) NOT NULL DEFAULT 1.0000,
  `bet_amount` bigint unsigned NOT NULL DEFAULT 0,
  `payout_amount` bigint unsigned NOT NULL DEFAULT 0,
  `win_status` tinyint(1) NOT NULL DEFAULT 0 COMMENT '0 pending, 1 win, 2 lose, 3 refund',
  `create_time` int unsigned NOT NULL DEFAULT 0,
  `update_time` int unsigned NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  KEY `idx_order` (`order_id`),
  KEY `idx_match_market` (`match_id`, `market_code`),
  KEY `idx_uid_match` (`uid`, `match_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Sports bet item';

CREATE TABLE IF NOT EXISTS `cmf_sports_settle_log` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `match_id` int unsigned NOT NULL DEFAULT 0,
  `settle_key` varchar(100) NOT NULL DEFAULT '',
  `home_score` smallint NOT NULL DEFAULT -1,
  `away_score` smallint NOT NULL DEFAULT -1,
  `orders_total` int unsigned NOT NULL DEFAULT 0,
  `orders_win` int unsigned NOT NULL DEFAULT 0,
  `orders_lose` int unsigned NOT NULL DEFAULT 0,
  `orders_refund` int unsigned NOT NULL DEFAULT 0,
  `payout_total` bigint unsigned NOT NULL DEFAULT 0,
  `success` tinyint(1) NOT NULL DEFAULT 0,
  `message` varchar(255) NOT NULL DEFAULT '',
  `create_time` int unsigned NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_match_key` (`match_id`, `settle_key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Sports settle log';

CREATE TABLE IF NOT EXISTS `cmf_sports_sync_log` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `source` varchar(40) NOT NULL DEFAULT '',
  `api_name` varchar(80) NOT NULL DEFAULT '',
  `success` tinyint(1) NOT NULL DEFAULT 0,
  `message` varchar(255) NOT NULL DEFAULT '',
  `raw_response` text,
  `create_time` int unsigned NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  KEY `idx_source_time` (`source`, `create_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Sports sync log';

CREATE TABLE IF NOT EXISTS `cmf_sports_score_log` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `match_id` int unsigned NOT NULL DEFAULT 0,
  `source` varchar(40) NOT NULL DEFAULT '',
  `source_match_id` varchar(80) NOT NULL DEFAULT '',
  `old_home_score` smallint NOT NULL DEFAULT -1,
  `old_away_score` smallint NOT NULL DEFAULT -1,
  `new_home_score` smallint NOT NULL DEFAULT -1,
  `new_away_score` smallint NOT NULL DEFAULT -1,
  `old_status` varchar(30) NOT NULL DEFAULT '',
  `new_status` varchar(30) NOT NULL DEFAULT '',
  `old_status_text` varchar(80) NOT NULL DEFAULT '',
  `new_status_text` varchar(80) NOT NULL DEFAULT '',
  `accepted` tinyint(1) NOT NULL DEFAULT 1 COMMENT '1 accepted, 0 rejected',
  `reason` varchar(80) NOT NULL DEFAULT '',
  `raw_status` varchar(60) NOT NULL DEFAULT '',
  `create_time` int unsigned NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  KEY `idx_match_time` (`match_id`, `create_time`),
  KEY `idx_source_match_time` (`source`, `source_match_id`, `create_time`),
  KEY `idx_reason_time` (`reason`, `create_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Sports score sync log';

INSERT INTO `cmf_admin_menu`
  (`id`, `parent_id`, `type`, `status`, `list_order`, `app`, `controller`, `action`, `param`, `name`, `icon`, `remark`)
VALUES
  (9100, 600, 1, 1, 80, 'admin', 'Sportsbet', 'index', '', '体育投注', '', ''),
  (9101, 9100, 1, 1, 10, 'admin', 'Sportsbet', 'index', '', '比赛管理', '', ''),
  (9102, 9100, 1, 1, 20, 'admin', 'Sportsbet', 'orders', '', '体育注单', '', ''),
  (9103, 9100, 1, 1, 30, 'admin', 'Sportsbet', 'settleLogs', '', '结算日志', '', ''),
  (9104, 9100, 1, 0, 40, 'admin', 'Sportsbet', 'markets', '', '赔率配置', '', ''),
  (9105, 9100, 1, 0, 41, 'admin', 'Sportsbet', 'saveMarketsPost', '', '保存赔率', '', ''),
  (9106, 9100, 1, 0, 42, 'admin', 'Sportsbet', 'editMatch', '', '编辑比赛', '', ''),
  (9107, 9100, 1, 0, 43, 'admin', 'Sportsbet', 'editMatchPost', '', '保存比赛', '', ''),
  (9108, 9100, 1, 0, 44, 'admin', 'Sportsbet', 'setBetStatus', '', '设置投注状态', '', ''),
  (9109, 9100, 1, 0, 45, 'admin', 'Sportsbet', 'settleMatch', '', '结算比赛', '', ''),
  (9110, 9100, 1, 0, 46, 'admin', 'Sportsbet', 'refundMatch', '', '退款比赛', '', ''),
  (9112, 9100, 1, 0, 48, 'admin', 'Sportsbet', 'addMatch', '', '添加比赛', '', ''),
  (9113, 9100, 1, 0, 49, 'admin', 'Sportsbet', 'addMatchPost', '', '添加比赛提交', '', ''),
  (9114, 9100, 1, 1, 50, 'admin', 'Sportsbet', 'scoreLogs', '', '比分流水', '', '')
ON DUPLICATE KEY UPDATE
  `parent_id` = VALUES(`parent_id`),
  `type` = VALUES(`type`),
  `status` = VALUES(`status`),
  `list_order` = VALUES(`list_order`),
  `app` = VALUES(`app`),
  `controller` = VALUES(`controller`),
  `action` = VALUES(`action`),
  `name` = VALUES(`name`),
  `remark` = VALUES(`remark`);

INSERT INTO `cmf_auth_rule`
  (`id`, `status`, `app`, `type`, `name`, `param`, `title`, `condition`)
VALUES
  (9100, 1, 'Admin', 'admin_url', 'Admin/Sportsbet/index', '', '体育投注', ''),
  (9101, 1, 'Admin', 'admin_url', 'Admin/Sportsbet/index', '', '比赛管理', ''),
  (9102, 1, 'Admin', 'admin_url', 'Admin/Sportsbet/orders', '', '体育注单', ''),
  (9103, 1, 'Admin', 'admin_url', 'Admin/Sportsbet/settleLogs', '', '结算日志', ''),
  (9104, 1, 'Admin', 'admin_url', 'Admin/Sportsbet/markets', '', '赔率配置', ''),
  (9105, 1, 'Admin', 'admin_url', 'Admin/Sportsbet/saveMarketsPost', '', '保存赔率', ''),
  (9106, 1, 'Admin', 'admin_url', 'Admin/Sportsbet/editMatch', '', '编辑比赛', ''),
  (9107, 1, 'Admin', 'admin_url', 'Admin/Sportsbet/editMatchPost', '', '保存比赛', ''),
  (9108, 1, 'Admin', 'admin_url', 'Admin/Sportsbet/setBetStatus', '', '设置投注状态', ''),
  (9109, 1, 'Admin', 'admin_url', 'Admin/Sportsbet/settleMatch', '', '结算比赛', ''),
  (9110, 1, 'Admin', 'admin_url', 'Admin/Sportsbet/refundMatch', '', '退款比赛', ''),
  (9112, 1, 'Admin', 'admin_url', 'Admin/Sportsbet/addMatch', '', '添加比赛', ''),
  (9113, 1, 'Admin', 'admin_url', 'Admin/Sportsbet/addMatchPost', '', '添加比赛提交', ''),
  (9114, 1, 'Admin', 'admin_url', 'Admin/Sportsbet/scoreLogs', '', '比分流水', '')
ON DUPLICATE KEY UPDATE
  `status` = VALUES(`status`),
  `app` = VALUES(`app`),
  `type` = VALUES(`type`),
  `name` = VALUES(`name`),
  `title` = VALUES(`title`);
