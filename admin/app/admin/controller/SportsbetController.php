<?php
namespace app\admin\controller;

use cmf\controller\AdminBaseController;
use think\facade\Db;

class SportsbetController extends AdminBaseController {
    const ORDER_PENDING = 0;
    const ORDER_WIN = 1;
    const ORDER_LOSE = 2;
    const ORDER_REFUND = 3;
    const ORDER_CANCELED = 4;

    const BET_DISABLED = 0;
    const BET_OPEN = 1;
    const BET_CLOSED = 2;

    const SETTLE_PENDING = 0;
    const SETTLE_DONE = 1;
    const SETTLE_REFUNDED = 2;
    const SETTLE_BLOCKED = 3;

    const COIN_ACTION_REFUND = 30;
    const COIN_ACTION_PAYOUT = 31;
    const MARKET_TEMPLATE_OPTION = 'sports_market_template';

    protected function navTabs($active = 'index') {
        $tabs = array(
            'index' => array('title' => '比赛管理', 'url' => url('Sportsbet/index')),
            'orders' => array('title' => '体育注单', 'url' => url('Sportsbet/orders')),
            'settle' => array('title' => '结算日志', 'url' => url('Sportsbet/settleLogs')),
            'score' => array('title' => '比分流水', 'url' => url('Sportsbet/scoreLogs')),
        );
        $this->assign('nav_tabs', $tabs);
        $this->assign('active_tab', $active);
    }

    protected function betStatus($key = '') {
        $items = array(
            '0' => '未开放',
            '1' => '可投注',
            '2' => '已停止',
        );
        return $key === '' ? $items : ($items[(string)$key] ?? '');
    }

    protected function settleStatus($key = '') {
        $items = array(
            '0' => '待结算',
            '1' => '已结算',
            '2' => '已退款',
            '3' => '异常待处理',
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

    public function index() {
        $data = $this->request->param();
        $map = array();

        $keyword = trim($data['keyword'] ?? '');
        if ($keyword !== '') {
            $displayMatch = $this->splitDisplayMatchId($keyword);
            if ($displayMatch['source'] !== '' && $displayMatch['source_match_id'] !== '') {
                $map[] = array('source', '=', $displayMatch['source']);
                $map[] = array('source_match_id', '=', $displayMatch['source_match_id']);
            } else {
                $map[] = array('competition|home_name|away_name|source_match_id|source', 'like', '%' . $keyword . '%');
            }
        }

        $date = trim($data['date'] ?? '');
        if ($date !== '') {
            $start = strtotime($date);
            if ($start) {
                $map[] = array('kickoff_time', '>=', $start);
                $map[] = array('kickoff_time', '<', $start + 86400);
            }
        }

        $betStatus = $data['bet_status'] ?? '';
        if ($betStatus !== '') {
            $map[] = array('bet_status', '=', (int)$betStatus);
        }

        $settleStatus = $data['settle_status'] ?? '';
        if ($settleStatus !== '') {
            $map[] = array('settle_status', '=', (int)$settleStatus);
        }

        $list = Db::name('sports_match')
            ->where($map)
            ->order('kickoff_time desc,id desc')
            ->paginate(20);

        $list->each(function ($item) {
            $item['display_match_id'] = $this->displayMatchId($item);
            $item['markets_count'] = Db::name('sports_market')->where('match_id', $item['id'])->where('status', 1)->count();
            $item['options_count'] = Db::name('sports_option')
                ->alias('o')
                ->join('sports_market m', 'm.id=o.market_id')
                ->where('m.match_id', $item['id'])
                ->where('o.status', 1)
                ->count();
            $item['pending_orders'] = Db::name('sports_bet_order')->where('match_id', $item['id'])->where('status', self::ORDER_PENDING)->count();
            $item['score_rejects'] = Db::name('sports_score_log')->where('match_id', $item['id'])->where('accepted', 0)->count();
            return $item;
        });

        $list->appends($data);
        $this->assign('list', $list);
        $this->assign('page', $list->render());
        $this->assign('bet_status', $this->betStatus());
        $this->assign('settle_status', $this->settleStatus());
        $marketTemplate = $this->getSportsMarketTemplate();
        $this->assign('has_market_template', !empty($marketTemplate['markets']));
        $this->navTabs('index');
        return $this->fetch();
    }

    public function addMatch() {
        $this->assign('data', array(
            'source' => 'manual',
            'source_match_id' => '',
            'competition' => '',
            'competition_type' => '',
            'country' => '',
            'home_name' => '',
            'away_name' => '',
            'match_date' => '',
            'kickoff_time_text' => '',
            'home_score' => -1,
            'away_score' => -1,
            'status' => 'NS',
            'status_text' => '未开赛',
            'bet_status' => 0,
            'settle_status' => 0,
            'seal_advance_sec' => 300,
            'min_bet' => 10,
            'max_bet' => 500000,
            'max_match_bet' => 1000000,
        ));
        $this->assign('is_edit', 0);
        $this->assign('post_url', url('Sportsbet/addMatchPost'));
        $this->assign('bet_status', $this->betStatus());
        $this->navTabs('');
        return $this->fetch('match_form');
    }

    public function addMatchPost() {
        $this->saveMatch();
    }

    public function editMatch() {
        $id = $this->request->param('id', 0, 'intval');
        $data = Db::name('sports_match')->where('id', $id)->find();
        if (!$data) {
            $this->error('比赛不存在');
        }
        $data['kickoff_time_text'] = (int)$data['kickoff_time'] > 0 ? date('Y-m-d H:i:s', (int)$data['kickoff_time']) : '';
        $this->assign('data', $data);
        $this->assign('is_edit', 1);
        $this->assign('post_url', url('Sportsbet/editMatchPost'));
        $this->assign('bet_status', $this->betStatus());
        $this->navTabs('');
        return $this->fetch('match_form');
    }

    public function editMatchPost() {
        $this->saveMatch();
    }

    protected function saveMatch() {
        $data = $this->request->param();
        $id = isset($data['id']) ? (int)$data['id'] : 0;
        $source = trim($data['source'] ?? 'manual');
        $sourceMatchId = trim($data['source_match_id'] ?? '');
        $homeName = trim($data['home_name'] ?? '');
        $awayName = trim($data['away_name'] ?? '');
        $kickoffTimeText = trim($data['kickoff_time_text'] ?? '');
        $kickoffTime = $kickoffTimeText !== '' ? strtotime($kickoffTimeText) : 0;

        if ($source === '' || $sourceMatchId === '' || $homeName === '' || $awayName === '' || $kickoffTime < 1) {
            $this->error('请填写数据源、比赛ID、主客队和开赛时间');
        }

        $exists = Db::name('sports_match')
            ->where('source', $source)
            ->where('source_match_id', $sourceMatchId)
            ->where('id', '<>', $id)
            ->find();
        if ($exists) {
            $this->error('同数据源比赛ID已存在');
        }

        $sealAdvance = max(300, (int)($data['seal_advance_sec'] ?? 300));
        $homeScore = trim((string)($data['home_score'] ?? '-1'));
        $awayScore = trim((string)($data['away_score'] ?? '-1'));
        $homeScore = is_numeric($homeScore) ? (int)$homeScore : -1;
        $awayScore = is_numeric($awayScore) ? (int)$awayScore : -1;
        $now = time();
        $row = array(
            'source' => $source,
            'source_match_id' => $sourceMatchId,
            'competition' => trim($data['competition'] ?? ''),
            'competition_type' => trim($data['competition_type'] ?? ''),
            'country' => trim($data['country'] ?? ''),
            'home_name' => $homeName,
            'away_name' => $awayName,
            'match_date' => date('Y-m-d', $kickoffTime),
            'kickoff_time' => $kickoffTime,
            'bet_close_time' => max(0, $kickoffTime - $sealAdvance),
            'seal_advance_sec' => $sealAdvance,
            'home_score' => $homeScore,
            'away_score' => $awayScore,
            'status' => trim($data['status'] ?? 'NS'),
            'status_text' => trim($data['status_text'] ?? ''),
            'raw_status' => trim($data['raw_status'] ?? ''),
            'bet_status' => in_array((int)($data['bet_status'] ?? 0), array(0, 1, 2), true) ? (int)$data['bet_status'] : 0,
            'min_bet' => max(1, (int)($data['min_bet'] ?? 10)),
            'max_bet' => max(1, (int)($data['max_bet'] ?? 500000)),
            'max_match_bet' => max(1, (int)($data['max_match_bet'] ?? 1000000)),
            'update_time' => $now,
        );

        if ($id > 0) {
            Db::name('sports_match')->where('id', $id)->update($row);
        } else {
            $row['settle_status'] = self::SETTLE_PENDING;
            $row['sync_time'] = $now;
            $row['create_time'] = $now;
            $id = Db::name('sports_match')->insertGetId($row);
        }

        $this->success('保存成功', url('Sportsbet/markets', array('match_id' => $id)));
    }

    public function markets() {
        $matchId = $this->request->param('match_id', 0, 'intval');
        $match = Db::name('sports_match')->where('id', $matchId)->find();
        if (!$match) {
            $this->error('比赛不存在');
        }
        $markets = $this->ensureMarkets($matchId);
        $marketTemplate = $this->getSportsMarketTemplate();
        $this->assign('match', $match);
        $this->assign('markets', $markets);
        $this->assign('has_market_template', !empty($marketTemplate['markets']));
        $this->assign('market_template_time', !empty($marketTemplate['update_time']) ? date('Y-m-d H:i', (int)$marketTemplate['update_time']) : '');
        $this->navTabs('index');
        return $this->fetch();
    }

    public function saveMarketsPost() {
        $data = $this->request->param();
        $matchId = (int)($data['match_id'] ?? 0);
        $match = Db::name('sports_match')->where('id', $matchId)->find();
        if (!$match) {
            $this->error('比赛不存在');
        }

        $marketStatus = isset($data['market_status']) && is_array($data['market_status']) ? $data['market_status'] : array();
        $optionStatus = isset($data['option_status']) && is_array($data['option_status']) ? $data['option_status'] : array();
        $optionOdds = isset($data['odds']) && is_array($data['odds']) ? $data['odds'] : array();
        $now = time();

        foreach ($marketStatus as $marketId => $status) {
            $market = Db::name('sports_market')->where('id', (int)$marketId)->where('match_id', $matchId)->find();
            if (!$market) {
                continue;
            }
            Db::name('sports_market')->where('id', $market['id'])->update(array(
                'status' => (int)$status === 1 ? 1 : 0,
                'update_time' => $now,
            ));
        }

        foreach ($optionOdds as $optionId => $odds) {
            $option = Db::name('sports_option')->alias('o')
                ->join('sports_market m', 'm.id=o.market_id')
                ->field('o.*,m.market_code,m.match_id')
                ->where('o.id', (int)$optionId)
                ->where('m.match_id', $matchId)
                ->find();
            if (!$option) {
                continue;
            }
            $ruleError = $this->validateOptionRule($option['market_code'], $option['option_code']);
            if ($ruleError !== '') {
                $this->error($ruleError);
            }
            $cleanOdds = number_format(max(0.0001, (float)$odds), 4, '.', '');
            Db::name('sports_option')->where('id', $option['id'])->update(array(
                'odds' => $cleanOdds,
                'status' => isset($optionStatus[$optionId]) && (int)$optionStatus[$optionId] === 1 ? 1 : 0,
                'update_time' => $now,
            ));
        }

        $this->success('赔率保存成功', url('Sportsbet/markets', array('match_id' => $matchId)));
    }

    public function saveMarketsTemplate() {
        $data = $this->request->param();
        $matchId = (int)($data['match_id'] ?? 0);
        $match = Db::name('sports_match')->where('id', $matchId)->find();
        if (!$match) {
            return json(array('code' => 0, 'msg' => '比赛不存在'));
        }

        $template = $this->buildSportsMarketTemplate($matchId, $data);
        $this->saveSportsMarketTemplate($template);

        return json(array(
            'code' => 1,
            'msg' => '模板保存成功',
            'data' => array('update_time' => date('Y-m-d H:i', (int)$template['update_time'])),
        ));
    }

    public function applyMarketsTemplate() {
        $matchId = $this->request->param('match_id', 0, 'intval');
        $match = Db::name('sports_match')->where('id', $matchId)->find();
        if (!$match) {
            return json(array('code' => 0, 'msg' => '比赛不存在'));
        }

        $template = $this->getSportsMarketTemplate();
        if (empty($template['markets']) || !is_array($template['markets'])) {
            return json(array('code' => 0, 'msg' => '还没有保存赔率模板'));
        }

        $result = $this->applySportsMarketTemplate($matchId, $template);
        if (!empty($result['error'])) {
            return json(array('code' => 0, 'msg' => $result['error']));
        }

        return json(array(
            'code' => 1,
            'msg' => '模板应用成功，更新玩法 ' . $result['markets'] . ' 个，投注项 ' . $result['options'] . ' 个',
        ));
    }

    public function applyMarketsTemplateBatch() {
        $data = $this->request->param();
        $hasFilter = trim($data['keyword'] ?? '') !== ''
            || trim($data['date'] ?? '') !== ''
            || ($data['bet_status'] ?? '') !== ''
            || ($data['settle_status'] ?? '') !== '';
        if (!$hasFilter) {
            return json(array('code' => 0, 'msg' => '请先筛选日期、状态或关键词后再批量套模板'));
        }

        $template = $this->getSportsMarketTemplate();
        if (empty($template['markets']) || !is_array($template['markets'])) {
            return json(array('code' => 0, 'msg' => '还没有保存赔率模板'));
        }

        $map = $this->buildSportsMatchMap($data);
        $matches = Db::name('sports_match')
            ->where($map)
            ->where('settle_status', self::SETTLE_PENDING)
            ->field('id')
            ->select()
            ->toArray();
        if (empty($matches)) {
            return json(array('code' => 0, 'msg' => '当前筛选没有可应用的未结算比赛'));
        }

        $matchCount = 0;
        $marketCount = 0;
        $optionCount = 0;
        foreach ($matches as $match) {
            $result = $this->applySportsMarketTemplate((int)$match['id'], $template);
            if (!empty($result['error'])) {
                return json(array('code' => 0, 'msg' => $result['error']));
            }
            $matchCount++;
            $marketCount += (int)$result['markets'];
            $optionCount += (int)$result['options'];
        }

        return json(array(
            'code' => 1,
            'msg' => '批量应用成功，比赛 ' . $matchCount . ' 场，玩法 ' . $marketCount . ' 个，投注项 ' . $optionCount . ' 个',
        ));
    }

    public function setBetStatus() {
        $id = $this->request->param('id', 0, 'intval');
        $status = $this->request->param('status', 0, 'intval');
        if ($id < 1 || !in_array($status, array(0, 1, 2), true)) {
            $this->error('参数错误');
        }
        Db::name('sports_match')->where('id', $id)->update(array('bet_status' => $status, 'update_time' => time()));
        $this->success('操作成功');
    }

    public function setMarketStatus() {
        $matchId = $this->request->param('match_id', 0, 'intval');
        $id = $this->request->param('id', 0, 'intval');
        $status = $this->request->param('status', 0, 'intval') === 1 ? 1 : 0;
        if ($matchId < 1 || $id < 1) {
            return json(array('code' => 0, 'msg' => '参数错误'));
        }

        $market = Db::name('sports_market')
            ->where('id', $id)
            ->where('match_id', $matchId)
            ->find();
        if (!$market) {
            return json(array('code' => 0, 'msg' => '玩法不存在'));
        }

        Db::name('sports_market')->where('id', $id)->update(array(
            'status' => $status,
            'update_time' => time(),
        ));

        return json(array('code' => 1, 'msg' => '操作成功', 'data' => array('status' => $status)));
    }

    public function setOptionStatus() {
        $matchId = $this->request->param('match_id', 0, 'intval');
        $id = $this->request->param('id', 0, 'intval');
        $status = $this->request->param('status', 0, 'intval') === 1 ? 1 : 0;
        if ($matchId < 1 || $id < 1) {
            return json(array('code' => 0, 'msg' => '参数错误'));
        }

        $option = Db::name('sports_option')->alias('o')
            ->join('sports_market m', 'm.id=o.market_id')
            ->field('o.id')
            ->where('o.id', $id)
            ->where('m.match_id', $matchId)
            ->find();
        if (!$option) {
            return json(array('code' => 0, 'msg' => '投注项不存在'));
        }

        Db::name('sports_option')->where('id', $id)->update(array(
            'status' => $status,
            'update_time' => time(),
        ));

        return json(array('code' => 1, 'msg' => '操作成功', 'data' => array('status' => $status)));
    }

    public function settleMatch() {
        $id = $this->request->param('id', 0, 'intval');
        $result = $this->doSettleMatch($id);
        if (!empty($result['error'])) {
            $this->error($result['error']);
        }
        $this->success('结算成功，处理注单 ' . $result['orders_total'] . ' 笔');
    }

    public function refundMatch() {
        $id = $this->request->param('id', 0, 'intval');
        $reason = trim($this->request->param('reason', '比赛取消或异常退款'));
        $result = $this->doRefundMatch($id, $reason);
        if (!empty($result['error'])) {
            $this->error($result['error']);
        }
        $this->success('退款成功，处理注单 ' . $result['orders_total'] . ' 笔');
    }

    public function orders() {
        $data = $this->request->param();
        $map = array();

        $uid = $data['uid'] ?? '';
        if ($uid !== '') {
            $map[] = array('uid', '=', (int)$uid);
        }

        $matchId = $data['match_id'] ?? '';
        if ($matchId !== '') {
            $resolvedMatchId = $this->resolveMatchIdForSearch($matchId);
            $map[] = array('match_id', '=', $resolvedMatchId);
        }

        $status = $data['status'] ?? '';
        if ($status !== '') {
            $map[] = array('status', '=', (int)$status);
        }

        $keyword = trim($data['keyword'] ?? '');
        if ($keyword !== '') {
            $displayMatch = $this->splitDisplayMatchId($keyword);
            if ($displayMatch['source_match_id'] !== '') {
                $keyword = $displayMatch['source_match_id'];
            }
            $map[] = array('order_no|source_match_id|match_title', 'like', '%' . $keyword . '%');
        }

        $list = Db::name('sports_bet_order')
            ->where($map)
            ->order('id desc')
            ->paginate(20);

        $list->each(function ($item) {
            $item['match'] = Db::name('sports_match')->where('id', $item['match_id'])->find();
            $item['display_match_id'] = !empty($item['match']) ? $this->displayMatchId($item['match']) : $item['source_match_id'];
            $item['userinfo'] = function_exists('getUserInfo') ? getUserInfo($item['uid']) : array();
            $item['items'] = Db::name('sports_bet_item')->where('order_id', $item['id'])->order('id asc')->select()->toArray();
            return $item;
        });

        $list->appends($data);
        $this->assign('list', $list);
        $this->assign('page', $list->render());
        $this->assign('order_status', $this->orderStatus());
        $this->navTabs('orders');
        return $this->fetch();
    }

    public function settleLogs() {
        $data = $this->request->param();
        $map = array();
        $matchId = $data['match_id'] ?? '';
        if ($matchId !== '') {
            $resolvedMatchId = $this->resolveMatchIdForSearch($matchId);
            $map[] = array('match_id', '=', $resolvedMatchId);
        }
        $list = Db::name('sports_settle_log')
            ->where($map)
            ->order('id desc')
            ->paginate(20);
        $list->each(function ($item) {
            $item['match'] = Db::name('sports_match')->where('id', $item['match_id'])->find();
            $item['display_match_id'] = !empty($item['match']) ? $this->displayMatchId($item['match']) : (string)$item['match_id'];
            return $item;
        });
        $list->appends($data);
        $this->assign('list', $list);
        $this->assign('page', $list->render());
        $this->navTabs('settle');
        return $this->fetch('settle_logs');
    }

    public function scoreLogs() {
        $data = $this->request->param();
        $map = array();

        $matchId = $data['match_id'] ?? '';
        if ($matchId !== '') {
            $resolvedMatchId = $this->resolveMatchIdForSearch($matchId);
            $map[] = array('match_id', '=', $resolvedMatchId);
        }

        $sourceMatchId = trim($data['source_match_id'] ?? '');
        if ($sourceMatchId !== '') {
            $map[] = array('source_match_id', '=', $sourceMatchId);
        }

        $accepted = $data['accepted'] ?? '';
        if ($accepted !== '') {
            $map[] = array('accepted', '=', (int)$accepted);
        }

        $reason = trim($data['reason'] ?? '');
        if ($reason !== '') {
            $map[] = array('reason', '=', $reason);
        }

        $reasons = array(
            'match_created' => '比赛创建',
            'score_snapshot_changed' => '比分/状态变化',
            'live_score_regression' => '非完场比分回退拒收',
            'final_score_regression_blocked' => '完场比分回退待核验',
            'settled_snapshot_ignored' => '已结算快照忽略',
        );

        $list = Db::name('sports_score_log')
            ->where($map)
            ->order('id desc')
            ->paginate(30);
        $list->each(function ($item) use ($reasons) {
            $item['match'] = Db::name('sports_match')->where('id', $item['match_id'])->find();
            $item['display_match_id'] = !empty($item['match']) ? $this->displayMatchId($item['match']) : $item['source_match_id'];
            $item['reason_text'] = isset($reasons[$item['reason']]) ? $reasons[$item['reason']] : $item['reason'];
            return $item;
        });

        $list->appends($data);
        $this->assign('list', $list);
        $this->assign('page', $list->render());
        $this->assign('reasons', $reasons);
        $this->navTabs('score');
        return $this->fetch('score_logs');
    }

    protected function displayMatchId($match) {
        $sourceMatchId = isset($match['source_match_id']) ? trim((string)$match['source_match_id']) : '';
        return $sourceMatchId !== '' ? $sourceMatchId : (isset($match['id']) ? (string)$match['id'] : '');
    }

    protected function splitDisplayMatchId($value) {
        $value = trim((string)$value);
        if ($value === '' || strpos($value, ':') === false) {
            return array('source' => '', 'source_match_id' => $value);
        }
        $parts = explode(':', $value, 2);
        return array(
            'source' => trim($parts[0]),
            'source_match_id' => trim($parts[1]),
        );
    }

    protected function buildSportsMatchMap($data) {
        $map = array();

        $keyword = trim($data['keyword'] ?? '');
        if ($keyword !== '') {
            $displayMatch = $this->splitDisplayMatchId($keyword);
            if ($displayMatch['source'] !== '' && $displayMatch['source_match_id'] !== '') {
                $map[] = array('source', '=', $displayMatch['source']);
                $map[] = array('source_match_id', '=', $displayMatch['source_match_id']);
            } else {
                $map[] = array('competition|home_name|away_name|source_match_id|source', 'like', '%' . $keyword . '%');
            }
        }

        $date = trim($data['date'] ?? '');
        if ($date !== '') {
            $start = strtotime($date);
            if ($start) {
                $map[] = array('kickoff_time', '>=', $start);
                $map[] = array('kickoff_time', '<', $start + 86400);
            }
        }

        $betStatus = $data['bet_status'] ?? '';
        if ($betStatus !== '') {
            $map[] = array('bet_status', '=', (int)$betStatus);
        }

        $settleStatus = $data['settle_status'] ?? '';
        if ($settleStatus !== '') {
            $map[] = array('settle_status', '=', (int)$settleStatus);
        }

        return $map;
    }

    protected function resolveMatchIdForSearch($value) {
        $value = trim((string)$value);
        if ($value === '') {
            return -1;
        }

        $display = $this->splitDisplayMatchId($value);
        if ($display['source'] !== '' && $display['source_match_id'] !== '') {
            $match = Db::name('sports_match')
                ->where('source', $display['source'])
                ->where('source_match_id', $display['source_match_id'])
                ->find();
            return $match ? (int)$match['id'] : -1;
        }

        if (ctype_digit($value)) {
            $local = Db::name('sports_match')->where('id', (int)$value)->find();
            if ($local) {
                return (int)$local['id'];
            }
        }

        $match = Db::name('sports_match')->where('source_match_id', $value)->find();
        return $match ? (int)$match['id'] : -1;
    }

    protected function getSportsMarketTemplate() {
        $template = array();
        if (function_exists('cmf_get_option')) {
            $template = cmf_get_option(self::MARKET_TEMPLATE_OPTION);
        } else {
            $optionValue = Db::name('option')->where('option_name', self::MARKET_TEMPLATE_OPTION)->value('option_value');
            $template = $optionValue ? json_decode($optionValue, true) : array();
        }

        if (is_string($template) && $template !== '') {
            $decoded = json_decode($template, true);
            $template = is_array($decoded) ? $decoded : array();
        }

        return is_array($template) ? $template : array();
    }

    protected function saveSportsMarketTemplate($template) {
        if (function_exists('cmf_set_option')) {
            cmf_set_option(self::MARKET_TEMPLATE_OPTION, $template, true);
            return;
        }

        $optionValue = json_encode($template, JSON_UNESCAPED_UNICODE);
        $exists = Db::name('option')->where('option_name', self::MARKET_TEMPLATE_OPTION)->find();
        if ($exists) {
            Db::name('option')->where('option_name', self::MARKET_TEMPLATE_OPTION)->update(array(
                'autoload' => 1,
                'option_value' => $optionValue,
            ));
            return;
        }

        Db::name('option')->insert(array(
            'autoload' => 1,
            'option_name' => self::MARKET_TEMPLATE_OPTION,
            'option_value' => $optionValue,
        ));
    }

    protected function buildSportsMarketTemplate($matchId, $data = array()) {
        $match = Db::name('sports_match')->where('id', $matchId)->find();
        $markets = $this->ensureMarkets($matchId);
        $marketStatus = isset($data['market_status']) && is_array($data['market_status']) ? $data['market_status'] : array();
        $optionStatus = isset($data['option_status']) && is_array($data['option_status']) ? $data['option_status'] : array();
        $optionOdds = isset($data['odds']) && is_array($data['odds']) ? $data['odds'] : array();

        $template = array(
            'version' => 1,
            'source_match_id' => $match ? (string)$match['source_match_id'] : '',
            'source_match_title' => $match ? trim($match['home_name'] . ' vs ' . $match['away_name']) : '',
            'update_time' => time(),
            'markets' => array(),
        );

        foreach ($markets as $market) {
            $marketId = (int)$market['id'];
            $marketCode = (string)$market['market_code'];
            $status = array_key_exists($marketId, $marketStatus)
                ? ((int)$marketStatus[$marketId] === 1 ? 1 : 0)
                : ((int)$market['status'] === 1 ? 1 : 0);

            $marketRow = array(
                'market_name' => (string)$market['market_name'],
                'status' => $status,
                'options' => array(),
            );

            foreach ($market['options'] as $option) {
                $optionId = (int)$option['id'];
                $optionCode = (string)$option['option_code'];
                $optionStatusValue = array_key_exists($optionId, $optionStatus)
                    ? ((int)$optionStatus[$optionId] === 1 ? 1 : 0)
                    : ((int)$option['status'] === 1 ? 1 : 0);
                $odds = array_key_exists($optionId, $optionOdds) ? $optionOdds[$optionId] : $option['odds'];

                $marketRow['options'][$optionCode] = array(
                    'option_name' => (string)$option['option_name'],
                    'odds' => number_format(max(0.0001, (float)$odds), 4, '.', ''),
                    'status' => $optionStatusValue,
                );
            }

            $template['markets'][$marketCode] = $marketRow;
        }

        return $template;
    }

    protected function applySportsMarketTemplate($matchId, $template) {
        $markets = $this->ensureMarkets($matchId);
        $now = time();
        $marketCount = 0;
        $optionCount = 0;

        Db::startTrans();
        try {
            foreach ($markets as $market) {
                $marketCode = (string)$market['market_code'];
                if (empty($template['markets'][$marketCode]) || !is_array($template['markets'][$marketCode])) {
                    continue;
                }

                $tplMarket = $template['markets'][$marketCode];
                if (array_key_exists('status', $tplMarket)) {
                    Db::name('sports_market')->where('id', (int)$market['id'])->update(array(
                        'status' => (int)$tplMarket['status'] === 1 ? 1 : 0,
                        'update_time' => $now,
                    ));
                    $marketCount++;
                }

                $tplOptions = isset($tplMarket['options']) && is_array($tplMarket['options']) ? $tplMarket['options'] : array();
                foreach ($market['options'] as $option) {
                    $optionCode = (string)$option['option_code'];
                    if (empty($tplOptions[$optionCode]) || !is_array($tplOptions[$optionCode])) {
                        continue;
                    }

                    $tplOption = $tplOptions[$optionCode];
                    Db::name('sports_option')->where('id', (int)$option['id'])->update(array(
                        'odds' => isset($tplOption['odds']) ? number_format(max(0.0001, (float)$tplOption['odds']), 4, '.', '') : $option['odds'],
                        'status' => isset($tplOption['status']) && (int)$tplOption['status'] === 1 ? 1 : 0,
                        'update_time' => $now,
                    ));
                    $optionCount++;
                }
            }
            Db::commit();
        } catch (\Exception $e) {
            Db::rollback();
            return array('error' => '模板应用失败: ' . $e->getMessage());
        }

        return array('markets' => $marketCount, 'options' => $optionCount);
    }

    protected function ensureMarkets($matchId) {
        $now = time();
        foreach ($this->marketDefinitions() as $marketCode => $market) {
            $exists = Db::name('sports_market')->where('match_id', $matchId)->where('market_code', $marketCode)->find();
            if ($exists) {
                $marketId = (int)$exists['id'];
            } else {
                $marketId = Db::name('sports_market')->insertGetId(array(
                    'match_id' => $matchId,
                    'market_code' => $marketCode,
                    'market_name' => $market['name'],
                    'market_rule' => $market['rule'],
                    'sort' => $market['sort'],
                    'status' => 0,
                    'create_time' => $now,
                    'update_time' => $now,
                ));
            }

            foreach ($market['options'] as $option) {
                $optionExists = Db::name('sports_option')->where('market_id', $marketId)->where('option_code', $option[0])->find();
                if ($optionExists) {
                    continue;
                }
                Db::name('sports_option')->insert(array(
                    'market_id' => $marketId,
                    'option_code' => $option[0],
                    'option_name' => $option[1],
                    'odds' => '1.0000',
                    'sort' => $option[2],
                    'status' => 0,
                    'create_time' => $now,
                    'update_time' => $now,
                ));
            }
        }

        $markets = Db::name('sports_market')->where('match_id', $matchId)->order('sort desc,id asc')->select()->toArray();
        foreach ($markets as $key => $market) {
            $markets[$key]['options'] = Db::name('sports_option')->where('market_id', $market['id'])->order('sort desc,id asc')->select()->toArray();
        }
        return $markets;
    }

    protected function doSettleMatch($matchId) {
        $match = Db::name('sports_match')->where('id', $matchId)->find();
        if (!$match) {
            return array('error' => '比赛不存在');
        }
        if ((int)$match['settle_status'] === self::SETTLE_DONE) {
            return array('error' => '比赛已结算');
        }
        if ((int)$match['settle_status'] === self::SETTLE_BLOCKED) {
            return array('error' => '比分异常待人工核验，禁止直接结算');
        }
        if (!$this->isFinishedStatus($match['status']) || (int)$match['home_score'] < 0 || (int)$match['away_score'] < 0) {
            return array('error' => '比赛必须完场且比分完整才可结算');
        }

        $orders = Db::name('sports_bet_order')->where('match_id', $matchId)->where('status', self::ORDER_PENDING)->select()->toArray();
        $ordersTotal = 0;
        $ordersWin = 0;
        $ordersLose = 0;
        $payoutTotal = 0;
        $now = time();

        Db::startTrans();
        try {
            foreach ($orders as $order) {
                $ordersTotal++;
                $items = Db::name('sports_bet_item')->where('order_id', $order['id'])->select()->toArray();
                $orderPayout = 0;
                foreach ($items as $item) {
                    $win = $this->isItemWin($match, $item['market_code'], $item['option_code']);
                    $payout = $win ? (int)floor((int)$item['bet_amount'] * (float)$item['odds']) : 0;
                    $orderPayout += $payout;
                    Db::name('sports_bet_item')->where('id', $item['id'])->update(array(
                        'payout_amount' => $payout,
                        'win_status' => $win ? 1 : 2,
                        'update_time' => $now,
                    ));
                }

                if ($orderPayout > 0) {
                    $ordersWin++;
                    $payoutTotal += $orderPayout;
                    Db::name('user')->where('id', $order['uid'])->update(array('coin' => Db::raw('coin+' . $orderPayout)));
                    Db::name('user_coinrecord')->insert(array(
                        'type' => 1,
                        'action' => self::COIN_ACTION_PAYOUT,
                        'uid' => $order['uid'],
                        'touid' => $order['uid'],
                        'giftid' => $order['id'],
                        'giftcount' => 1,
                        'totalcoin' => $orderPayout,
                        'showid' => 0,
                        'addtime' => $now,
                    ));
                } else {
                    $ordersLose++;
                }

                Db::name('sports_bet_order')->where('id', $order['id'])->where('status', self::ORDER_PENDING)->update(array(
                    'total_payout' => $orderPayout,
                    'net_amount' => $orderPayout - (int)$order['total_bet'],
                    'status' => $orderPayout > 0 ? self::ORDER_WIN : self::ORDER_LOSE,
                    'settle_time' => $now,
                    'update_time' => $now,
                ));
            }

            Db::name('sports_match')->where('id', $matchId)->update(array(
                'bet_status' => self::BET_CLOSED,
                'settle_status' => self::SETTLE_DONE,
                'settle_time' => $now,
                'settle_remark' => 'ok',
                'update_time' => $now,
            ));

            Db::name('sports_settle_log')->insert(array(
                'match_id' => $matchId,
                'settle_key' => 'admin_match_' . $matchId . '_' . (int)$match['home_score'] . '_' . (int)$match['away_score'],
                'home_score' => $match['home_score'],
                'away_score' => $match['away_score'],
                'orders_total' => $ordersTotal,
                'orders_win' => $ordersWin,
                'orders_lose' => $ordersLose,
                'orders_refund' => 0,
                'payout_total' => $payoutTotal,
                'success' => 1,
                'message' => 'ok',
                'create_time' => $now,
            ));

            Db::commit();
        } catch (\Exception $e) {
            Db::rollback();
            return array('error' => '结算失败: ' . $e->getMessage());
        }

        return array('orders_total' => $ordersTotal, 'orders_win' => $ordersWin, 'orders_lose' => $ordersLose);
    }

    protected function doRefundMatch($matchId, $reason) {
        $match = Db::name('sports_match')->where('id', $matchId)->find();
        if (!$match) {
            return array('error' => '比赛不存在');
        }
        if ((int)$match['settle_status'] === self::SETTLE_DONE) {
            return array('error' => '已结算比赛不能退款');
        }

        $orders = Db::name('sports_bet_order')->where('match_id', $matchId)->where('status', self::ORDER_PENDING)->select()->toArray();
        $now = time();
        $ordersTotal = 0;
        $refundTotal = 0;

        Db::startTrans();
        try {
            foreach ($orders as $order) {
                $ordersTotal++;
                $refundAmount = (int)$order['total_bet'];
                $refundTotal += $refundAmount;
                Db::name('user')->where('id', $order['uid'])->update(array('coin' => Db::raw('coin+' . $refundAmount)));
                Db::name('sports_bet_item')->where('order_id', $order['id'])->update(array(
                    'payout_amount' => Db::raw('bet_amount'),
                    'win_status' => 3,
                    'update_time' => $now,
                ));
                Db::name('sports_bet_order')->where('id', $order['id'])->where('status', self::ORDER_PENDING)->update(array(
                    'total_payout' => $refundAmount,
                    'net_amount' => 0,
                    'status' => self::ORDER_REFUND,
                    'settle_time' => $now,
                    'settle_remark' => $reason,
                    'update_time' => $now,
                ));
                Db::name('user_coinrecord')->insert(array(
                    'type' => 1,
                    'action' => self::COIN_ACTION_REFUND,
                    'uid' => $order['uid'],
                    'touid' => $order['uid'],
                    'giftid' => $order['id'],
                    'giftcount' => 1,
                    'totalcoin' => $refundAmount,
                    'showid' => 0,
                    'addtime' => $now,
                ));
            }

            Db::name('sports_match')->where('id', $matchId)->update(array(
                'bet_status' => self::BET_CLOSED,
                'settle_status' => self::SETTLE_REFUNDED,
                'settle_time' => $now,
                'settle_remark' => $reason,
                'update_time' => $now,
            ));
            Db::name('sports_settle_log')->insert(array(
                'match_id' => $matchId,
                'settle_key' => 'admin_refund_' . $matchId,
                'home_score' => $match['home_score'],
                'away_score' => $match['away_score'],
                'orders_total' => $ordersTotal,
                'orders_win' => 0,
                'orders_lose' => 0,
                'orders_refund' => $ordersTotal,
                'payout_total' => $refundTotal,
                'success' => 1,
                'message' => $reason,
                'create_time' => $now,
            ));
            Db::commit();
        } catch (\Exception $e) {
            Db::rollback();
            return array('error' => '退款失败: ' . $e->getMessage());
        }

        return array('orders_total' => $ordersTotal);
    }

    protected function marketDefinitions() {
        $definitions = array(
            'CORRECT_SCORE' => array('name' => '比分', 'rule' => 'correct_score', 'sort' => 100, 'options' => array()),
            'MATCH_RESULT' => array('name' => '主胜/平/主败', 'rule' => 'match_result', 'sort' => 90, 'options' => array(
                array('HOME_WIN', '主胜', 100),
                array('DRAW', '平局', 90),
                array('AWAY_WIN', '主败', 80),
            )),
            'TOTAL_GOALS' => array('name' => '总进球', 'rule' => 'total_goals', 'sort' => 80, 'options' => array()),
            'HOME_GOALS' => array('name' => '主队进球', 'rule' => 'home_goals', 'sort' => 70, 'options' => array()),
            'AWAY_GOALS' => array('name' => '客队进球', 'rule' => 'away_goals', 'sort' => 60, 'options' => array()),
        );
        $sort = 1000;
        for ($home = 0; $home <= 7; $home++) {
            for ($away = 0; $away <= 7; $away++) {
                $definitions['CORRECT_SCORE']['options'][] = array('CS_' . $home . '_' . $away, $home . ':' . $away, $sort--);
            }
        }
        $definitions['CORRECT_SCORE']['options'][] = array('OTHER', '其他比分', 1);
        for ($goals = 0; $goals <= 15; $goals++) {
            $definitions['TOTAL_GOALS']['options'][] = array('TG_' . $goals, $goals . '球', 100 - $goals);
        }
        $definitions['TOTAL_GOALS']['options'][] = array('OTHER', '其他(16+)', 1);
        for ($goals = 0; $goals <= 7; $goals++) {
            $definitions['HOME_GOALS']['options'][] = array('HG_' . $goals, $goals . '球', 100 - $goals);
            $definitions['AWAY_GOALS']['options'][] = array('AG_' . $goals, $goals . '球', 100 - $goals);
        }
        return $definitions;
    }

    protected function validateOptionRule($marketCode, $optionCode) {
        $marketCode = strtoupper(trim((string)$marketCode));
        $optionCode = strtoupper(trim((string)$optionCode));
        if ($marketCode === 'TOTAL_GOALS') {
            if ($optionCode === 'OTHER') {
                return '';
            }
            if (preg_match('/^TG_(\d+)$/', $optionCode, $m) && (int)$m[1] <= 15) {
                return '';
            }
            return '总进球投注项只能配置 0-15，超过 15 请使用其他';
        }
        if ($marketCode === 'HOME_GOALS' || $marketCode === 'AWAY_GOALS') {
            $prefix = $marketCode === 'HOME_GOALS' ? 'HG_' : 'AG_';
            if (preg_match('/^' . $prefix . '(\d+)$/', $optionCode, $m) && (int)$m[1] <= 7) {
                return '';
            }
            return '主客进球投注项只能配置 0-7';
        }
        if ($marketCode === 'MATCH_RESULT') {
            return in_array($optionCode, array('HOME_WIN', 'DRAW', 'AWAY_WIN'), true) ? '' : '胜负投注项编码错误';
        }
        if ($marketCode === 'CORRECT_SCORE') {
            return $optionCode === 'OTHER' || preg_match('/^CS_([0-7])_([0-7])$/', $optionCode) ? '' : '比分投注项编码错误';
        }
        return '玩法编码错误';
    }

    protected function isFinishedStatus($status) {
        return in_array((string)$status, array('FT', 'Fin', 'Final', 'Res', 'AET', 'Pen', 'PEN', 'AWD', 'WO'), true);
    }

    protected function isItemWin($match, $marketCode, $optionCode) {
        $marketCode = strtoupper((string)$marketCode);
        $optionCode = strtoupper((string)$optionCode);
        $home = (int)$match['home_score'];
        $away = (int)$match['away_score'];

        if ($marketCode === 'CORRECT_SCORE') {
            return $optionCode === 'OTHER' ? ($home > 7 || $away > 7) : $optionCode === 'CS_' . $home . '_' . $away;
        }
        if ($marketCode === 'MATCH_RESULT') {
            return ($home > $away && $optionCode === 'HOME_WIN')
                || ($home === $away && $optionCode === 'DRAW')
                || ($home < $away && $optionCode === 'AWAY_WIN');
        }
        if ($marketCode === 'TOTAL_GOALS') {
            $total = $home + $away;
            return $total > 15 ? $optionCode === 'OTHER' : $optionCode === 'TG_' . $total;
        }
        if ($marketCode === 'HOME_GOALS') {
            return $home <= 7 && $optionCode === 'HG_' . $home;
        }
        if ($marketCode === 'AWAY_GOALS') {
            return $away <= 7 && $optionCode === 'AG_' . $away;
        }
        return false;
    }
}
