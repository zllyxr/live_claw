-- 后台菜单重组：减少散乱顶级菜单，按业务域收拢。
-- 运行位置：claw_live 数据库。

START TRANSACTION;

UPDATE cmf_admin_menu SET list_order = 10  WHERE id = 6;    -- 设置
UPDATE cmf_admin_menu SET list_order = 20  WHERE id = 110;  -- 用户管理
UPDATE cmf_admin_menu SET list_order = 30  WHERE id = 218;  -- 直播管理
UPDATE cmf_admin_menu SET list_order = 40  WHERE id = 281;  -- 动态管理
UPDATE cmf_admin_menu SET list_order = 50  WHERE id = 312;  -- 视频管理
UPDATE cmf_admin_menu SET list_order = 60  WHERE id = 561;  -- 内容管理
UPDATE cmf_admin_menu SET list_order = 70  WHERE id = 371;  -- 财务管理
UPDATE cmf_admin_menu SET list_order = 80  WHERE id = 414;  -- 道具管理
UPDATE cmf_admin_menu SET list_order = 90  WHERE id = 600;  -- 游戏管理
UPDATE cmf_admin_menu SET list_order = 100 WHERE id = 551;  -- 消息管理
UPDATE cmf_admin_menu SET list_order = 200 WHERE id = 1;    -- 应用中心

UPDATE cmf_admin_menu SET parent_id = 110, list_order = 30 WHERE id = 214; -- 身份认证
UPDATE cmf_admin_menu SET parent_id = 110, list_order = 40 WHERE id = 446; -- 等级管理
UPDATE cmf_admin_menu SET parent_id = 110, list_order = 50 WHERE id = 395; -- 家族管理
UPDATE cmf_admin_menu SET parent_id = 110, list_order = 60 WHERE id = 459; -- 邀请奖励
UPDATE cmf_admin_menu SET parent_id = 110, list_order = 70 WHERE id = 547; -- 登录奖励

UPDATE cmf_admin_menu SET parent_id = 218, list_order = 110 WHERE id = 439; -- 守护管理
UPDATE cmf_admin_menu SET parent_id = 218, list_order = 120 WHERE id = 437; -- 红包管理

UPDATE cmf_admin_menu SET parent_id = 600, list_order = 5  WHERE id = 206; -- 奖池管理
UPDATE cmf_admin_menu SET parent_id = 600, list_order = 30 WHERE id = 536; -- 大转盘
UPDATE cmf_admin_menu SET parent_id = 600, list_order = 60 WHERE id = 279; -- 游戏记录
UPDATE cmf_admin_menu SET name = '彩票游戏', list_order = 10 WHERE id = 9001;
UPDATE cmf_admin_menu SET list_order = 20 WHERE id = 9100; -- 体育投注
UPDATE cmf_admin_menu SET list_order = 40 WHERE id = 601;  -- 星球探宝
UPDATE cmf_admin_menu SET list_order = 50 WHERE id = 602;  -- 幸运大转盘

-- 隐藏应用中心及插件管理入口。
UPDATE cmf_admin_menu
SET status = 0
WHERE id = 1
   OR parent_id = 1
   OR parent_id = 42
   OR (app = 'admin' AND controller = 'Plugin');

COMMIT;
