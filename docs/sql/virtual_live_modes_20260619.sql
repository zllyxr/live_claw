-- Add virtual live source modes.
-- Safe to run on existing MySQL databases.

SET @has_source_type := (
  SELECT COUNT(*)
  FROM INFORMATION_SCHEMA.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE()
    AND TABLE_NAME = 'cmf_virtual_live_task'
    AND COLUMN_NAME = 'source_type'
);
SET @sql := IF(
  @has_source_type = 0,
  'ALTER TABLE `cmf_virtual_live_task` ADD COLUMN `source_type` tinyint(1) NOT NULL DEFAULT ''1'' COMMENT ''1视频素材 2OBS外部推流 3PAGE拉流转推'' AFTER `video_id`',
  'SELECT 1'
);
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @has_source_page := (
  SELECT COUNT(*)
  FROM INFORMATION_SCHEMA.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE()
    AND TABLE_NAME = 'cmf_virtual_live_task'
    AND COLUMN_NAME = 'source_page'
);
SET @sql := IF(
  @has_source_page = 0,
  'ALTER TABLE `cmf_virtual_live_task` ADD COLUMN `source_page` varchar(1000) NOT NULL DEFAULT '''' COMMENT ''PAGE拉流地址'' AFTER `source_type`',
  'SELECT 1'
);
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;
