<?php
namespace app\appapi\controller;

use app\common\controller\HomeBaseController;

class LotterygameController extends HomeBaseController {
    public function detail() {
        $gameId = (int)$this->request->param('game_id', 0);
        $gameCode = (string)$this->request->param('game_code', '');
        $uid = (string)$this->request->param('uid', '');
        $token = (string)$this->request->param('token', '');
        $language = (string)$this->request->param('language', '');
        $liveuid = (string)$this->request->param('liveuid', '');
        $stream = (string)$this->request->param('stream', '');
        $overlay = (string)$this->request->param('overlay', '0');

        $this->assign('game_id', $gameId);
        $this->assign('game_code', $gameCode);
        $this->assign('uid', $uid);
        $this->assign('token', $token);
        $this->assign('language', $language);
        $this->assign('liveuid', $liveuid);
        $this->assign('stream', $stream);
        $this->assign('overlay', $overlay);
        $this->assign('game_id_json', json_encode((string)$gameId, JSON_UNESCAPED_UNICODE));
        $this->assign('game_code_json', json_encode($gameCode, JSON_UNESCAPED_UNICODE));
        $this->assign('uid_json', json_encode($uid, JSON_UNESCAPED_UNICODE));
        $this->assign('token_json', json_encode($token, JSON_UNESCAPED_UNICODE));
        $this->assign('language_json', json_encode($language, JSON_UNESCAPED_UNICODE));
        $this->assign('liveuid_json', json_encode($liveuid, JSON_UNESCAPED_UNICODE));
        $this->assign('stream_json', json_encode($stream, JSON_UNESCAPED_UNICODE));
        $this->assign('overlay_json', json_encode($overlay === '1', JSON_UNESCAPED_UNICODE));

        return $this->fetch();
    }

    public function record() {
        $gameId = (int)$this->request->param('game_id', 0);
        $gameCode = (string)$this->request->param('game_code', '');
        $uid = (string)$this->request->param('uid', '');
        $token = (string)$this->request->param('token', '');
        $language = (string)$this->request->param('language', '');
        $liveuid = (string)$this->request->param('liveuid', '');
        $stream = (string)$this->request->param('stream', '');
        $overlay = (string)$this->request->param('overlay', '0');

        $this->assign('game_id', $gameId);
        $this->assign('game_code', $gameCode);
        $this->assign('uid', $uid);
        $this->assign('token', $token);
        $this->assign('language', $language);
        $this->assign('liveuid', $liveuid);
        $this->assign('stream', $stream);
        $this->assign('overlay', $overlay);
        $this->assign('game_id_json', json_encode((string)$gameId, JSON_UNESCAPED_UNICODE));
        $this->assign('game_code_json', json_encode($gameCode, JSON_UNESCAPED_UNICODE));
        $this->assign('uid_json', json_encode($uid, JSON_UNESCAPED_UNICODE));
        $this->assign('token_json', json_encode($token, JSON_UNESCAPED_UNICODE));
        $this->assign('language_json', json_encode($language, JSON_UNESCAPED_UNICODE));
        $this->assign('liveuid_json', json_encode($liveuid, JSON_UNESCAPED_UNICODE));
        $this->assign('stream_json', json_encode($stream, JSON_UNESCAPED_UNICODE));
        $this->assign('overlay_json', json_encode($overlay === '1', JSON_UNESCAPED_UNICODE));

        return $this->fetch();
    }
}
