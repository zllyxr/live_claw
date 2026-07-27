<?php

namespace app\home\controller;

use app\common\controller\HomeBaseController;
use think\facade\Db;

class AppController extends HomebaseController{

    public function programe(){

        //获取轮播图
        $slide_list = Db::name("slide_item")->where(['slide_id'=>7,'status'=>1])->order('list_order')->select()->toArray();;

        foreach ($slide_list as $k => $v) {
            $slide_list[$k]['image']=get_upload_path($v['image']);
        }

    	$this->assign("current","download");
        $this->assign("slide_list",$slide_list);

        return $this->fetch();
    }



}
