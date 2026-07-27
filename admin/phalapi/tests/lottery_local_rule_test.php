<?php
require_once __DIR__ . '/../src/app/Library/LotteryLocalRule.php';

use App\Library\LotteryLocalRule;

$failures = 0;

function checkRule($label, $expected, $openCode, $playCode, $optionCode, $resultRule) {
    global $failures;
    $actual = LotteryLocalRule::isWin($openCode, $playCode, $optionCode, $resultRule);
    if ($actual !== $expected) {
        $failures++;
        echo "FAIL {$label}: expected " . ($expected ? 'true' : 'false') . ', got ' . ($actual ? 'true' : 'false') . PHP_EOL;
        return;
    }
    echo "ok {$label}" . PHP_EOL;
}

checkRule('k3 sum_size small wins on 1,5,3 sum 9', true, '1,5,3', 'SUM_SIZE', 'SMALL', 'sum_size:1:6');
checkRule('k3 sum_size big loses on 1,5,3 sum 9', false, '1,5,3', 'SUM_SIZE', 'BIG', 'sum_size:1:6');
checkRule('k3 sum_size big wins on 6,6,6 sum 18', true, '6,6,6', 'SUM_SIZE', 'BIG', 'sum_size:1:6');
checkRule('k3 sum_size small loses on 6,6,6 sum 18', false, '6,6,6', 'SUM_SIZE', 'SMALL', 'sum_size:1:6');
checkRule('k3 sum_odd_even odd wins on sum 9', true, '1,5,3', 'SUM_ODD_EVEN', 'ODD', 'sum_odd_even');
checkRule('k3 sum_odd_even even loses on sum 9', false, '1,5,3', 'SUM_ODD_EVEN', 'EVEN', 'sum_odd_even');
checkRule('pc28 threshold small wins at 13', true, '4,4,5', 'SUM_SIZE', 'SMALL', 'sum_size_threshold:13');
checkRule('pc28 threshold big loses at 13', false, '4,4,5', 'SUM_SIZE', 'BIG', 'sum_size_threshold:13');
checkRule('pc28 threshold big wins at 14', true, '4,5,5', 'SUM_SIZE', 'BIG', 'sum_size_threshold:13');
checkRule('digit sum_size big wins above midpoint', true, '9,9,9,9,9', 'SUM_SIZE', 'BIG', 'sum_size:0:9');
checkRule('digit exact_sum wins', true, '9,9,9,9,9', 'SUM_EXACT', 'SUM_45', 'exact_sum');
checkRule('digit exact_sum loses', false, '9,9,9,9,8', 'SUM_EXACT', 'SUM_45', 'exact_sum');
checkRule('position_1 wins first number', true, '3,4,5', 'POSITION_1', 'NUM_3', 'position_1');
checkRule('position_2 loses wrong number', false, '3,4,5', 'POSITION_2', 'NUM_3', 'position_2');
checkRule('dragon tiger unchanged', true, '9,1,2,3,4', 'DRAGON_TIGER_1_5', 'DRAGON', 'dragon_tiger:1:5');
checkRule('contains number unchanged', true, '1,5,3', 'DICE_ANY', '5', 'contains_number');

if ($failures > 0) {
    echo "{$failures} failure(s)" . PHP_EOL;
    exit(1);
}

echo "all tests passed" . PHP_EOL;
