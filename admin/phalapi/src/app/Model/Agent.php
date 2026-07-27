<?php
namespace App\Model;
use PhalApi\Model\NotORMModel as NotORM;

class Agent extends NotORM {

	public function getCode($uid) {
		
        $agentinfo=\PhalApi\DI()->notorm->agent_code
            ->select('code')
            ->where('uid=?',$uid)
            ->fetchOne();
            
		return $agentinfo;
	}			

}
