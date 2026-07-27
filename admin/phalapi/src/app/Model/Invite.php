<?php
namespace App\Model;

use PhalApi\Model\NotORMModel as NotORM;
use App\Model\User as Model_User;

class Invite extends NotORM {
	const CLICK_TTL = 604800;
	const IP_RECENT_TTL = 7200;
	const AUTO_BIND_CONFIDENCE = 60;

	public function resolve($params) {
		$params = $this->normalizeParams($params);
		$now = time();
		$client = $this->getClientContext($params);
		$candidate = $this->findCandidate($params, $client, $now);

		if (!$candidate) {
			return $this->emptyResolve();
		}

		if (!empty($candidate['click_id'])) {
			\PhalApi\DI()->notorm->invite_click
				->where('click_id=?', $candidate['click_id'])
				->update(array(
					'status' => 1,
					'updated_at' => $now,
					'device_fingerprint' => $client['device_fingerprint'] ?: $candidate['device_fingerprint'],
				));
		}

		return array(
			'matched' => '1',
			'code' => $candidate['ref_code'],
			'inviter_uid' => (string)$candidate['inviter_uid'],
			'click_id' => $candidate['click_id'],
			'match_method' => $candidate['match_method'],
			'confidence' => (string)$candidate['confidence'],
			'auto_bind' => $candidate['confidence'] >= self::AUTO_BIND_CONFIDENCE ? '1' : '0',
			'expires_at' => (string)$candidate['expires_at'],
		);
	}

	public function bind($uid, $params) {
		$params = $this->normalizeParams($params);
		$now = time();
		$client = $this->getClientContext($params);

		$isBound = \PhalApi\DI()->notorm->agent
			->select('uid,one_uid')
			->where('uid=?', $uid)
			->fetchOne();
		if ($isBound) {
			$result = array(
				'matched' => '1',
				'already_bound' => '1',
				'bound' => '1',
				'inviter_uid' => (string)$isBound['one_uid'],
				'match_method' => 'existing',
				'confidence' => '100',
				'msg' => \PhalApi\T('已设置，不能更改'),
			);
			$this->recordBind($uid, $isBound['one_uid'], '', '', 'existing', 100, $client, 3, 1004, $result['msg'], $now);
			return $result;
		}

		$candidate = $this->findCandidate($params, $client, $now);
		if (!$candidate) {
			return array(
				'matched' => '0',
				'bound' => '0',
				'api_code' => 1002,
				'api_msg' => \PhalApi\T('邀请码错误'),
				'msg' => \PhalApi\T('未匹配到邀请来源'),
			);
		}

		if ($candidate['confidence'] < self::AUTO_BIND_CONFIDENCE) {
			$msg = \PhalApi\T('未匹配到可靠邀请来源');
			$this->recordBind($uid, $candidate['inviter_uid'], $candidate['ref_code'], $candidate['click_id'], $candidate['match_method'], $candidate['confidence'], $client, 2, 1005, $msg, $now);
			return array(
				'matched' => '1',
				'bound' => '0',
				'code' => $candidate['ref_code'],
				'inviter_uid' => (string)$candidate['inviter_uid'],
				'click_id' => $candidate['click_id'],
				'match_method' => $candidate['match_method'],
				'confidence' => (string)$candidate['confidence'],
				'api_code' => 1005,
				'api_msg' => $msg,
				'msg' => $msg,
			);
		}

		$userModel = new Model_User();
		$bindResult = $userModel->setDistribut($uid, $candidate['ref_code']);
		$msg = $this->bindResultMessage($bindResult);
		$bound = $bindResult === 0;
		$status = $bound ? 1 : 2;
		if ($bindResult === 1004) {
			$status = 3;
		}

		$this->recordBind($uid, $candidate['inviter_uid'], $candidate['ref_code'], $candidate['click_id'], $candidate['match_method'], $candidate['confidence'], $client, $status, $bindResult, $msg, $now);

		if (!empty($candidate['click_id'])) {
			\PhalApi\DI()->notorm->invite_click
				->where('click_id=?', $candidate['click_id'])
				->update(array(
					'status' => $bound ? 2 : 1,
					'matched_uid' => $uid,
					'device_fingerprint' => $client['device_fingerprint'] ?: $candidate['device_fingerprint'],
					'updated_at' => $now,
				));
		}

		$result = array(
			'matched' => '1',
			'bound' => $bound ? '1' : '0',
			'code' => $candidate['ref_code'],
			'inviter_uid' => (string)$candidate['inviter_uid'],
			'click_id' => $candidate['click_id'],
			'match_method' => $candidate['match_method'],
			'confidence' => (string)$candidate['confidence'],
			'msg' => $msg,
		);

		if (!$bound && $bindResult !== 1004) {
			$result['api_code'] = $bindResult ?: 1002;
			$result['api_msg'] = $msg;
		}

		return $result;
	}

	private function findCandidate($params, $client, $now) {
		$code = $params['code'] ?: $params['ref'];
		if ($code !== '') {
			$inviter = $this->findInviterByCode($code);
			if ($inviter) {
				return array(
					'ref_code' => $inviter['code'],
					'inviter_uid' => (int)$inviter['uid'],
					'click_id' => '',
					'device_fingerprint' => $client['device_fingerprint'],
					'match_method' => 'direct_ref',
					'confidence' => 100,
					'expires_at' => (string)($now + self::CLICK_TTL),
				);
			}
		}

		if ($params['click_id'] !== '') {
			$click = \PhalApi\DI()->notorm->invite_click
				->where('click_id=? and expires_at>=?', $params['click_id'], $now)
				->fetchOne();
			if ($click) {
				return $this->clickToCandidate($click, 'click_id', 95);
			}
		}

		if ($client['device_fingerprint'] !== '') {
			$click = \PhalApi\DI()->notorm->invite_click
				->where('device_fingerprint=? and expires_at>=?', $client['device_fingerprint'], $now)
				->order('created_at desc')
				->fetchOne();
			if ($click) {
				return $this->clickToCandidate($click, 'device_fingerprint', 90);
			}
		}

		if ($client['ip_hash'] !== '' && $client['ua_hash'] !== '') {
			$click = \PhalApi\DI()->notorm->invite_click
				->where('ip_hash=? and ua_hash=? and expires_at>=?', $client['ip_hash'], $client['ua_hash'], $now)
				->order('created_at desc')
				->fetchOne();
			if ($click) {
				return $this->clickToCandidate($click, 'ip_ua', 70);
			}
		}

		if ($client['ip_hash'] !== '') {
			$click = \PhalApi\DI()->notorm->invite_click
				->where('ip_hash=? and created_at>=? and expires_at>=?', $client['ip_hash'], $now - self::IP_RECENT_TTL, $now)
				->order('created_at desc')
				->fetchOne();
			if ($click) {
				return $this->clickToCandidate($click, 'ip_recent', 40);
			}
		}

		return null;
	}

	private function clickToCandidate($click, $method, $confidence) {
		return array(
			'ref_code' => $click['ref_code'],
			'inviter_uid' => (int)$click['inviter_uid'],
			'click_id' => $click['click_id'],
			'device_fingerprint' => $click['device_fingerprint'],
			'match_method' => $method,
			'confidence' => $confidence,
			'expires_at' => (string)$click['expires_at'],
		);
	}

	private function findInviterByCode($code) {
		$code = $this->normalizeCode($code);
		if ($code === '') {
			return null;
		}
		$info = \PhalApi\DI()->notorm->agent_code
			->select('uid,code')
			->where('code=?', $code)
			->fetchOne();
		return $info ?: null;
	}

	private function recordBind($uid, $inviterUid, $refCode, $clickId, $matchMethod, $confidence, $client, $status, $resultCode, $resultMsg, $now) {
		$data = array(
			'invitee_uid' => $uid,
			'inviter_uid' => (int)$inviterUid,
			'ref_code' => $refCode,
			'click_id' => $clickId,
			'match_method' => $matchMethod,
			'confidence' => (int)$confidence,
			'device_fingerprint' => $client['device_fingerprint'],
			'ip' => $client['ip'],
			'ua_hash' => $client['ua_hash'],
			'status' => $status,
			'bind_result_code' => (int)$resultCode,
			'bind_result_msg' => mb_substr($resultMsg, 0, 255),
			'created_at' => $now,
			'updated_at' => $now,
		);
		$setData = $data;
		unset($setData['created_at']);
		return \PhalApi\DI()->notorm->invite_bind
			->insert_update(array('invitee_uid' => $uid), $data, $setData);
	}

	private function bindResultMessage($result) {
		if ($result === 0) {
			return \PhalApi\T('设置成功');
		}
		if ($result == 1004) {
			return \PhalApi\T('已设置，不能更改');
		}
		if ($result == 1003) {
			return \PhalApi\T('不能填写自己下级的邀请码');
		}
		return \PhalApi\T('邀请码错误');
	}

	private function emptyResolve() {
		return array(
			'matched' => '0',
			'code' => '',
			'inviter_uid' => '0',
			'click_id' => '',
			'match_method' => '',
			'confidence' => '0',
			'auto_bind' => '0',
			'expires_at' => '0',
		);
	}

	private function getClientContext($params) {
		$ua = $params['user_agent'] ?: ($_SERVER['HTTP_USER_AGENT'] ?? '');
		$ip = $this->getClientIp();
		return array(
			'ip' => mb_substr($ip, 0, 45),
			'ip_hash' => $ip !== '' ? sha1($ip) : '',
			'user_agent' => mb_substr($ua, 0, 512),
			'ua_hash' => $ua !== '' ? sha1($ua) : '',
			'device_fingerprint' => $this->normalizeDeviceFingerprint($params),
		);
	}

	private function normalizeParams($params) {
		$keys = array('ref', 'code', 'click_id', 'platform', 'device_id', 'idfa', 'idfv', 'oaid', 'android_id', 'user_agent');
		foreach ($keys as $key) {
			$params[$key] = isset($params[$key]) ? trim((string)$params[$key]) : '';
		}
		$params['ref'] = $this->normalizeCode($params['ref']);
		$params['code'] = $this->normalizeCode($params['code']);
		$params['click_id'] = preg_replace('/[^A-Za-z0-9_\\-]/', '', $params['click_id']);
		return $params;
	}

	private function normalizeCode($code) {
		$code = strtoupper(trim((string)$code));
		return preg_replace('/[^A-Z0-9]/', '', $code);
	}

	private function normalizeDeviceFingerprint($params) {
		$parts = array(
			$params['device_id'] ?? '',
			$params['idfa'] ?? '',
			$params['idfv'] ?? '',
			$params['oaid'] ?? '',
			$params['android_id'] ?? '',
		);
		$parts = array_filter(array_map('trim', $parts));
		if (!$parts) {
			return '';
		}
		return sha1(implode('|', $parts));
	}

	private function getClientIp() {
		$headers = array('HTTP_X_FORWARDED_FOR', 'HTTP_X_REAL_IP', 'REMOTE_ADDR');
		foreach ($headers as $header) {
			if (empty($_SERVER[$header])) {
				continue;
			}
			$value = $_SERVER[$header];
			if ($header === 'HTTP_X_FORWARDED_FOR') {
				$value = explode(',', $value)[0];
			}
			$value = trim($value);
			if ($value !== '') {
				return $value;
			}
		}
		return '';
	}
}
