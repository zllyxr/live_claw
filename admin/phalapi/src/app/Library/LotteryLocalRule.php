<?php
namespace App\Library;

class LotteryLocalRule {
    public static function inferConfig($game) {
        $code = strtoupper((string)self::rowValue($game, 'game_code'));
        $name = (string)self::rowValue($game, 'game_name');
        $categoryId = (int)self::rowValue($game, 'category_id');
        $template = self::templateCode($categoryId, $code, $name);

        $config = array(
            'template_code' => $template,
            'draw_count' => 5,
            'number_min' => 0,
            'number_max' => 9,
            'number_unique' => 0,
            'number_pad' => 0,
            'sum_big_threshold' => 0,
            'status' => 1,
        );

        switch ($template) {
            case 'k3':
                $config['draw_count'] = 3;
                $config['number_min'] = 1;
                $config['number_max'] = 6;
                break;
            case 'pk10':
                $config['draw_count'] = 10;
                $config['number_min'] = 1;
                $config['number_max'] = 10;
                $config['number_unique'] = 1;
                $config['number_pad'] = 2;
                break;
            case '11x5':
                $config['draw_count'] = 5;
                $config['number_min'] = 1;
                $config['number_max'] = 11;
                $config['number_unique'] = 1;
                $config['number_pad'] = 2;
                break;
            case 'klsf':
                $config['draw_count'] = 8;
                $config['number_min'] = 1;
                $config['number_max'] = 20;
                $config['number_unique'] = 1;
                $config['number_pad'] = 2;
                break;
            case 'kl8':
                $config['draw_count'] = 20;
                $config['number_min'] = 1;
                $config['number_max'] = 80;
                $config['number_unique'] = 1;
                $config['number_pad'] = 2;
                break;
            case 'lhc':
                $config['draw_count'] = 7;
                $config['number_min'] = 1;
                $config['number_max'] = 49;
                $config['number_unique'] = 1;
                $config['number_pad'] = 2;
                break;
            case 'official':
                $config['draw_count'] = 7;
                $config['number_min'] = 1;
                $config['number_max'] = 35;
                $config['number_unique'] = 1;
                $config['number_pad'] = 2;
                break;
            case 'pc28':
                $config['draw_count'] = 3;
                $config['number_min'] = 0;
                $config['number_max'] = 9;
                break;
            case 'ssc':
            case 'digit':
            default:
                $config['draw_count'] = self::drawCountForGame($categoryId, $code);
                $range = self::numberRangeForGame($categoryId, $code, $name);
                $config['number_min'] = $range[0];
                $config['number_max'] = $range[1];
                $config['number_pad'] = $range[1] >= 10 && $range[0] > 0 ? 2 : 0;
                break;
        }

        return $config;
    }

    public static function normalizeConfig($config, $game = array()) {
        $defaults = self::inferConfig($game);
        if (is_array($config)) {
            foreach ($config as $key => $value) {
                if ($value !== null && $value !== '') {
                    $defaults[$key] = $value;
                }
            }
        }

        $defaults['template_code'] = trim((string)$defaults['template_code']);
        if ($defaults['template_code'] === '') {
            $defaults['template_code'] = 'digit';
        }
        $defaults['draw_count'] = max(1, min(80, (int)$defaults['draw_count']));
        $defaults['number_min'] = (int)$defaults['number_min'];
        $defaults['number_max'] = (int)$defaults['number_max'];
        if ($defaults['number_max'] < $defaults['number_min']) {
            $tmp = $defaults['number_min'];
            $defaults['number_min'] = $defaults['number_max'];
            $defaults['number_max'] = $tmp;
        }
        $defaults['number_unique'] = (int)$defaults['number_unique'] === 1 ? 1 : 0;
        $defaults['number_pad'] = max(0, min(4, (int)$defaults['number_pad']));
        $defaults['sum_big_threshold'] = max(0, (int)$defaults['sum_big_threshold']);
        $defaults['status'] = (int)$defaults['status'] === 0 ? 0 : 1;

        $rangeSize = $defaults['number_max'] - $defaults['number_min'] + 1;
        if ($defaults['number_unique'] === 1 && $defaults['draw_count'] > $rangeSize) {
            $defaults['draw_count'] = $rangeSize;
        }

        return $defaults;
    }

    public static function templateCode($categoryId, $gameCode, $gameName = '') {
        $code = strtoupper((string)$gameCode);
        $text = strtolower($code . ' ' . (string)$gameName);
        if ((int)$categoryId === 4 || strpos($text, 'k3') !== false || strpos($text, 'ks') !== false || strpos($text, '快三') !== false) {
            return 'k3';
        }
        if (strpos($text, 'pk10') !== false || strpos($text, 'ft') !== false || strpos($text, '飞艇') !== false || strpos($text, '赛车') !== false || strpos($text, 'racing') !== false) {
            return 'pk10';
        }
        if ((int)$categoryId === 3 || strpos($text, 'syxw') !== false || strpos($text, '11x5') !== false || strpos($text, '11选5') !== false) {
            return '11x5';
        }
        if ((int)$categoryId === 5 || strpos($text, 'klsf') !== false || strpos($text, '快乐十分') !== false || strpos($text, '农场') !== false) {
            return 'klsf';
        }
        if (strpos($text, 'kl8') !== false || strpos($text, 'bingo') !== false || strpos($text, '賓果') !== false) {
            return 'kl8';
        }
        if (strpos($text, 'lhc') !== false || strpos($text, 'hk6') !== false || strpos($text, 'macau6') !== false || strpos($text, '六合彩') !== false) {
            return 'lhc';
        }
        if (strpos($text, '28') !== false || strpos($text, 'pcdd') !== false) {
            return 'pc28';
        }
        if (in_array($code, array('SSQ', 'QLC', 'DLT', 'QXC'), true)) {
            return 'official';
        }
        if (strpos($text, 'ssc') !== false || strpos($text, 'ffc') !== false || strpos($text, '5fc') !== false || strpos($text, '分分彩') !== false || strpos($text, '时时彩') !== false) {
            return 'ssc';
        }
        return 'digit';
    }

    public static function drawCountForGame($categoryId, $gameCode) {
        $code = strtoupper((string)$gameCode);
        if ((int)$categoryId === 4 || strpos($code, 'K3') !== false || strpos($code, 'KS') !== false) {
            return 3;
        }
        if (strpos($code, 'PK10') !== false || strpos($code, 'FT') !== false || strpos($code, 'RACING') !== false) {
            return 10;
        }
        if ((int)$categoryId === 3 || strpos($code, 'SYXW') !== false) {
            return 5;
        }
        if ((int)$categoryId === 5 || strpos($code, 'KLSF') !== false || strpos($code, 'KL10') !== false || strpos($code, 'XYNC') !== false) {
            return 8;
        }
        if (strpos($code, 'KL8') !== false || strpos($code, 'BINGO') !== false || strpos($code, 'XY20') !== false) {
            return 20;
        }
        if (strpos($code, 'LHC') !== false || strpos($code, 'HK6') !== false || strpos($code, 'MACAU6') !== false) {
            return 7;
        }
        if (in_array($code, array('FC3D', 'PLS'), true)) {
            return 3;
        }
        if (in_array($code, array('SSQ', 'QLC', 'DLT', 'QXC'), true)) {
            return 7;
        }
        return 5;
    }

    public static function numberRangeForGame($categoryId, $gameCode, $gameName = '') {
        $code = strtoupper((string)$gameCode);
        $text = strtolower($code . ' ' . (string)$gameName);
        if ((int)$categoryId === 4 || strpos($text, 'k3') !== false || strpos($text, 'ks') !== false || strpos($text, '快三') !== false) {
            return array(1, 6);
        }
        if ((int)$categoryId === 3 || strpos($text, 'syxw') !== false || strpos($text, '11选5') !== false) {
            return array(1, 11);
        }
        if (strpos($text, 'pk10') !== false || strpos($text, 'ft') !== false || strpos($text, '飞艇') !== false || strpos($text, '赛车') !== false || strpos($text, 'racing') !== false) {
            return array(1, 10);
        }
        if (strpos($text, 'lhc') !== false || strpos($text, 'hk6') !== false || strpos($text, 'macau6') !== false || strpos($text, '六合彩') !== false) {
            return array(1, 49);
        }
        if (strpos($text, 'kl8') !== false || strpos($text, 'bingo') !== false || strpos($text, '賓果') !== false) {
            return array(1, 80);
        }
        if ((int)$categoryId === 5 || strpos($text, 'klsf') !== false || strpos($text, '快乐十分') !== false || strpos($text, '农场') !== false) {
            return array(1, 20);
        }
        if (in_array($code, array('SSQ', 'QLC', 'DLT', 'QXC'), true)) {
            return array(1, 35);
        }
        if (strpos($text, '28') !== false || in_array($code, array('FC3D', 'PLS', 'PLW'), true)) {
            return array(0, 9);
        }
        return array(0, 9);
    }

    public static function generateOpenCode($config) {
        $config = self::normalizeConfig($config);
        $min = (int)$config['number_min'];
        $max = (int)$config['number_max'];
        $count = (int)$config['draw_count'];
        $pad = (int)$config['number_pad'];
        $numbers = array();

        if ((int)$config['number_unique'] === 1) {
            $pool = range($min, $max);
            for ($i = 0; $i < $count && count($pool) > 0; $i++) {
                $index = random_int(0, count($pool) - 1);
                $numbers[] = $pool[$index];
                array_splice($pool, $index, 1);
            }
        } else {
            for ($i = 0; $i < $count; $i++) {
                $numbers[] = random_int($min, $max);
            }
        }

        $labels = array();
        foreach ($numbers as $number) {
            $labels[] = self::numberLabel($number, $pad);
        }

        return implode(',', $labels);
    }

    public static function validateOpenCode($openCode, $config) {
        $config = self::normalizeConfig($config);
        $numbers = self::parseOpenCode($openCode);
        if (count($numbers) !== (int)$config['draw_count']) {
            return false;
        }
        $seen = array();
        foreach ($numbers as $number) {
            if ($number < (int)$config['number_min'] || $number > (int)$config['number_max']) {
                return false;
            }
            if ((int)$config['number_unique'] === 1) {
                if (isset($seen[$number])) {
                    return false;
                }
                $seen[$number] = true;
            }
        }
        return true;
    }

    public static function normalizeOpenCode($openCode, $config) {
        $config = self::normalizeConfig($config);
        $numbers = self::parseOpenCode($openCode);
        $labels = array();
        foreach ($numbers as $number) {
            $labels[] = self::numberLabel($number, (int)$config['number_pad']);
        }
        return implode(',', $labels);
    }

    public static function templatePlays($config, $game = array()) {
        $config = self::normalizeConfig($config, $game);
        $template = (string)$config['template_code'];
        $count = (int)$config['draw_count'];
        $plays = array();

        if ($template === 'k3') {
            $plays[] = self::play('DICE_ANY', '单骰', 'contains_number', 100, self::numberOptions(1, 6, 0, '1.9800'));
            $plays[] = self::play('TRIPLE_ANY', '任意三同号', 'k3_triple_any', 90, array(array('ANY', '三同号通选', '30.0000', 100)));
            $plays[] = self::play('TRIPLE_EXACT', '指定三同号', 'k3_triple_exact', 80, self::repeatOptions(1, 6, 3, '150.0000'));
            $plays[] = self::play('PAIR_EXACT', '指定二同号', 'k3_pair_exact', 70, self::repeatOptions(1, 6, 2, '8.0000'));
            return $plays;
        }

        if ($template === 'pk10') {
            $plays[] = self::play('DRAGON_TIGER_1_10', '龙虎 1V10', 'dragon_tiger:1:10', 100, self::dragonTigerOptions(false));
            return $plays;
        }

        if ($template === 'pc28') {
            $plays[] = self::play('PC28_EXTREME', '极值', 'pc28_extreme', 100, array(
                array('EXTREME_BIG', '极大', '10.0000', 100),
                array('EXTREME_SMALL', '极小', '10.0000', 90),
            ));
            return $plays;
        }

        if ($template === 'lhc') {
            $plays[] = self::play('SPECIAL_NUMBER', '特码', 'lhc_special_number', 100, self::numberOptions(1, 49, 2, '45.0000'));
            $plays[] = self::play('SPECIAL_SIZE', '特码大小', 'lhc_special_size', 90, self::binaryOptions());
            $plays[] = self::play('SPECIAL_ODD_EVEN', '特码单双', 'lhc_special_odd_even', 80, self::oddEvenOptions());
            $plays[] = self::play('SPECIAL_COLOR', '特码色波', 'lhc_special_color', 70, array(
                array('RED', '红波', '2.8000', 100),
                array('BLUE', '蓝波', '2.8000', 90),
                array('GREEN', '绿波', '2.8000', 80),
            ));
            $plays[] = self::play('SPECIAL_ZODIAC', '特码生肖', 'lhc_special_zodiac', 60, self::zodiacOptions());
            return $plays;
        }

        if ($count >= 2) {
            $plays[] = self::play('DRAGON_TIGER_1_' . $count, '龙虎和', 'dragon_tiger:1:' . $count, 100, self::dragonTigerOptions(true));
        }

        return $plays;
    }

    public static function isWin($openCode, $playCode, $optionCode, $resultRule = '') {
        $numbers = self::parseOpenCode($openCode);
        if (count($numbers) < 1) {
            return false;
        }

        $playCode = strtoupper((string)$playCode);
        $optionCode = strtoupper((string)$optionCode);
        $rule = strtolower(trim((string)$resultRule));

        $ruleParts = explode(':', $rule);
        $baseRule = $ruleParts[0];

        if ($baseRule === 'dragon_tiger') {
            $left = isset($ruleParts[1]) ? (int)$ruleParts[1] - 1 : 0;
            $right = isset($ruleParts[2]) ? (int)$ruleParts[2] - 1 : count($numbers) - 1;
            if (!isset($numbers[$left]) || !isset($numbers[$right])) {
                return false;
            }
            if ((int)$numbers[$left] > (int)$numbers[$right]) {
                return $optionCode === 'DRAGON';
            }
            if ((int)$numbers[$left] < (int)$numbers[$right]) {
                return $optionCode === 'TIGER';
            }
            return $optionCode === 'TIE';
        }

        if ($baseRule === 'sum_size') {
            $sum = array_sum($numbers);
            $min = isset($ruleParts[1]) && is_numeric($ruleParts[1]) ? (int)$ruleParts[1] : 0;
            $max = isset($ruleParts[2]) && is_numeric($ruleParts[2]) ? (int)$ruleParts[2] : 9;
            if ($max < $min) {
                $tmp = $min;
                $min = $max;
                $max = $tmp;
            }
            $threshold = (int)floor(count($numbers) * ($min + $max) / 2);
            return ($optionCode === 'BIG' && $sum > $threshold)
                || ($optionCode === 'SMALL' && $sum <= $threshold);
        }

        if ($baseRule === 'sum_size_threshold') {
            $sum = array_sum($numbers);
            $threshold = isset($ruleParts[1]) && is_numeric($ruleParts[1]) ? (int)$ruleParts[1] : 0;
            return ($optionCode === 'BIG' && $sum > $threshold)
                || ($optionCode === 'SMALL' && $sum <= $threshold);
        }

        if ($baseRule === 'sum_odd_even') {
            $sum = array_sum($numbers);
            return ($optionCode === 'ODD' && $sum % 2 === 1)
                || ($optionCode === 'EVEN' && $sum % 2 === 0);
        }

        if ($baseRule === 'exact_sum') {
            $target = self::optionNumber($optionCode);
            return $target !== null && array_sum($numbers) === $target;
        }

        if (strpos($baseRule, 'position_') === 0) {
            $position = (int)substr($baseRule, 9) - 1;
            $target = self::optionNumber($optionCode);
            return $target !== null && isset($numbers[$position]) && (int)$numbers[$position] === $target;
        }

        if ($baseRule === 'contains_number') {
            $target = self::optionNumber($optionCode);
            return $target !== null && in_array($target, $numbers, true);
        }

        if ($baseRule === 'k3_triple_any') {
            return count($numbers) >= 3 && $numbers[0] === $numbers[1] && $numbers[1] === $numbers[2];
        }

        if ($baseRule === 'k3_triple_exact') {
            $target = self::optionNumber($optionCode);
            if ($target === null) {
                return false;
            }
            $digit = (int)substr((string)$target, 0, 1);
            return count($numbers) >= 3 && $numbers[0] === $digit && $numbers[1] === $digit && $numbers[2] === $digit;
        }

        if ($baseRule === 'k3_pair_exact') {
            $target = self::optionNumber($optionCode);
            if ($target === null) {
                return false;
            }
            $digit = (int)substr((string)$target, 0, 1);
            $hits = 0;
            foreach ($numbers as $number) {
                if ((int)$number === $digit) {
                    $hits++;
                }
            }
            return $hits >= 2;
        }

        if ($baseRule === 'pc28_extreme') {
            $sum = array_sum($numbers);
            return ($optionCode === 'EXTREME_BIG' && $sum >= 22)
                || ($optionCode === 'EXTREME_SMALL' && $sum <= 5);
        }

        if ($baseRule === 'lhc_special_number' || $baseRule === 'lhc_special_size' || $baseRule === 'lhc_special_odd_even' || $baseRule === 'lhc_special_color' || $baseRule === 'lhc_special_zodiac') {
            $special = (int)$numbers[count($numbers) - 1];
            if ($baseRule === 'lhc_special_number') {
                $target = self::optionNumber($optionCode);
                return $target !== null && $special === $target;
            }
            if ($baseRule === 'lhc_special_size') {
                return ($optionCode === 'BIG' && $special >= 25)
                    || ($optionCode === 'SMALL' && $special <= 24);
            }
            if ($baseRule === 'lhc_special_odd_even') {
                return ($optionCode === 'ODD' && $special % 2 === 1)
                    || ($optionCode === 'EVEN' && $special % 2 === 0);
            }
            if ($baseRule === 'lhc_special_color') {
                return $optionCode === self::lhcColor($special);
            }
            return $optionCode === self::lhcZodiac($special);
        }

        return false;
    }

    public static function isDeprecatedPlay($playCode, $resultRule) {
        $code = strtoupper(trim((string)$playCode));
        $baseRule = strtolower(explode(':', trim((string)$resultRule))[0]);
        if (strpos($code, 'POSITION_') === 0) {
            return true;
        }
        if (in_array($code, array(
            'SUM_SIZE',
            'SUM_ODD_EVEN',
            'SUM_EXACT',
            'CHAMPION',
            'RUNNER_UP',
            'CHAMPION_SIZE',
            'CHAMPION_ODD_EVEN',
            'TOP2_SUM_SIZE',
            'TOP2_SUM_ODD_EVEN',
        ), true)) {
            return true;
        }
        return $baseRule === 'exact_sum'
            || strpos($baseRule, 'sum_') === 0
            || strpos($baseRule, 'position_') === 0;
    }

    public static function parseOpenCode($openCode) {
        $parts = preg_split('/,|，|\s+/', trim((string)$openCode));
        $numbers = array();
        foreach ($parts as $part) {
            if ($part !== '' && is_numeric($part)) {
                $numbers[] = (int)$part;
            }
        }
        return $numbers;
    }

    public static function optionNumber($optionCode) {
        $optionCode = strtoupper(trim((string)$optionCode));
        if ($optionCode === '') {
            return null;
        }
        if (is_numeric($optionCode)) {
            return (int)$optionCode;
        }
        if (preg_match('/(\d+)$/', $optionCode, $matches)) {
            return (int)$matches[1];
        }
        return null;
    }

    public static function lhcColor($number) {
        $red = array(1, 2, 7, 8, 12, 13, 18, 19, 23, 24, 29, 30, 34, 35, 40, 45, 46);
        $blue = array(3, 4, 9, 10, 14, 15, 20, 25, 26, 31, 36, 37, 41, 42, 47, 48);
        if (in_array((int)$number, $red, true)) {
            return 'RED';
        }
        if (in_array((int)$number, $blue, true)) {
            return 'BLUE';
        }
        return 'GREEN';
    }

    public static function lhcZodiac($number, $year = null) {
        $animals = array('RAT', 'OX', 'TIGER', 'RABBIT', 'DRAGON', 'SNAKE', 'HORSE', 'GOAT', 'MONKEY', 'ROOSTER', 'DOG', 'PIG');
        $year = $year === null ? (int)date('Y') : (int)$year;
        $yearIndex = (($year - 2020) % 12 + 12) % 12;
        $index = ($yearIndex - ((int)$number - 1)) % 12;
        if ($index < 0) {
            $index += 12;
        }
        return $animals[$index];
    }

    protected static function play($code, $name, $rule, $sort, $options) {
        return array(
            'play_code' => $code,
            'play_name' => $name,
            'result_rule' => $rule,
            'sort' => $sort,
            'options' => $options,
        );
    }

    protected static function binaryOptions() {
        return array(
            array('BIG', '大', '1.9500', 100),
            array('SMALL', '小', '1.9500', 90),
        );
    }

    protected static function oddEvenOptions() {
        return array(
            array('ODD', '单', '1.9500', 100),
            array('EVEN', '双', '1.9500', 90),
        );
    }

    protected static function dragonTigerOptions($withTie) {
        $options = array(
            array('DRAGON', '龙', '1.9500', 100),
            array('TIGER', '虎', '1.9500', 90),
        );
        if ($withTie) {
            $options[] = array('TIE', '和', '9.8000', 80);
        }
        return $options;
    }

    protected static function numberOptions($min, $max, $pad = 0, $odds = '') {
        $options = array();
        $min = (int)$min;
        $max = (int)$max;
        if ($odds === '') {
            $odds = number_format(max(1.95, ($max - $min + 1) * 0.95), 4, '.', '');
        }
        for ($i = $min; $i <= $max; $i++) {
            $label = self::numberLabel($i, $pad);
            $options[] = array($label, $label, $odds, 1000 - $i);
        }
        return $options;
    }

    protected static function repeatOptions($min, $max, $repeat, $odds) {
        $options = array();
        for ($i = (int)$min; $i <= (int)$max; $i++) {
            $label = str_repeat((string)$i, (int)$repeat);
            $options[] = array($label, $label, $odds, 1000 - $i);
        }
        return $options;
    }

    protected static function zodiacOptions() {
        return array(
            array('RAT', '鼠', '11.0000', 120),
            array('OX', '牛', '11.0000', 110),
            array('TIGER', '虎', '11.0000', 100),
            array('RABBIT', '兔', '11.0000', 90),
            array('DRAGON', '龙', '11.0000', 80),
            array('SNAKE', '蛇', '11.0000', 70),
            array('HORSE', '马', '11.0000', 60),
            array('GOAT', '羊', '11.0000', 50),
            array('MONKEY', '猴', '11.0000', 40),
            array('ROOSTER', '鸡', '11.0000', 30),
            array('DOG', '狗', '11.0000', 20),
            array('PIG', '猪', '11.0000', 10),
        );
    }

    protected static function numberLabel($number, $pad) {
        $number = (int)$number;
        $pad = (int)$pad;
        if ($pad > 0) {
            return str_pad((string)$number, $pad, '0', STR_PAD_LEFT);
        }
        return (string)$number;
    }

    protected static function rowValue($row, $key, $fallback = '') {
        if (is_array($row) && array_key_exists($key, $row)) {
            return $row[$key];
        }
        if ($row instanceof \ArrayAccess && isset($row[$key])) {
            return $row[$key];
        }
        if (is_object($row) && isset($row->{$key})) {
            return $row->{$key};
        }
        return $fallback;
    }
}
