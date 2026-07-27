-- ============================================================
-- 牌桌扩展：炸金花 / 跑得快 / 红中麻将
-- 生成日期: 2026-07-26
-- 依赖: cmf_minigame 注册表、cardgames 服务、平台钱包结算接口
-- 说明: 三款游戏默认进入各自的 1000 桌匹配池，全部为真人实时对战
-- ============================================================

SET NAMES utf8mb4;
SET @now := UNIX_TIMESTAMP();

INSERT INTO `cmf_minigame`
    (`code`, `name`, `name_en`, `category`, `cover`, `entry_type`, `entry_url`,
     `players_min`, `players_max`, `play_mode`, `need_login`, `use_wallet`, `orientation`,
     `license`, `upstream`, `remark`, `is_hot`, `is_new`, `sort`, `status`, `create_time`, `update_time`)
VALUES
    ('zhajinhua', '炸金花', 'Golden Flower', 'battle',
     '/static/art/minigame/zhajinhua-v1.webp', 'iframe', '/minigame/cards/zhajinhua/',
     3, 3, 'realtime', 1, 1, 'landscape',
     '', '', '三张牌比大小，可看牌、下注、跟注、弃牌与比牌；默认匹配 1000 桌，平台钱包统一结算',
     1, 1, 96, 1, @now, @now),

    ('paodekuai', '跑得快', 'Run Fast', 'battle',
     '/static/art/minigame/paodekuai-v1.webp', 'iframe', '/minigame/cards/paodekuai/',
     3, 3, 'realtime', 1, 1, 'landscape',
     '', '', '三人 48 张玩法，黑桃 3 首出；默认匹配 1000 桌，平台钱包统一结算',
     1, 1, 95, 1, @now, @now),

    ('mahjong_red', '红中麻将', 'Red Center Mahjong', 'casual',
     '/static/art/minigame/mahjong-red-v1.webp', 'iframe', '/minigame/cards/mahjong/?variant=red',
     4, 4, 'realtime', 1, 1, 'landscape',
     '', '', '红中作赖子，可补将、刻子与顺子；默认匹配 1000 桌，平台钱包统一结算',
     1, 1, 94, 1, @now, @now)
ON DUPLICATE KEY UPDATE
    `name` = VALUES(`name`), `name_en` = VALUES(`name_en`), `category` = VALUES(`category`),
    `cover` = VALUES(`cover`), `entry_type` = VALUES(`entry_type`), `entry_url` = VALUES(`entry_url`),
    `players_min` = VALUES(`players_min`), `players_max` = VALUES(`players_max`),
    `play_mode` = VALUES(`play_mode`), `need_login` = VALUES(`need_login`),
    `use_wallet` = VALUES(`use_wallet`), `orientation` = VALUES(`orientation`),
    `remark` = VALUES(`remark`), `is_hot` = VALUES(`is_hot`), `is_new` = VALUES(`is_new`),
    `sort` = VALUES(`sort`), `status` = VALUES(`status`), `update_time` = VALUES(`update_time`);

SELECT `code`, `name`, `entry_url`, `players_max`, `use_wallet`, `orientation`, `status`
FROM `cmf_minigame`
WHERE `code` IN ('zhajinhua', 'paodekuai', 'mahjong_red')
ORDER BY `sort` DESC;
