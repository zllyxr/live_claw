-- ============================================================
-- 小游戏注册表
--
-- 生成日期: 2026-07-26
-- 目的: 统一管理平台内的小游戏（电子/休闲/对战），支持后续持续新增游戏
--       而无需改动客户端代码 —— 客户端只读 MiniGame.list 渲染。
--
-- 执行:
--   docker compose exec -T db mysql -uclaw -pclaw_dev_pwd claw_live < docs/sql/minigame_registry_20260726.sql
-- ============================================================

SET NAMES utf8mb4;

CREATE TABLE IF NOT EXISTS `cmf_minigame` (
    `id`            int unsigned NOT NULL AUTO_INCREMENT,
    `code`          varchar(60)  NOT NULL DEFAULT ''  COMMENT '唯一标识，如 deepsea_hunter',
    `name`          varchar(80)  NOT NULL DEFAULT ''  COMMENT '中文名',
    `name_en`       varchar(80)  NOT NULL DEFAULT ''  COMMENT '英文名/副标题',
    `category`      varchar(30)  NOT NULL DEFAULT 'arcade' COMMENT 'arcade电子 casual休闲 battle对战',
    `cover`         varchar(255) NOT NULL DEFAULT ''  COMMENT '封面图，静态路径或上传key',
    `entry_type`    varchar(20)  NOT NULL DEFAULT 'iframe' COMMENT 'iframe同源内嵌 external外链',
    `entry_url`     varchar(255) NOT NULL DEFAULT ''  COMMENT '入口地址，同源建议用 /minigame/xxx/ 形式',
    `players_min`   tinyint unsigned NOT NULL DEFAULT 1,
    `players_max`   tinyint unsigned NOT NULL DEFAULT 1,
    `play_mode`     varchar(30)  NOT NULL DEFAULT 'single' COMMENT 'single realtime local-keyboard local-turn-based webrtc',
    `need_login`    tinyint(1)   NOT NULL DEFAULT 1  COMMENT '是否要求登录后进入',
    -- 平台内游戏统一使用业务钱包；资金变化只能经 core 幂等结算接口。
    `use_wallet`    tinyint(1)   NOT NULL DEFAULT 1  COMMENT '1平台业务钱包',
    `orientation`   varchar(12)  NOT NULL DEFAULT 'auto' COMMENT 'auto portrait landscape',
    `license`       varchar(60)  NOT NULL DEFAULT ''  COMMENT '开源许可证，便于合规审计',
    `upstream`      varchar(255) NOT NULL DEFAULT ''  COMMENT '上游仓库地址',
    `remark`        varchar(255) NOT NULL DEFAULT ''  COMMENT '备注/玩法简介',
    `is_hot`        tinyint(1)   NOT NULL DEFAULT 0,
    `is_new`        tinyint(1)   NOT NULL DEFAULT 0,
    `sort`          int          NOT NULL DEFAULT 0  COMMENT '越大越靠前',
    `status`        tinyint(1)   NOT NULL DEFAULT 1  COMMENT '1上架 0下架',
    `create_time`   int unsigned NOT NULL DEFAULT 0,
    `update_time`   int unsigned NOT NULL DEFAULT 0,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_code` (`code`),
    KEY `idx_status_sort` (`status`, `sort`),
    KEY `idx_category` (`category`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='小游戏注册表';

SET @now := UNIX_TIMESTAMP();

-- ---------- 种子数据 ----------
-- 深海猎手（自研，Node + WebSocket 服务端权威，独立容器）
INSERT INTO `cmf_minigame`
    (`code`, `name`, `name_en`, `category`, `cover`, `entry_type`, `entry_url`,
     `players_min`, `players_max`, `play_mode`, `need_login`, `use_wallet`, `orientation`,
     `license`, `upstream`, `remark`, `is_hot`, `is_new`, `sort`, `status`, `create_time`, `update_time`)
VALUES
    ('deepsea_hunter', '深海猎手', 'Deep Sea Hunter', 'arcade',
     '/static/art/minigame/fish.webp', 'iframe', '/minigame/fish/',
     1, 4, 'realtime', 1, 1, 'landscape',
     '', '', '四座实时街机捕鱼，默认匹配 1000 桌，统一平台钱包结算', 1, 1, 100, 1, @now, @now)
ON DUPLICATE KEY UPDATE
    `name` = VALUES(`name`), `cover` = VALUES(`cover`), `entry_url` = VALUES(`entry_url`),
    `players_max` = VALUES(`players_max`), `orientation` = VALUES(`orientation`),
    `play_mode` = VALUES(`play_mode`), `need_login` = VALUES(`need_login`),
    `use_wallet` = VALUES(`use_wallet`), `remark` = VALUES(`remark`),
    `update_time` = VALUES(`update_time`);

-- game2 目录下的开源休闲/对战游戏（默认下架，逐个确认许可证与移动端适配后再上架）
INSERT INTO `cmf_minigame`
    (`code`, `name`, `name_en`, `category`, `cover`, `entry_type`, `entry_url`,
     `players_min`, `players_max`, `play_mode`, `need_login`, `use_wallet`, `orientation`,
     `license`, `upstream`, `remark`, `sort`, `status`, `create_time`, `update_time`)
VALUES
    ('achtung_kurve', '贪吃曲线', 'Achtung, die Kurve!', 'battle',
     '', 'iframe', '/minigame/lib/achtung-kurve/index.html',
     2, 5, 'local-keyboard', 1, 1, 'landscape',
     'GPL-3.0', 'https://github.com/maechler/kurve', '同屏键盘多人，需评估移动端操作', 50, 0, @now, @now),
    ('scorch', '焦土坦克', 'Scorch Clone', 'battle',
     '', 'iframe', '/minigame/lib/scorch/index.html',
     2, 4, 'local-turn-based', 1, 1, 'landscape',
     'MIT', 'https://github.com/webermn15/Scorch_a-scorched-earth-clone', '回合制炮击', 49, 0, @now, @now),
    ('fluid_table_tennis', '流体乒乓', 'Fluid Table Tennis', 'casual',
     '', 'iframe', '/minigame/lib/fluid-table-tennis/build/index.html',
     2, 2, 'local-keyboard', 1, 1, 'landscape',
     'LicenseRef-Upstream-Permissive', 'https://github.com/anirudhjoshi/fluid_table_tennis', '双人对打', 48, 0, @now, @now),
    ('siege_wars', '攻城战争', 'Siege Wars', 'battle',
     '', 'iframe', '/minigame/lib/siege-wars/dist/index.html',
     2, 2, 'local-turn-based', 1, 1, 'landscape',
     'MIT', 'https://github.com/raaaahman/siege-wars', '素材含 CC-BY-3.0，需保留署名', 47, 0, @now, @now),
    ('libreludo', '飞行棋', 'LibreLudo', 'casual',
     '', 'iframe', '/minigame/lib/libreludo/dist/index.html',
     2, 4, 'local-turn-based', 1, 1, 'auto',
     'AGPL-3.0-only', 'https://github.com/priyanshurav/libreludo', 'AGPL 传染性强，商用前需法务确认', 46, 0, @now, @now),
    ('p2p_maze_shooter', '迷宫对枪', 'P2P Maze Shooter', 'battle',
     '', 'iframe', '/minigame/lib/p2p-maze-shooter/src/index.html',
     2, 2, 'webrtc', 1, 1, 'landscape',
     'MIT', 'https://github.com/arifulislamat/p2p-maze-shooter', '依赖公网 WebRTC 信令', 45, 0, @now, @now)
ON DUPLICATE KEY UPDATE
    `name` = VALUES(`name`), `license` = VALUES(`license`), `remark` = VALUES(`remark`),
    `update_time` = VALUES(`update_time`);

-- ---------- 校验 ----------
SELECT `category`, COUNT(*) AS games, SUM(`status`) AS online
FROM `cmf_minigame` GROUP BY `category`;
