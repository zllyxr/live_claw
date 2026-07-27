<?php
namespace App\Domain;
use App\Model\Agent as Model_Agent;

class Agent {
	public function getCode($uid) {
		$rs = array();

		$model = new Model_Agent();
		$rs = $model->getCode($uid);

		return $rs;
	}
	
}
