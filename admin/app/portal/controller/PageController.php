<?php

namespace app\portal\controller;
use app\common\controller\HomeBaseController;
use think\facade\Db;

class PageController extends HomebaseController{
	public function index() {

        $data = $this->request->param();
        $id=(int)($data['id'] ?? 0);
        $ish5=isset($data['ish5']) ? $data['ish5']: '0';
		$this->assign('ish5', $ish5);
        if(!$id){
            $this->assign('page', [
                'post_title'=>lang('信息错误'),
                'post_content'=>lang('信息错误'),
            ]);
            return $this->fetch();
        }

        $page=Db::name("portal_post")->where(['id'=>$id])->find();
        if(!$page){
            $this->assign('page', [
                'post_title'=>lang('信息错误'),
                'post_content'=>lang('信息错误'),
            ]);
            return $this->fetch();
        }
        $page['post_content']=html_entity_decode($page['post_content']);

        //语言包
        $language=$this->language;
        if($language=='en' && !empty($page['post_title_en'])){
            $page['post_title']=$page['post_title_en'];
        }
        
        $this->assign('page', $page);
		
		return $this->fetch();
	}
}
