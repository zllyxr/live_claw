-- 虚拟直播/后台推流模块
-- 执行日期: 2026-06-06

ALTER TABLE `cmf_user`
  ADD COLUMN `is_virtual` tinyint(1) NOT NULL DEFAULT '0' COMMENT '虚拟直播账号 0否 1是' AFTER `is_ad`;

CREATE TABLE IF NOT EXISTS `cmf_virtual_live_video` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `title` varchar(255) NOT NULL DEFAULT '' COMMENT '素材名称',
  `file_path` varchar(500) NOT NULL DEFAULT '' COMMENT '存储路径 local_/minio_',
  `file_url` varchar(1000) NOT NULL DEFAULT '' COMMENT '可访问地址',
  `cover` varchar(500) NOT NULL DEFAULT '' COMMENT '封面',
  `duration` int unsigned NOT NULL DEFAULT '0' COMMENT '时长秒',
  `filesize` bigint unsigned NOT NULL DEFAULT '0' COMMENT '文件大小',
  `status` tinyint(1) NOT NULL DEFAULT '1' COMMENT '状态 0停用 1启用',
  `addtime` int unsigned NOT NULL DEFAULT '0',
  `updatetime` int unsigned NOT NULL DEFAULT '0',
  PRIMARY KEY (`id`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='虚拟直播视频素材';

CREATE TABLE IF NOT EXISTS `cmf_virtual_live_task` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `uid` bigint unsigned NOT NULL DEFAULT '0' COMMENT '虚拟用户ID',
  `video_id` bigint unsigned NOT NULL DEFAULT '0' COMMENT '素材ID',
  `source_type` tinyint(1) NOT NULL DEFAULT '1' COMMENT '1视频素材 2OBS外部推流 3PAGE拉流转推',
  `source_page` varchar(1000) NOT NULL DEFAULT '' COMMENT 'PAGE拉流地址',
  `title` varchar(255) NOT NULL DEFAULT '' COMMENT '直播标题',
  `topic` varchar(255) NOT NULL DEFAULT '' COMMENT '管理话题',
  `thumb` varchar(500) NOT NULL DEFAULT '' COMMENT '直播封面',
  `liveclassid` int unsigned NOT NULL DEFAULT '0' COMMENT '直播分类',
  `type` tinyint(1) NOT NULL DEFAULT '0' COMMENT '房间类型',
  `type_val` varchar(255) NOT NULL DEFAULT '' COMMENT '密码/价格',
  `anyway` tinyint(1) NOT NULL DEFAULT '0' COMMENT '0竖屏 1横屏',
  `province` varchar(255) NOT NULL DEFAULT '',
  `city` varchar(255) NOT NULL DEFAULT '',
  `lng` varchar(255) NOT NULL DEFAULT '',
  `lat` varchar(255) NOT NULL DEFAULT '',
  `loop_play` tinyint(1) NOT NULL DEFAULT '1' COMMENT '是否循环播放',
  `stream` varchar(255) NOT NULL DEFAULT '' COMMENT '流名',
  `push_url` varchar(1000) NOT NULL DEFAULT '' COMMENT '推流地址',
  `pull_url` varchar(1000) NOT NULL DEFAULT '' COMMENT '拉流地址',
  `input_url` varchar(1000) NOT NULL DEFAULT '' COMMENT 'ffmpeg 输入源',
  `pid` int unsigned NOT NULL DEFAULT '0' COMMENT 'ffmpeg PID',
  `status` tinyint(1) NOT NULL DEFAULT '0' COMMENT '0待开始 1推流中 2已停止 3失败',
  `command` text COMMENT '启动命令',
  `log_file` varchar(1000) NOT NULL DEFAULT '' COMMENT '日志文件',
  `error_msg` text COMMENT '错误信息',
  `starttime` int unsigned NOT NULL DEFAULT '0',
  `stoptime` int unsigned NOT NULL DEFAULT '0',
  `addtime` int unsigned NOT NULL DEFAULT '0',
  `updatetime` int unsigned NOT NULL DEFAULT '0',
  PRIMARY KEY (`id`),
  KEY `idx_uid` (`uid`),
  KEY `idx_status` (`status`),
  KEY `idx_video` (`video_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='虚拟直播推流任务';

-- 后台菜单/权限
INSERT INTO `cmf_admin_menu`
(`parent_id`,`type`,`status`,`list_order`,`app`,`controller`,`action`,`param`,`name`,`icon`,`remark`)
SELECT 218,1,1,6.5,'admin','Virtuallive','index','','虚拟直播','','后台创建虚拟用户录播推流'
WHERE NOT EXISTS (SELECT 1 FROM `cmf_admin_menu` WHERE `app`='admin' AND `controller`='Virtuallive' AND `action`='index');

SET @virtual_live_parent_id := (SELECT `id` FROM `cmf_admin_menu` WHERE `app`='admin' AND `controller`='Virtuallive' AND `action`='index' LIMIT 1);

INSERT INTO `cmf_admin_menu`
(`parent_id`,`type`,`status`,`list_order`,`app`,`controller`,`action`,`param`,`name`,`icon`,`remark`)
SELECT @virtual_live_parent_id,1,0,1,'admin','Virtuallive','add','','创建虚拟直播','',''
WHERE NOT EXISTS (SELECT 1 FROM `cmf_admin_menu` WHERE `app`='admin' AND `controller`='Virtuallive' AND `action`='add');
INSERT INTO `cmf_admin_menu`
(`parent_id`,`type`,`status`,`list_order`,`app`,`controller`,`action`,`param`,`name`,`icon`,`remark`)
SELECT @virtual_live_parent_id,1,0,2,'admin','Virtuallive','addPost','','创建虚拟直播提交','',''
WHERE NOT EXISTS (SELECT 1 FROM `cmf_admin_menu` WHERE `app`='admin' AND `controller`='Virtuallive' AND `action`='addPost');
INSERT INTO `cmf_admin_menu`
(`parent_id`,`type`,`status`,`list_order`,`app`,`controller`,`action`,`param`,`name`,`icon`,`remark`)
SELECT @virtual_live_parent_id,1,0,3,'admin','Virtuallive','start','','启动推流','',''
WHERE NOT EXISTS (SELECT 1 FROM `cmf_admin_menu` WHERE `app`='admin' AND `controller`='Virtuallive' AND `action`='start');
INSERT INTO `cmf_admin_menu`
(`parent_id`,`type`,`status`,`list_order`,`app`,`controller`,`action`,`param`,`name`,`icon`,`remark`)
SELECT @virtual_live_parent_id,1,0,4,'admin','Virtuallive','stop','','停止推流','',''
WHERE NOT EXISTS (SELECT 1 FROM `cmf_admin_menu` WHERE `app`='admin' AND `controller`='Virtuallive' AND `action`='stop');
INSERT INTO `cmf_admin_menu`
(`parent_id`,`type`,`status`,`list_order`,`app`,`controller`,`action`,`param`,`name`,`icon`,`remark`)
SELECT @virtual_live_parent_id,1,0,5,'admin','Virtuallive','restart','','重启推流','',''
WHERE NOT EXISTS (SELECT 1 FROM `cmf_admin_menu` WHERE `app`='admin' AND `controller`='Virtuallive' AND `action`='restart');
INSERT INTO `cmf_admin_menu`
(`parent_id`,`type`,`status`,`list_order`,`app`,`controller`,`action`,`param`,`name`,`icon`,`remark`)
SELECT @virtual_live_parent_id,1,0,6,'admin','Virtuallive','del','','删除任务','',''
WHERE NOT EXISTS (SELECT 1 FROM `cmf_admin_menu` WHERE `app`='admin' AND `controller`='Virtuallive' AND `action`='del');
INSERT INTO `cmf_admin_menu`
(`parent_id`,`type`,`status`,`list_order`,`app`,`controller`,`action`,`param`,`name`,`icon`,`remark`)
SELECT @virtual_live_parent_id,1,0,7,'admin','Virtuallive','video','','视频素材','',''
WHERE NOT EXISTS (SELECT 1 FROM `cmf_admin_menu` WHERE `app`='admin' AND `controller`='Virtuallive' AND `action`='video');
INSERT INTO `cmf_admin_menu`
(`parent_id`,`type`,`status`,`list_order`,`app`,`controller`,`action`,`param`,`name`,`icon`,`remark`)
SELECT @virtual_live_parent_id,1,0,8,'admin','Virtuallive','videoAdd','','上传素材','',''
WHERE NOT EXISTS (SELECT 1 FROM `cmf_admin_menu` WHERE `app`='admin' AND `controller`='Virtuallive' AND `action`='videoAdd');
INSERT INTO `cmf_admin_menu`
(`parent_id`,`type`,`status`,`list_order`,`app`,`controller`,`action`,`param`,`name`,`icon`,`remark`)
SELECT @virtual_live_parent_id,1,0,9,'admin','Virtuallive','videoAddPost','','上传素材提交','',''
WHERE NOT EXISTS (SELECT 1 FROM `cmf_admin_menu` WHERE `app`='admin' AND `controller`='Virtuallive' AND `action`='videoAddPost');
INSERT INTO `cmf_admin_menu`
(`parent_id`,`type`,`status`,`list_order`,`app`,`controller`,`action`,`param`,`name`,`icon`,`remark`)
SELECT @virtual_live_parent_id,1,0,10,'admin','Virtuallive','videoDel','','删除素材','',''
WHERE NOT EXISTS (SELECT 1 FROM `cmf_admin_menu` WHERE `app`='admin' AND `controller`='Virtuallive' AND `action`='videoDel');
INSERT INTO `cmf_admin_menu`
(`parent_id`,`type`,`status`,`list_order`,`app`,`controller`,`action`,`param`,`name`,`icon`,`remark`)
SELECT @virtual_live_parent_id,1,0,11,'admin','Virtuallive','accounts','','虚拟账号池','',''
WHERE NOT EXISTS (SELECT 1 FROM `cmf_admin_menu` WHERE `app`='admin' AND `controller`='Virtuallive' AND `action`='accounts');
INSERT INTO `cmf_admin_menu`
(`parent_id`,`type`,`status`,`list_order`,`app`,`controller`,`action`,`param`,`name`,`icon`,`remark`)
SELECT @virtual_live_parent_id,1,0,12,'admin','Virtuallive','generateUsers','','批量生成虚拟账号','',''
WHERE NOT EXISTS (SELECT 1 FROM `cmf_admin_menu` WHERE `app`='admin' AND `controller`='Virtuallive' AND `action`='generateUsers');
INSERT INTO `cmf_admin_menu`
(`parent_id`,`type`,`status`,`list_order`,`app`,`controller`,`action`,`param`,`name`,`icon`,`remark`)
SELECT @virtual_live_parent_id,1,0,13,'admin','Virtuallive','log','','推流日志','',''
WHERE NOT EXISTS (SELECT 1 FROM `cmf_admin_menu` WHERE `app`='admin' AND `controller`='Virtuallive' AND `action`='log');

INSERT INTO `cmf_auth_rule` (`status`,`app`,`type`,`name`,`param`,`title`,`condition`)
SELECT 1,'admin','admin_url',LOWER(CONCAT('admin/Virtuallive/',`action`)),'',`name`,''
FROM `cmf_admin_menu`
WHERE `app`='admin' AND `controller`='Virtuallive'
ON DUPLICATE KEY UPDATE `title`=VALUES(`title`);
