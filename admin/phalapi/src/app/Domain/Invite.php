<?php
namespace App\Domain;

use App\Model\Invite as Model_Invite;

class Invite {
	public function resolve($params) {
		$model = new Model_Invite();
		return $model->resolve($params);
	}

	public function bind($uid, $params) {
		$model = new Model_Invite();
		return $model->bind($uid, $params);
	}
}
