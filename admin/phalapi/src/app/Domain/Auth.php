<?php
namespace App\Domain;
use App\Model\Auth as Model_Auth;

class Auth {
	public function getAuth($uid) {
		$rs = array();

		$model = new Model_Auth();
		$rs = $model->getAuth($uid);

		return $rs;
	}
	
	
	public function setAuth($data) {
		$rs = array();

		$model = new Model_Auth();
		$rs = $model->setAuth($data);

		return $rs;
	}


	
}
