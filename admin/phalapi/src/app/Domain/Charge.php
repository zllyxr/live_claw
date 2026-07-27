<?php
namespace App\Domain;
use App\Model\Charge as Model_Charge;

class Charge{
	public function getOrderId($changeid,$orderinfo) {
		$rs = array();

		$model = new Model_Charge();
		$rs = $model->getOrderId($changeid,$orderinfo);

		return $rs;
	}

	public function getFirstChargeRules(){
		$rs = array();

		$model = new Model_Charge();
		$rs = $model->getFirstChargeRules();

		return $rs;
	}


	
}
