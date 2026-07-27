<?php
namespace App\Api;

use PhalApi\Api;
use App\Domain\Invite as Domain_Invite;

/**
 * 邀请归因
 */
class Invite extends Api {

	public function getRules() {
		return array(
			'resolve' => array(
				'ref' => array('name' => 'ref', 'type' => 'string', 'default' => '', 'desc' => '邀请短链 ref'),
				'code' => array('name' => 'code', 'type' => 'string', 'default' => '', 'desc' => '邀请码'),
				'click_id' => array('name' => 'click_id', 'type' => 'string', 'default' => '', 'desc' => '点击ID'),
				'platform' => array('name' => 'platform', 'type' => 'string', 'default' => '', 'desc' => '平台 android/ios/web'),
				'device_id' => array('name' => 'device_id', 'type' => 'string', 'default' => '', 'desc' => '设备指纹'),
				'idfa' => array('name' => 'idfa', 'type' => 'string', 'default' => '', 'desc' => 'iOS IDFA'),
				'idfv' => array('name' => 'idfv', 'type' => 'string', 'default' => '', 'desc' => 'iOS IDFV'),
				'oaid' => array('name' => 'oaid', 'type' => 'string', 'default' => '', 'desc' => 'Android OAID'),
				'android_id' => array('name' => 'android_id', 'type' => 'string', 'default' => '', 'desc' => 'Android ID'),
				'user_agent' => array('name' => 'user_agent', 'type' => 'string', 'default' => '', 'desc' => '客户端UA'),
			),
			'bind' => array(
				'uid' => array('name' => 'uid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '用户ID'),
				'token' => array('name' => 'token', 'type' => 'string', 'require' => true, 'desc' => '用户token'),
				'ref' => array('name' => 'ref', 'type' => 'string', 'default' => '', 'desc' => '邀请短链 ref'),
				'code' => array('name' => 'code', 'type' => 'string', 'default' => '', 'desc' => '邀请码'),
				'click_id' => array('name' => 'click_id', 'type' => 'string', 'default' => '', 'desc' => '点击ID'),
				'platform' => array('name' => 'platform', 'type' => 'string', 'default' => '', 'desc' => '平台 android/ios/web'),
				'device_id' => array('name' => 'device_id', 'type' => 'string', 'default' => '', 'desc' => '设备指纹'),
				'idfa' => array('name' => 'idfa', 'type' => 'string', 'default' => '', 'desc' => 'iOS IDFA'),
				'idfv' => array('name' => 'idfv', 'type' => 'string', 'default' => '', 'desc' => 'iOS IDFV'),
				'oaid' => array('name' => 'oaid', 'type' => 'string', 'default' => '', 'desc' => 'Android OAID'),
				'android_id' => array('name' => 'android_id', 'type' => 'string', 'default' => '', 'desc' => 'Android ID'),
				'user_agent' => array('name' => 'user_agent', 'type' => 'string', 'default' => '', 'desc' => '客户端UA'),
			),
		);
	}

	/**
	 * 解析邀请来源
	 * @desc App 首次打开或登录前查询邀请来源，不写入上下级关系
	 */
	public function resolve() {
		$rs = array('code' => 0, 'msg' => '', 'info' => array());

		$domain = new Domain_Invite();
		$rs['info'][0] = $domain->resolve($this->collectParams());

		return $rs;
	}

	/**
	 * 绑定邀请来源
	 * @desc App 登录后根据邀请归因绑定分销上级
	 */
	public function bind() {
		$rs = array('code' => 0, 'msg' => '', 'info' => array());

		$uid = \App\checkNull($this->uid);
		$token = \App\checkNull($this->token);
		$checkToken = \App\checkToken($uid, $token);
		if ($checkToken == 700) {
			$rs['code'] = $checkToken;
			$rs['msg'] = \PhalApi\T('您的登陆状态失效，请重新登陆！');
			return $rs;
		} else if ($checkToken == 10020) {
			$rs['code'] = 700;
			$rs['msg'] = \PhalApi\T('该账号已被禁用');
			return $rs;
		}

		$domain = new Domain_Invite();
		$result = $domain->bind($uid, $this->collectParams());
		if (!empty($result['api_code'])) {
			$rs['code'] = $result['api_code'];
			$rs['msg'] = $result['api_msg'];
			unset($result['api_code'], $result['api_msg']);
		}

		$rs['info'][0] = $result;
		return $rs;
	}

	private function collectParams() {
		return array(
			'ref' => \App\checkNull($this->ref),
			'code' => \App\checkNull($this->code),
			'click_id' => \App\checkNull($this->click_id),
			'platform' => \App\checkNull($this->platform),
			'device_id' => \App\checkNull($this->device_id),
			'idfa' => \App\checkNull($this->idfa),
			'idfv' => \App\checkNull($this->idfv),
			'oaid' => \App\checkNull($this->oaid),
			'android_id' => \App\checkNull($this->android_id),
			'user_agent' => \App\checkNull($this->user_agent),
		);
	}
}
