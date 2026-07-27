INSERT INTO `cmf_admin_menu`
(`id`,`parent_id`,`type`,`status`,`list_order`,`app`,`controller`,`action`,`param`,`name`,`icon`,`remark`)
VALUES
(627,613,1,0,10000,'admin','Virtuallive','bulkPageStart','','批量发现并转推','','')
ON DUPLICATE KEY UPDATE
  `parent_id`=VALUES(`parent_id`),
  `type`=VALUES(`type`),
  `status`=VALUES(`status`),
  `app`=VALUES(`app`),
  `controller`=VALUES(`controller`),
  `action`=VALUES(`action`),
  `name`=VALUES(`name`);
