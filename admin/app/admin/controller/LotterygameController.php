<?php
namespace app\admin\controller;

use App\Library\LotteryLocalRule;
use cmf\controller\AdminBaseController;
use think\facade\Db;

require_once CMF_ROOT . 'phalapi/src/app/Library/LotteryLocalRule.php';

class LotterygameController extends AdminBaseController {
    const LOCAL_LOTTERY_ICON_BASE = '/static/appapi/lottery-flat-icons/png/icons_128/';
    const LOCAL_LOTTERY_DEFAULT_ICON = 'lottery_default.png';

    protected function navTabs($active = 'index') {
        $tabs = array(
            'category' => array('title' => '分类管理', 'url' => url('Lotterygame/categories')),
            'index' => array('title' => '游戏管理', 'url' => url('Lotterygame/index')),
            'issues' => array('title' => '期号管理', 'url' => url('Lotterygame/issues')),
            'orders' => array('title' => '注单管理', 'url' => url('Lotterygame/orders')),
        );
        $this->assign('nav_tabs', $tabs);
        $this->assign('active_tab', $active);
    }

    protected function gameStatus($key = '') {
        $items = array(
            '0' => '隐藏',
            '1' => '启用',
            '2' => '维护',
        );
        return $key === '' ? $items : ($items[(string)$key] ?? '');
    }

    protected function issueStatus($key = '') {
        $items = array(
            '0' => '开盘中',
            '1' => '已封盘',
            '2' => '已开奖',
            '3' => '已结算',
            '4' => '已取消',
        );
        return $key === '' ? $items : ($items[(string)$key] ?? '');
    }

    protected function orderStatus($key = '') {
        $items = array(
            '0' => '待结算',
            '1' => '已中奖',
            '2' => '未中奖',
            '3' => '已退款',
            '4' => '已取消',
        );
        return $key === '' ? $items : ($items[(string)$key] ?? '');
    }

    protected function playRule($key = '') {
        $items = array(
            'dragon_tiger' => '龙虎和',
            'contains_number' => '包含号码',
            'k3_triple_any' => '快三任意三同号',
            'k3_triple_exact' => '快三指定三同号',
            'k3_pair_exact' => '快三指定二同号',
            'pc28_extreme' => 'PC28 极值',
            'lhc_special_number' => '六合彩特码',
            'lhc_special_size' => '六合彩特码大小',
            'lhc_special_odd_even' => '六合彩特码单双',
            'lhc_special_color' => '六合彩特码色波',
            'lhc_special_zodiac' => '六合彩特码生肖',
            'manual' => '手动结算/暂不自动结算',
        );
        return $key === '' ? $items : ($items[(string)$key] ?? '');
    }

    protected function playRuleName($rule) {
        $rule = trim((string)$rule);
        if ($rule === '') {
            return '';
        }
        $items = $this->playRule();
        if (isset($items[$rule])) {
            return $items[$rule];
        }
        $base = explode(':', $rule)[0];
        return isset($items[$base]) ? $items[$base] . ' / ' . $rule : $rule;
    }

    protected function isSupportedPlayRule($rule) {
        $rule = trim((string)$rule);
        if ($rule === '') {
            return false;
        }
        $items = $this->playRule();
        if (isset($items[$rule])) {
            return true;
        }
        $base = explode(':', $rule)[0];
        return isset($items[$base]);
    }

    protected function templateCodes() {
        return array(
            'ssc' => '时时彩/分分彩',
            'pk10' => 'PK10/飞艇/赛车',
            'k3' => '快三',
            'pc28' => 'PC28',
            'lhc' => '六合彩',
            '11x5' => '11选5',
            'klsf' => '快乐十分',
            'kl8' => '快乐8',
            'official' => '福彩/体彩',
            'digit' => '数字型',
        );
    }

    protected function getCategories() {
        return Db::name('lottery_category')->order('sort desc,id asc')->select();
    }

    protected function normalizeIcon($icon) {
        $icon = trim((string)$icon);
        if ($icon === '') {
            return '';
        }
        if (preg_match('/^(local|minio|qiniu|aws)_/i', $icon)) {
            return $icon;
        }
        if (strpos($icon, 'http://') === 0 || strpos($icon, 'https://') === 0) {
            return html_entity_decode(htmlspecialchars_decode($icon));
        }
        if (strpos($icon, '/') === 0 && strpos($icon, '/upload/') !== 0) {
            return $icon;
        }
        if (preg_match('/^[a-z0-9_-]+$/i', $icon)) {
            return $icon;
        }
        return set_upload_path($icon);
    }

    protected function iconPreviewUrl($icon) {
        $icon = trim((string)$icon);
        if ($icon === '') {
            return '';
        }
        if (preg_match('/^[a-z0-9_-]+$/i', $icon)) {
            return $this->localLotteryIconPath($icon);
        }
        return get_upload_path($icon);
    }

    protected function localLotteryIconPath($icon) {
        $key = strtolower(trim((string)$icon));
        if ($key === '') {
            return self::LOCAL_LOTTERY_ICON_BASE . self::LOCAL_LOTTERY_DEFAULT_ICON;
        }

        $map = $this->localLotteryIconMap();
        $file = isset($map[$key]) ? $map[$key] : self::LOCAL_LOTTERY_DEFAULT_ICON;
        return self::LOCAL_LOTTERY_ICON_BASE . $file;
    }

    protected function localLotteryIconMap() {
        static $map = null;
        if ($map !== null) {
            return $map;
        }

        $map = array(
            'hot' => self::LOCAL_LOTTERY_DEFAULT_ICON,
            'live' => self::LOCAL_LOTTERY_DEFAULT_ICON,
            'card' => 'FC3D__3D.png',
            'fishing' => 'JISUKL8__8.png',
            'arcade' => 'BJPK10__.png',
            'sports' => self::LOCAL_LOTTERY_DEFAULT_ICON,
            'ol' => self::LOCAL_LOTTERY_DEFAULT_ICON,
            'lottery' => self::LOCAL_LOTTERY_DEFAULT_ICON,
            'default' => self::LOCAL_LOTTERY_DEFAULT_ICON,
        );

        $dir = CMF_ROOT . 'public' . self::LOCAL_LOTTERY_ICON_BASE;
        if (is_dir($dir)) {
            $files = glob($dir . '*.png');
            if (is_array($files)) {
                foreach ($files as $file) {
                    $filename = basename($file);
                    $name = pathinfo($filename, PATHINFO_FILENAME);
                    $code = strtolower(preg_replace('/[^a-z0-9].*$/i', '', $name));
                    if ($code !== '') {
                        $map[$code] = $filename;
                    }
                }
            }
        }

        return $map;
    }

    protected function localLotteryIconChoices() {
        $choices = array();
        foreach ($this->localLotteryIconMap() as $key => $file) {
            $choices[] = array(
                'key' => $key,
                'label' => strtoupper($key) . ' / ' . $file,
                'url' => self::LOCAL_LOTTERY_ICON_BASE . $file,
            );
        }
        usort($choices, function ($a, $b) {
            return strcmp($a['key'], $b['key']);
        });
        return $choices;
    }

    protected function getDrawConfig($game) {
        $config = array();
        try {
            $config = Db::name('lottery_draw_config')->where('game_id', (int)$game['id'])->find();
        } catch (\Throwable $e) {
            $config = array();
        }
        return LotteryLocalRule::normalizeConfig($config ?: array(), $game);
    }

    protected function saveDrawConfig($gameId, $data, $game) {
        $config = LotteryLocalRule::normalizeConfig(array(
            'template_code' => trim($data['template_code'] ?? ''),
            'draw_count' => (int)($data['draw_count'] ?? 0),
            'number_min' => (int)($data['number_min'] ?? 0),
            'number_max' => (int)($data['number_max'] ?? 0),
            'number_unique' => (int)($data['number_unique'] ?? 0),
            'number_pad' => (int)($data['number_pad'] ?? 0),
            'sum_big_threshold' => 0,
            'status' => 1,
        ), $game);

        $now = time();
        $row = array(
            'game_id' => (int)$gameId,
            'draw_mode' => 'local_auto',
            'template_code' => $config['template_code'],
            'draw_count' => $config['draw_count'],
            'number_min' => $config['number_min'],
            'number_max' => $config['number_max'],
            'number_unique' => $config['number_unique'],
            'number_pad' => $config['number_pad'],
            'sum_big_threshold' => $config['sum_big_threshold'],
            'status' => 1,
            'update_time' => $now,
        );

        try {
            $exists = Db::name('lottery_draw_config')->where('game_id', (int)$gameId)->find();
            if ($exists) {
                Db::name('lottery_draw_config')->where('game_id', (int)$gameId)->update($row);
            } else {
                $row['create_time'] = $now;
                Db::name('lottery_draw_config')->insert($row);
            }
        } catch (\Throwable $e) {
        }
    }

    protected function upsertPlayWithOptions($gameId, $play, $options, $now) {
        $playData = array(
            'game_id' => (int)$gameId,
            'play_code' => (string)$play['play_code'],
            'play_name' => (string)$play['play_name'],
            'result_rule' => (string)$play['result_rule'],
            'sort' => (int)$play['sort'],
            'status' => 1,
            'update_time' => $now,
        );
        $exists = Db::name('lottery_play')
            ->where('game_id', (int)$gameId)
            ->where('play_code', $playData['play_code'])
            ->find();
        if ($exists) {
            Db::name('lottery_play')->where('id', $exists['id'])->update($playData);
            $playId = (int)$exists['id'];
        } else {
            $playData['create_time'] = $now;
            $playId = Db::name('lottery_play')->insertGetId($playData);
        }

        $count = 1;
        foreach ($options as $option) {
            $optionData = array(
                'play_id' => $playId,
                'option_code' => (string)$option[0],
                'option_name' => (string)$option[1],
                'odds' => (string)$option[2],
                'sort' => (int)$option[3],
                'status' => 1,
                'update_time' => $now,
            );
            $existsOption = Db::name('lottery_option')
                ->where('play_id', $playId)
                ->where('option_code', $optionData['option_code'])
                ->find();
            if ($existsOption) {
                Db::name('lottery_option')->where('id', $existsOption['id'])->update($optionData);
            } else {
                $optionData['create_time'] = $now;
                Db::name('lottery_option')->insert($optionData);
            }
            $count++;
        }

        return $count;
    }

    protected function generateTemplatesForGame($game) {
        $now = time();
        $config = $this->getDrawConfig($game);
        $plays = LotteryLocalRule::templatePlays($config, $game);
        $count = 0;
        foreach ($plays as $play) {
            $options = $play['options'] ?? array();
            unset($play['options']);
            $count += $this->upsertPlayWithOptions((int)$game['id'], $play, $options, $now);
        }
        return $count + $this->disableDeprecatedLocalPlays((int)$game['id'], $now);
    }

    protected function disableDeprecatedLocalPlays($gameId, $now) {
        $count = 0;
        $plays = Db::name('lottery_play')
            ->where('game_id', (int)$gameId)
            ->where('status', 1)
            ->select()
            ->toArray();
        foreach ($plays as $play) {
            if (!LotteryLocalRule::isDeprecatedPlay($play['play_code'] ?? '', $play['result_rule'] ?? '')) {
                continue;
            }
            Db::name('lottery_play')
                ->where('id', (int)$play['id'])
                ->update(array('status' => 0, 'update_time' => $now));
            $count++;
        }
        return $count;
    }

    public function index() {
        $data = $this->request->param();
        $map = array();

        $status = $data['status'] ?? '';
        if ($status !== '') {
            $map[] = array('status', '=', (int)$status);
        }

        $keyword = trim($data['keyword'] ?? '');
        if ($keyword !== '') {
            $map[] = array('game_name|game_code', 'like', '%' . $keyword . '%');
        }

        $list = Db::name('lottery_game')
            ->where($map)
            ->order('sort desc,id desc')
            ->paginate(20);

        $list->each(function ($item) {
            $item['category'] = Db::name('lottery_category')->where('id', $item['category_id'])->find();
            $item['icon_url'] = $this->iconPreviewUrl($item['icon']);
            $item['draw_config'] = $this->getDrawConfig($item);
            $item['current_issue'] = Db::name('lottery_issue')
                ->where('game_id', $item['id'])
                ->where('status', 'in', [0, 1])
                ->order('open_time asc,id asc')
                ->find();
            return $item;
        });

        $list->appends($data);
        $this->assign('list', $list);
        $this->assign('page', $list->render());
        $this->assign('game_status', $this->gameStatus());
        $this->navTabs('index');
        return $this->fetch();
    }

    public function addGame() {
        $data = array(
            'category_id' => 1,
            'game_code' => '',
            'game_name' => '',
            'game_name_en' => '',
            'icon' => '',
            'icon_url' => '',
            'description' => '',
            'rule_desc' => '',
            'interval_sec' => 60,
            'seal_advance_sec' => 5,
            'min_bet' => 10,
            'max_bet' => 500000,
            'max_issue_bet' => 1000000,
            'sort' => 0,
            'status' => 2,
        );
        $data['draw_config'] = LotteryLocalRule::normalizeConfig(array(), $data);
        $this->assign('data', $data);
        $this->assign('categories', $this->getCategories());
        $this->assign('game_status', $this->gameStatus());
        $this->assign('template_codes', $this->templateCodes());
        $this->assign('icon_choices', $this->localLotteryIconChoices());
        $this->assign('is_edit', 0);
        $this->assign('post_url', url('Lotterygame/addGamePost'));
        $this->navTabs('');
        return $this->fetch('game_form');
    }

    public function addGamePost() {
        $this->saveGame();
    }

    public function editGame() {
        $id = $this->request->param('id', 0, 'intval');
        $data = Db::name('lottery_game')->where('id', $id)->find();
        if (!$data) {
            $this->error('游戏不存在');
        }
        $data['icon_url'] = $this->iconPreviewUrl($data['icon']);
        $data['draw_config'] = $this->getDrawConfig($data);

        $this->assign('data', $data);
        $this->assign('categories', $this->getCategories());
        $this->assign('game_status', $this->gameStatus());
        $this->assign('template_codes', $this->templateCodes());
        $this->assign('icon_choices', $this->localLotteryIconChoices());
        $this->assign('is_edit', 1);
        $this->assign('post_url', url('Lotterygame/editGamePost'));
        $this->navTabs('');
        return $this->fetch('game_form');
    }

    public function editGamePost() {
        $this->saveGame();
    }

    protected function saveGame() {
        $data = $this->request->param();
        $id = isset($data['id']) ? (int)$data['id'] : 0;
        $gameCode = strtoupper(trim($data['game_code'] ?? ''));
        $gameName = trim($data['game_name'] ?? '');
        if ($gameCode === '' || $gameName === '') {
            $this->error('请填写游戏编码和名称');
        }

        $row = array(
            'category_id' => (int)($data['category_id'] ?? 0),
            'game_code' => $gameCode,
            'game_name' => $gameName,
            'game_name_en' => trim($data['game_name_en'] ?? ''),
            'icon' => $this->normalizeIcon($data['icon'] ?? ''),
            'description' => trim($data['description'] ?? ''),
            'rule_desc' => trim($data['rule_desc'] ?? ''),
            'interval_sec' => max(1, (int)($data['interval_sec'] ?? 60)),
            'seal_advance_sec' => max(0, (int)($data['seal_advance_sec'] ?? 5)),
            'min_bet' => max(1, (int)($data['min_bet'] ?? 10)),
            'max_bet' => max(1, (int)($data['max_bet'] ?? 500000)),
            'max_issue_bet' => max(1, (int)($data['max_issue_bet'] ?? 1000000)),
            'sort' => (int)($data['sort'] ?? 0),
            'status' => in_array((int)($data['status'] ?? 2), [0, 1, 2], true) ? (int)$data['status'] : 2,
            'update_time' => time(),
        );

        $exists = Db::name('lottery_game')
            ->where('game_code', $gameCode)
            ->where('id', '<>', $id)
            ->find();
        if ($exists) {
            $this->error('游戏编码已存在');
        }

        if ($id > 0) {
            Db::name('lottery_game')->where('id', $id)->update($row);
        } else {
            $row['create_time'] = time();
            $id = Db::name('lottery_game')->insertGetId($row);
        }

        $gameForConfig = array_merge($row, array('id' => $id));
        $this->saveDrawConfig($id, $data, $gameForConfig);

        $this->success('保存成功', url('Lotterygame/index'));
    }

    public function setGameStatus() {
        $id = $this->request->param('id', 0, 'intval');
        $status = $this->request->param('status', 1, 'intval');
        if ($id < 1 || !in_array($status, [0, 1, 2], true)) {
            $this->error('参数错误');
        }

        Db::name('lottery_game')
            ->where('id', $id)
            ->update(array('status' => $status, 'update_time' => time()));

        $this->success('操作成功');
    }

    public function generateLocalTemplates() {
        $id = $this->request->param('id', 0, 'intval');
        $query = Db::name('lottery_game')->where('status', 'in', [1, 2]);
        if ($id > 0) {
            $query = $query->where('id', $id);
        }
        $games = $query->select();
        if ($id > 0 && count($games) < 1) {
            $this->error('游戏不存在');
        }

        $gameCount = 0;
        $itemCount = 0;
        foreach ($games as $game) {
            $itemCount += $this->generateTemplatesForGame($game);
            $gameCount++;
        }

        $this->success('生成完成：游戏 ' . $gameCount . ' 个，玩法/选项 ' . $itemCount . ' 个');
    }

    public function categories() {
        $list = Db::name('lottery_category')
            ->order('sort desc,id asc')
            ->paginate(20);
        $list->each(function ($item) {
            $item['icon_url'] = $this->iconPreviewUrl($item['icon']);
            return $item;
        });

        $list->appends($this->request->param());
        $this->assign('list', $list);
        $this->assign('page', $list->render());
        $this->navTabs('category');
        return $this->fetch();
    }

    public function addCategory() {
        $this->assign('data', array('name' => '', 'name_en' => '', 'icon' => '', 'icon_url' => '', 'sort' => 0, 'status' => 1));
        $this->assign('icon_choices', $this->localLotteryIconChoices());
        $this->assign('is_edit', 0);
        $this->assign('post_url', url('Lotterygame/addCategoryPost'));
        $this->navTabs('');
        return $this->fetch('category_form');
    }

    public function addCategoryPost() {
        $this->saveCategory();
    }

    public function editCategory() {
        $id = $this->request->param('id', 0, 'intval');
        $data = Db::name('lottery_category')->where('id', $id)->find();
        if (!$data) {
            $this->error('分类不存在');
        }
        $data['icon_url'] = $this->iconPreviewUrl($data['icon']);
        $this->assign('data', $data);
        $this->assign('icon_choices', $this->localLotteryIconChoices());
        $this->assign('is_edit', 1);
        $this->assign('post_url', url('Lotterygame/editCategoryPost'));
        $this->navTabs('');
        return $this->fetch('category_form');
    }

    public function editCategoryPost() {
        $this->saveCategory();
    }

    protected function saveCategory() {
        $data = $this->request->param();
        $id = isset($data['id']) ? (int)$data['id'] : 0;
        $name = trim($data['name'] ?? '');
        if ($name === '') {
            $this->error('请填写分类名称');
        }
        $row = array(
            'name' => $name,
            'name_en' => trim($data['name_en'] ?? ''),
            'icon' => $this->normalizeIcon($data['icon'] ?? ''),
            'sort' => (int)($data['sort'] ?? 0),
            'status' => (int)($data['status'] ?? 1) === 1 ? 1 : 0,
            'update_time' => time(),
        );
        if ($id > 0) {
            Db::name('lottery_category')->where('id', $id)->update($row);
        } else {
            $row['create_time'] = time();
            Db::name('lottery_category')->insert($row);
        }
        $this->success('保存成功', url('Lotterygame/categories'));
    }

    public function setCategoryStatus() {
        $id = $this->request->param('id', 0, 'intval');
        $status = $this->request->param('status', 1, 'intval') === 1 ? 1 : 0;
        if ($id < 1) {
            $this->error('参数错误');
        }
        Db::name('lottery_category')->where('id', $id)->update(array('status' => $status, 'update_time' => time()));
        $this->success('操作成功');
    }

    public function plays() {
        $gameId = $this->request->param('game_id', 0, 'intval');
        $game = Db::name('lottery_game')->where('id', $gameId)->find();
        if (!$game) {
            $this->error('游戏不存在');
        }

        $list = Db::name('lottery_play')
            ->where('game_id', $gameId)
            ->order('sort desc,id asc')
            ->paginate(20);

        $list->each(function ($item) {
            $item['options_count'] = Db::name('lottery_option')->where('play_id', $item['id'])->count();
            $item['result_rule_name'] = $this->playRuleName($item['result_rule']);
            return $item;
        });

        $list->appends($this->request->param());
        $this->assign('game', $game);
        $this->assign('list', $list);
        $this->assign('page', $list->render());
        $this->assign('play_rule', $this->playRule());
        $this->navTabs('index');
        return $this->fetch();
    }

    public function addPlay() {
        $gameId = $this->request->param('game_id', 0, 'intval');
        $game = Db::name('lottery_game')->where('id', $gameId)->find();
        if (!$game) {
            $this->error('游戏不存在');
        }
        $this->assign('game', $game);
        $this->assign('data', array('game_id' => $gameId, 'play_code' => '', 'play_name' => '', 'result_rule' => 'manual', 'sort' => 0, 'status' => 1));
        $this->assign('play_rule', $this->playRule());
        $this->assign('is_edit', 0);
        $this->assign('post_url', url('Lotterygame/addPlayPost'));
        $this->navTabs('');
        return $this->fetch('play_form');
    }

    public function addPlayPost() {
        $this->savePlay();
    }

    public function editPlay() {
        $id = $this->request->param('id', 0, 'intval');
        $data = Db::name('lottery_play')->where('id', $id)->find();
        if (!$data) {
            $this->error('玩法不存在');
        }
        $game = Db::name('lottery_game')->where('id', $data['game_id'])->find();
        $playRule = $this->playRule();
        if (!isset($playRule[$data['result_rule']])) {
            $playRule[$data['result_rule']] = $this->playRuleName($data['result_rule']);
        }
        $this->assign('game', $game);
        $this->assign('data', $data);
        $this->assign('play_rule', $playRule);
        $this->assign('is_edit', 1);
        $this->assign('post_url', url('Lotterygame/editPlayPost'));
        $this->navTabs('');
        return $this->fetch('play_form');
    }

    public function editPlayPost() {
        $this->savePlay();
    }

    protected function savePlay() {
        $data = $this->request->param();
        $id = isset($data['id']) ? (int)$data['id'] : 0;
        $gameId = (int)($data['game_id'] ?? 0);
        $playCode = strtoupper(trim($data['play_code'] ?? ''));
        $playName = trim($data['play_name'] ?? '');
        $resultRule = trim($data['result_rule'] ?? '');
        if ($gameId < 1 || $playCode === '' || $playName === '' || $resultRule === '') {
            $this->error('请填写完整玩法信息');
        }
        if (!Db::name('lottery_game')->where('id', $gameId)->find()) {
            $this->error('游戏不存在');
        }
        if (!$this->isSupportedPlayRule($resultRule)) {
            $this->error('玩法规则不支持');
        }

        $exists = Db::name('lottery_play')
            ->where('game_id', $gameId)
            ->where('play_code', $playCode)
            ->where('id', '<>', $id)
            ->find();
        if ($exists) {
            $this->error('玩法编码已存在');
        }

        $row = array(
            'game_id' => $gameId,
            'play_code' => $playCode,
            'play_name' => $playName,
            'result_rule' => $resultRule,
            'sort' => (int)($data['sort'] ?? 0),
            'status' => (int)($data['status'] ?? 1) === 1 ? 1 : 0,
            'update_time' => time(),
        );
        if ($id > 0) {
            Db::name('lottery_play')->where('id', $id)->update($row);
        } else {
            $row['create_time'] = time();
            Db::name('lottery_play')->insert($row);
        }

        $this->success('保存成功', url('Lotterygame/plays', array('game_id' => $gameId)));
    }

    public function setPlayStatus() {
        $id = $this->request->param('id', 0, 'intval');
        $status = $this->request->param('status', 1, 'intval') === 1 ? 1 : 0;
        $play = Db::name('lottery_play')->where('id', $id)->find();
        if (!$play) {
            $this->error('玩法不存在');
        }
        Db::name('lottery_play')->where('id', $id)->update(array('status' => $status, 'update_time' => time()));
        $this->success('操作成功');
    }

    public function options() {
        $playId = $this->request->param('play_id', 0, 'intval');
        $play = Db::name('lottery_play')->where('id', $playId)->find();
        if (!$play) {
            $this->error('玩法不存在');
        }
        $game = Db::name('lottery_game')->where('id', $play['game_id'])->find();
        $list = Db::name('lottery_option')
            ->where('play_id', $playId)
            ->order('sort desc,id asc')
            ->paginate(20);

        $list->appends($this->request->param());
        $this->assign('game', $game);
        $this->assign('play', $play);
        $this->assign('list', $list);
        $this->assign('page', $list->render());
        $this->navTabs('index');
        return $this->fetch();
    }

    public function addOption() {
        $playId = $this->request->param('play_id', 0, 'intval');
        $play = Db::name('lottery_play')->where('id', $playId)->find();
        if (!$play) {
            $this->error('玩法不存在');
        }
        $game = Db::name('lottery_game')->where('id', $play['game_id'])->find();
        $this->assign('game', $game);
        $this->assign('play', $play);
        $this->assign('data', array('play_id' => $playId, 'option_code' => '', 'option_name' => '', 'odds' => '1.9500', 'sort' => 0, 'status' => 1));
        $this->assign('is_edit', 0);
        $this->assign('post_url', url('Lotterygame/addOptionPost'));
        $this->navTabs('');
        return $this->fetch('option_form');
    }

    public function addOptionPost() {
        $this->saveOption();
    }

    public function editOption() {
        $id = $this->request->param('id', 0, 'intval');
        $data = Db::name('lottery_option')->where('id', $id)->find();
        if (!$data) {
            $this->error('投注项不存在');
        }
        $play = Db::name('lottery_play')->where('id', $data['play_id'])->find();
        $game = Db::name('lottery_game')->where('id', $play['game_id'])->find();
        $this->assign('game', $game);
        $this->assign('play', $play);
        $this->assign('data', $data);
        $this->assign('is_edit', 1);
        $this->assign('post_url', url('Lotterygame/editOptionPost'));
        $this->navTabs('');
        return $this->fetch('option_form');
    }

    public function editOptionPost() {
        $this->saveOption();
    }

    protected function saveOption() {
        $data = $this->request->param();
        $id = isset($data['id']) ? (int)$data['id'] : 0;
        $playId = (int)($data['play_id'] ?? 0);
        $optionCode = strtoupper(trim($data['option_code'] ?? ''));
        $optionName = trim($data['option_name'] ?? '');
        if ($playId < 1 || $optionCode === '' || $optionName === '') {
            $this->error('请填写完整投注项信息');
        }
        $play = Db::name('lottery_play')->where('id', $playId)->find();
        if (!$play) {
            $this->error('玩法不存在');
        }
        $exists = Db::name('lottery_option')
            ->where('play_id', $playId)
            ->where('option_code', $optionCode)
            ->where('id', '<>', $id)
            ->find();
        if ($exists) {
            $this->error('投注项编码已存在');
        }

        $odds = number_format(max(0.0001, (float)($data['odds'] ?? 1)), 4, '.', '');
        $row = array(
            'play_id' => $playId,
            'option_code' => $optionCode,
            'option_name' => $optionName,
            'odds' => $odds,
            'sort' => (int)($data['sort'] ?? 0),
            'status' => (int)($data['status'] ?? 1) === 1 ? 1 : 0,
            'update_time' => time(),
        );
        if ($id > 0) {
            Db::name('lottery_option')->where('id', $id)->update($row);
        } else {
            $row['create_time'] = time();
            Db::name('lottery_option')->insert($row);
        }

        $this->success('保存成功', url('Lotterygame/options', array('play_id' => $playId)));
    }

    public function setOptionStatus() {
        $id = $this->request->param('id', 0, 'intval');
        $status = $this->request->param('status', 1, 'intval') === 1 ? 1 : 0;
        $option = Db::name('lottery_option')->where('id', $id)->find();
        if (!$option) {
            $this->error('投注项不存在');
        }
        Db::name('lottery_option')->where('id', $id)->update(array('status' => $status, 'update_time' => time()));
        $this->success('操作成功');
    }

    public function issues() {
        $data = $this->request->param();
        $map = array();

        $gameId = $data['game_id'] ?? '';
        if ($gameId !== '') {
            $map[] = array('game_id', '=', (int)$gameId);
        }

        $status = $data['status'] ?? '';
        if ($status !== '') {
            $map[] = array('status', '=', (int)$status);
        }

        $issueNum = trim($data['issue_num'] ?? '');
        if ($issueNum !== '') {
            $map[] = array('issue_num', 'like', '%' . $issueNum . '%');
        }

        $list = Db::name('lottery_issue')
            ->where($map)
            ->order('open_time desc,id desc')
            ->paginate(20);

        $list->each(function ($item) {
            $item['game'] = Db::name('lottery_game')->where('id', $item['game_id'])->find();
            return $item;
        });

        $list->appends($data);
        $this->assign('list', $list);
        $this->assign('page', $list->render());
        $this->assign('games', Db::name('lottery_game')->order('sort desc,id desc')->select());
        $this->assign('issue_status', $this->issueStatus());
        $this->navTabs('issues');
        return $this->fetch();
    }

    public function presetIssue() {
        $id = $this->request->param('id', 0, 'intval');
        $issue = Db::name('lottery_issue')->where('id', $id)->find();
        if (!$issue) {
            $this->error('期号不存在');
        }
        if ((int)$issue['status'] >= 2) {
            $this->error('已开奖或已结算期号不能预设');
        }
        $game = Db::name('lottery_game')->where('id', $issue['game_id'])->find();
        if (!$game) {
            $this->error('游戏不存在');
        }
        $config = $this->getDrawConfig($game);
        $preset = array();
        try {
            $preset = Db::name('lottery_preset_draw')
                ->where('game_id', (int)$game['id'])
                ->where('issue_num', (string)$issue['issue_num'])
                ->where('status', 1)
                ->order('id desc')
                ->find();
        } catch (\Throwable $e) {
            $preset = array();
        }

        $this->assign('issue', $issue);
        $this->assign('game', $game);
        $this->assign('config', $config);
        $this->assign('preset', $preset ?: array('open_code' => '', 'remark' => ''));
        $this->assign('post_url', url('Lotterygame/presetIssuePost'));
        $this->navTabs('issues');
        return $this->fetch('preset_issue');
    }

    public function presetIssuePost() {
        $data = $this->request->param();
        $issueId = (int)($data['issue_id'] ?? 0);
        $issue = Db::name('lottery_issue')->where('id', $issueId)->find();
        if (!$issue) {
            $this->error('期号不存在');
        }
        if ((int)$issue['status'] >= 2) {
            $this->error('已开奖或已结算期号不能预设');
        }
        $game = Db::name('lottery_game')->where('id', $issue['game_id'])->find();
        if (!$game) {
            $this->error('游戏不存在');
        }
        $config = $this->getDrawConfig($game);
        $openCode = trim($data['open_code'] ?? '');
        if (!LotteryLocalRule::validateOpenCode($openCode, $config)) {
            $this->error('开奖号码不符合本彩种配置');
        }
        $openCode = LotteryLocalRule::normalizeOpenCode($openCode, $config);
        $adminId = function_exists('cmf_get_current_admin_id') ? (int)cmf_get_current_admin_id() : 0;
        $now = time();
        $row = array(
            'game_id' => (int)$game['id'],
            'issue_id' => (int)$issue['id'],
            'issue_num' => (string)$issue['issue_num'],
            'open_code' => $openCode,
            'status' => 1,
            'remark' => trim($data['remark'] ?? ''),
            'admin_id' => $adminId,
            'update_time' => $now,
        );

        try {
            $exists = Db::name('lottery_preset_draw')
                ->where('game_id', (int)$game['id'])
                ->where('issue_num', (string)$issue['issue_num'])
                ->find();
            if ($exists) {
                Db::name('lottery_preset_draw')->where('id', $exists['id'])->update($row);
            } else {
                $row['create_time'] = $now;
                Db::name('lottery_preset_draw')->insert($row);
            }
        } catch (\Throwable $e) {
            $this->error('保存失败，请先执行本地开奖迁移 SQL');
        }

        $this->success('预设开奖已保存', url('Lotterygame/issues', array('game_id' => $game['id'])));
    }

    public function orders() {
        $data = $this->request->param();
        $map = array();

        $uid = $data['uid'] ?? '';
        if ($uid !== '') {
            $map[] = array('uid', '=', (int)$uid);
        }

        $gameId = $data['game_id'] ?? '';
        if ($gameId !== '') {
            $map[] = array('game_id', '=', (int)$gameId);
        }

        $status = $data['status'] ?? '';
        if ($status !== '') {
            $map[] = array('status', '=', (int)$status);
        }

        $issueNum = trim($data['issue_num'] ?? '');
        if ($issueNum !== '') {
            $map[] = array('issue_num', 'like', '%' . $issueNum . '%');
        }

        $list = Db::name('lottery_bet_order')
            ->where($map)
            ->order('id desc')
            ->paginate(20);

        $list->each(function ($item) {
            $item['game'] = Db::name('lottery_game')->where('id', $item['game_id'])->find();
            $item['userinfo'] = function_exists('getUserInfo') ? getUserInfo($item['uid']) : array();
            return $item;
        });

        $list->appends($data);
        $this->assign('list', $list);
        $this->assign('page', $list->render());
        $this->assign('games', Db::name('lottery_game')->order('sort desc,id desc')->select());
        $this->assign('order_status', $this->orderStatus());
        $this->navTabs('orders');
        return $this->fetch();
    }

}
