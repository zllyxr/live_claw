<?php

namespace app\portal\controller;

use cmf\controller\AdminBaseController;

use think\facade\Db;

class AdminPageController extends AdminBaseController
{

    /**
     * 页面管理
     */
    public function index(){
  
		$data = $this->request->param();
		$map=[];
		if(!empty($data['keyword'])){
            $map[]=['post_title','like',"%".$data['keyword']."%"];
        }
		
		$lists = Db::name("portal_post")
            ->where($map)
			->order("create_time desc")
			->paginate(20);
        
        $lists->each(function($v,$k){

			$userinfo=$this->getUserInfo($v['user_id']);
			if(!empty($userinfo)){
                $v['user_nickname']=$userinfo['user_nickname'];
            }else{
                $v['user_nickname']='用户不存在';
            }
            return $v;           
        });
        
        $page = $lists->render();

    	$this->assign('pages', $lists);

    	$this->assign("page", $page);

        return $this->fetch();
    }

	function getUserInfo($uid){
		$map[]=['id','=',$uid];
		return Db::name('user')
					->field('id,user_nickname')
					->where($map)
					->find();	
	}


    public function add(){
        
        return $this->fetch();
    }


    public function addPost(){

		if ($this->request->isPost()) {
       
			$nowtime=time();
            $data = $this->request->param();
			$save_data=array(
                "type"=>$data['type'],
				"user_id"=>cmf_get_current_admin_id(),
				"create_time"=>$nowtime,
				"post_title"=>$data['post_title'],
                "post_title_en"=>$data['post_title_en'],
				"post_content"=>$data['post_content'],
			);
			
			$id = DB::name('portal_post')->insertGetId($save_data);
            if(!$id){
                $this->error("添加失败！");
            }

            $action="添加页面管理ID: ".$id;
            setAdminLog($action);

            $this->success("保存成功！");
        }

    }


    public function edit(){
		$id   = $this->request->param('id', 0, 'intval');
        
        $data=Db::name('portal_post')
            ->where("id={$id}")
            ->find();
        if(!$data){
            $this->error("信息错误");
        }
		
        $this->assign('post', $data);

        return $this->fetch();
    }


    public function editPost(){
        if ($this->request->isPost()) {
			
			$data = $this->request->param();

			$save_data=array(
				"id"=>$data['id'],
				"post_title"=>$data['post_title'],
                "post_title_en"=>$data['post_title_en'],
				"post_content"=>$data['post_content'],
                "update_time"=>time(),
                "type"=>$data['type']
			);

			$rs = DB::name('portal_post')->update($save_data);
            if($rs===false){
                $this->error("修改失败！");
            }
		
			$this->success("保存成功！");
		}

    }


    public function delete(){

		$id = $this->request->param('id', 0, 'intval');
        
        $rs = DB::name('portal_post')->where("id={$id}")->delete();
        if(!$rs){
            $this->error("删除失败！");
        }
        
        $this->success("删除成功！");
        

    }

    public function listOrder(){
        $model = DB::name('portal_post');
        parent::listOrders($model);
        $action="更新页面管理排序 ";
        setAdminLog($action);

        $this->success("排序更新成功！");
    }

}
