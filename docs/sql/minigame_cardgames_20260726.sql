-- ============================================================
-- 注册牌类游戏：斗地主 / 麻将
-- 生成日期: 2026-07-26
-- 依赖: docs/sql/minigame_registry_20260726.sql（cmf_minigame 表）
--
-- 服务: cardgames 容器，反代路径 /minigame/cards/
-- 说明: 均为服务端权威多人实时牌桌，默认匹配 1000 桌并使用平台钱包
-- ============================================================

SET NAMES utf8mb4;
SET @now := UNIX_TIMESTAMP();

INSERT INTO `cmf_minigame`
    (`code`, `name`, `name_en`, `category`, `cover`, `entry_type`, `entry_url`,
     `players_min`, `players_max`, `play_mode`, `need_login`, `use_wallet`, `orientation`,
     `license`, `upstream`, `remark`, `is_hot`, `is_new`, `sort`, `status`, `create_time`, `update_time`)
VALUES
    ('ddz', '斗地主', 'Dou Dizhu', 'casual',
     '/static/art/minigame/ddz.webp', 'iframe', '/minigame/cards/ddz/',
     3, 3, 'realtime', 1, 1, 'landscape',
     '', '', '经典斗地主，默认匹配 1000 桌，三位平台用户实时对战', 1, 1, 98, 1, @now, @now),

    ('mahjong', '麻将', 'Mahjong', 'casual',
     '/static/art/minigame/mahjong.webp', 'iframe', '/minigame/cards/mahjong/',
     4, 4, 'realtime', 1, 1, 'landscape',
     '', '', '推倒胡玩法，默认匹配 1000 桌，四位平台用户实时对战', 1, 1, 97, 1, @now, @now)
ON DUPLICATE KEY UPDATE
    `name` = VALUES(`name`), `cover` = VALUES(`cover`), `entry_url` = VALUES(`entry_url`),
    `players_min` = VALUES(`players_min`), `players_max` = VALUES(`players_max`),
    `play_mode` = VALUES(`play_mode`), `use_wallet` = VALUES(`use_wallet`),
    `remark` = VALUES(`remark`), `orientation` = VALUES(`orientation`),
    `sort` = VALUES(`sort`), `status` = VALUES(`status`), `update_time` = VALUES(`update_time`);

SELECT `code`, `name`, `category`, `entry_url`, `status` FROM `cmf_minigame` WHERE `status` = 1 ORDER BY `sort` DESC;
