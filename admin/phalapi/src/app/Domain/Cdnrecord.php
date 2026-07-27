<?php
namespace App\Domain;
use App\Model\Cdnrecord as Model_Cdnrecord;


class Cdnrecord {
	public function getCdnRecord($id) {
        $rs = array();
                
        $model = new Model_Cdnrecord();
        $rs = $model->getCdnRecord($id);

        return $rs;
    }	
	
}
