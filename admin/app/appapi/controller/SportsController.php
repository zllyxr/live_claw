<?php
namespace app\appapi\controller;

use app\common\controller\HomeBaseController;

class SportsController extends HomeBaseController {
    protected function assignSportsPageVars() {
        $matchId = (string)$this->request->param('match_id', '');
        $uid = (string)$this->request->param('uid', '');
        $token = (string)$this->request->param('token', '');
        $language = (string)$this->request->param('language', '');
        $competitionType = (string)$this->request->param('competition_type', '');

        $this->assign('match_id', $matchId);
        $this->assign('uid', $uid);
        $this->assign('token', $token);
        $this->assign('language', $language);
        $this->assign('competition_type', $competitionType);
        $this->assign('match_id_json', json_encode($matchId, JSON_UNESCAPED_UNICODE));
        $this->assign('uid_json', json_encode($uid, JSON_UNESCAPED_UNICODE));
        $this->assign('token_json', json_encode($token, JSON_UNESCAPED_UNICODE));
        $this->assign('language_json', json_encode($language, JSON_UNESCAPED_UNICODE));
        $this->assign('competition_type_json', json_encode($competitionType, JSON_UNESCAPED_UNICODE));
    }

    public function detail() {
        $this->assignSportsPageVars();
        return $this->fetch();
    }

    public function bet() {
        $this->assignSportsPageVars();
        return $this->fetch();
    }

    public function orders() {
        $this->assignSportsPageVars();
        return $this->fetch();
    }
}
