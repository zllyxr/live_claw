-- ============================================================
-- 清理未使用功能：家族 / 转盘 / 奖池 / 背包 / 靓号 / VIP / 音乐 /
--                 引导页 / 小游戏 / 印象 / 连麦 / PK / 装备 / 青少年
--
-- 生成日期: 2026-07-25
-- 适用版本: claw_live（种子库 claw_live_20260611_204607.sql 之后）
--
-- 背景: 前台（uniapp）与 App API 均无任何调用，代码层已同步删除：
--   - PhalApi:  Api/Domain/Model 下 12 个模块
--   - H5:       admin/app/appapi/controller 下 7 个控制器 + 模板
--   - 后台:     admin/app/admin/controller 下 14 个控制器 + 模板
--
-- 执行方式:
--   docker compose exec -T db mysql -uclaw -pclaw_dev_pwd claw_live < docs/sql/cleanup_unused_features_20260725.sql
--
-- 回滚: 本脚本为破坏性操作，无回滚。执行前请备份：
--   docker compose exec -T db mysqldump -uroot -pclaw_root_pwd --no-tablespaces claw_live | gzip > backup.sql.gz
-- ============================================================

SET NAMES utf8mb4;

-- ---------- 1. 删除后台菜单项 ----------
DELETE FROM `cmf_admin_menu` WHERE `controller` IN (
    'Family', 'Familyuser',
    'Turntable', 'Turntablecon',
    'Jackpot', 'Jackpotrate',
    'Music', 'Musiccat',
    'Vip', 'Vipuser',
    'Liang', 'Impression',
    'Guide', 'guide',
    'Game'
);

-- ---------- 2. 删除权限规则 ----------
DELETE FROM `cmf_auth_rule` WHERE `name` REGEXP
    '^admin/(Family|Familyuser|Turntable|Turntablecon|Jackpot|Jackpotrate|Music|Musiccat|Vip|Vipuser|Liang|Impression|Guide|guide|Game)/';

-- ---------- 3. 删除业务数据表 ----------
-- 家族
DROP TABLE IF EXISTS `cmf_family_user_divide_apply`;
DROP TABLE IF EXISTS `cmf_family_user`;
DROP TABLE IF EXISTS `cmf_family_profit`;
DROP TABLE IF EXISTS `cmf_family`;

-- 转盘
DROP TABLE IF EXISTS `cmf_turntable_win`;
DROP TABLE IF EXISTS `cmf_turntable_log`;
DROP TABLE IF EXISTS `cmf_turntable_con`;
DROP TABLE IF EXISTS `cmf_turntable`;

-- 奖池
DROP TABLE IF EXISTS `cmf_jackpot_rate`;
DROP TABLE IF EXISTS `cmf_jackpot_level`;
DROP TABLE IF EXISTS `cmf_jackpot`;

-- 背包 / 靓号 / VIP / 引导页
DROP TABLE IF EXISTS `cmf_backpack`;
DROP TABLE IF EXISTS `cmf_liang`;
DROP TABLE IF EXISTS `cmf_vip_user`;
DROP TABLE IF EXISTS `cmf_vip`;
DROP TABLE IF EXISTS `cmf_guide`;

-- 音乐
DROP TABLE IF EXISTS `cmf_music_collection`;
DROP TABLE IF EXISTS `cmf_music_classify`;
DROP TABLE IF EXISTS `cmf_music`;

-- 小游戏（转盘/夺宝类，非彩票与体育）
DROP TABLE IF EXISTS `cmf_gamerecord`;
DROP TABLE IF EXISTS `cmf_game`;

-- ---------- 4. 校验 ----------
SELECT COUNT(*) AS remaining_menus FROM `cmf_admin_menu`
WHERE `controller` IN ('Family','Familyuser','Turntable','Turntablecon','Jackpot','Jackpotrate','Music','Musiccat','Vip','Vipuser','Liang','Impression','Guide','guide','Game');

SELECT COUNT(*) AS remaining_tables FROM information_schema.tables
WHERE table_schema = DATABASE() AND table_name IN (
    'cmf_family','cmf_family_profit','cmf_family_user','cmf_family_user_divide_apply',
    'cmf_turntable','cmf_turntable_con','cmf_turntable_log','cmf_turntable_win',
    'cmf_jackpot','cmf_jackpot_level','cmf_jackpot_rate',
    'cmf_backpack','cmf_guide','cmf_liang','cmf_vip','cmf_vip_user',
    'cmf_music','cmf_music_classify','cmf_music_collection',
    'cmf_game','cmf_gamerecord'
);
