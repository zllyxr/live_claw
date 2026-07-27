-- Trim verbose local lottery plays.
-- Safe to run multiple times.

SET @now := UNIX_TIMESTAMP();

UPDATE `cmf_lottery_play`
SET
  `result_rule` = 'lhc_special_number',
  `status` = 1,
  `update_time` = @now
WHERE `play_code` = 'SPECIAL_NUMBER'
  AND `result_rule` LIKE 'position\_%';

UPDATE `cmf_lottery_play`
SET
  `status` = 0,
  `update_time` = @now
WHERE `status` = 1
  AND (
    `play_code` IN (
      'SUM_SIZE',
      'SUM_ODD_EVEN',
      'SUM_EXACT',
      'CHAMPION',
      'RUNNER_UP',
      'CHAMPION_SIZE',
      'CHAMPION_ODD_EVEN',
      'TOP2_SUM_SIZE',
      'TOP2_SUM_ODD_EVEN'
    )
    OR `play_code` LIKE 'POSITION\_%'
    OR SUBSTRING_INDEX(`result_rule`, ':', 1) IN (
      'sum_size',
      'sum_size_threshold',
      'sum_odd_even',
      'exact_sum',
      'position_size',
      'position_odd_even',
      'sum_positions_size',
      'sum_positions_odd_even'
    )
    OR SUBSTRING_INDEX(`result_rule`, ':', 1) REGEXP '^position_[0-9]+$'
  );
