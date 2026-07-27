<?php
/**
 * 下载页面
 */
namespace app\appapi\controller;

use app\common\controller\HomeBaseController;

class DownController extends HomebaseController {

	function index(){       
		return $this->fetch();
	}

}