-- Repair text that was imported through the wrong client charset.
-- Run this file with: mysql --default-character-set=utf8mb4 <db> < docs/sql/encoding_repair_20260611.sql

SET NAMES utf8mb4;
ALTER DATABASE CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;

SET @now := UNIX_TIMESTAMP();

INSERT INTO `cmf_lottery_category`
  (`id`, `name`, `name_en`, `icon`, `sort`, `status`, `create_time`, `update_time`)
VALUES
  (1, '热门', 'Hot', 'hot', 100, 1, @now, @now),
  (2, '真人', 'Live', 'live', 90, 1, @now, @now),
  (3, '棋牌', 'Card', 'card', 80, 1, @now, @now),
  (4, '捕鱼', 'Fishing', 'fishing', 70, 1, @now, @now),
  (5, '电子', 'Arcade', 'arcade', 60, 1, @now, @now),
  (6, '体育', 'Sports', 'sports', 50, 1, @now, @now)
ON DUPLICATE KEY UPDATE
  `name` = VALUES(`name`),
  `name_en` = VALUES(`name_en`),
  `icon` = VALUES(`icon`),
  `sort` = VALUES(`sort`),
  `status` = VALUES(`status`),
  `update_time` = VALUES(`update_time`);

UPDATE `cmf_admin_menu`
SET `name` = '游戏管理'
WHERE `id` = 9001;

UPDATE `cmf_auth_rule`
SET `title` = '游戏管理'
WHERE `id` = 9001 OR `name` = 'Admin/Lotterygame/index';

UPDATE `cmf_admin_menu`
SET `name` = CASE LOWER(`action`)
    WHEN 'index' THEN '虚拟直播'
    WHEN 'add' THEN '创建虚拟直播'
    WHEN 'addpost' THEN '创建虚拟直播提交'
    WHEN 'start' THEN '启动推流'
    WHEN 'stop' THEN '停止推流'
    WHEN 'restart' THEN '重启推流'
    WHEN 'del' THEN '删除任务'
    WHEN 'video' THEN '视频素材'
    WHEN 'videoadd' THEN '上传素材'
    WHEN 'videoaddpost' THEN '上传素材提交'
    WHEN 'videodel' THEN '删除素材'
    WHEN 'accounts' THEN '虚拟账号池'
    WHEN 'generateusers' THEN '批量生成虚拟账号'
    WHEN 'log' THEN '推流日志'
    ELSE `name`
  END,
  `remark` = CASE
    WHEN LOWER(`action`) = 'index' THEN '后台创建虚拟用户录播推流'
    ELSE `remark`
  END
WHERE `app` = 'admin' AND `controller` = 'Virtuallive';

UPDATE `cmf_auth_rule`
SET `title` = CASE LOWER(`name`)
    WHEN 'admin/virtuallive/index' THEN '虚拟直播'
    WHEN 'admin/virtuallive/add' THEN '创建虚拟直播'
    WHEN 'admin/virtuallive/addpost' THEN '创建虚拟直播提交'
    WHEN 'admin/virtuallive/start' THEN '启动推流'
    WHEN 'admin/virtuallive/stop' THEN '停止推流'
    WHEN 'admin/virtuallive/restart' THEN '重启推流'
    WHEN 'admin/virtuallive/del' THEN '删除任务'
    WHEN 'admin/virtuallive/video' THEN '视频素材'
    WHEN 'admin/virtuallive/videoadd' THEN '上传素材'
    WHEN 'admin/virtuallive/videoaddpost' THEN '上传素材提交'
    WHEN 'admin/virtuallive/videodel' THEN '删除素材'
    WHEN 'admin/virtuallive/accounts' THEN '虚拟账号池'
    WHEN 'admin/virtuallive/generateusers' THEN '批量生成虚拟账号'
    WHEN 'admin/virtuallive/log' THEN '推流日志'
    ELSE `title`
  END
WHERE LOWER(`name`) LIKE 'admin/virtuallive/%';
