<?php
// +----------------------------------------------------------------------
// | ThinkCMF [ WE CAN DO IT MORE SIMPLE ]
// +----------------------------------------------------------------------
// | Copyright (c) 2013-2014 http://www.thinkcmf.com All rights reserved.
// +----------------------------------------------------------------------
// | Author: Dean <zxxjjforever@163.com>
// +----------------------------------------------------------------------
namespace app\home\controller;

use app\common\controller\HomeBaseController;
use think\facade\Db;
/**
 * 单页
 */
class PageController extends HomebaseController {
	
    //服务条款
	public function agreement() {
        
        $id=4;
        $agreement = Db::name("portal_post")->where('id', $id)->find();
        $agreement['post_content']=htmlspecialchars_decode($agreement['post_content']);
        $this->assign("agreement",$agreement);
			
    	return $this->fetch();
    }	


}


