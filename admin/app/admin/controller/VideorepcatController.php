<?php

/**
 * 短视频-举报分类
 */
namespace app\admin\controller;

use cmf\controller\AdminBaseController;
use think\facade\Db;

class VideorepcatController extends AdminBaseController {
    
	
	function index(){
        $lists = Db::name("video_report_classify")
			->order("list_order asc")
			->paginate(20);

        $page = $lists->render();

    	$this->assign('lists', $lists);
    	$this->assign("page", $page);
    	return $this->fetch();
	}
    
    function del(){
        
        $id = $this->request->param('id', 0, 'intval');
        
        $rs = DB::name('video_report_classify')->where("id={$id}")->delete();
        if($rs===false){
            $this->error("删除失败！");
        }
		
		$action="视频管理-删除举报分类ID: ".$id;
		setAdminLog($action);
        $this->resetCache();
        $this->success("删除成功！");
	}
    
    //排序
    public function listOrder() { 
		
        $model = DB::name('video_report_classify');
        parent::listOrders($model);

		$action="视频管理-更新举报分类排序";
		setAdminLog($action);
        $this->resetCache();
        $this->success("排序更新成功！");
    }	
    
    function add(){
		return $this->fetch();
	}

	function addPost(){
		if ($this->request->isPost()) {
            
            $data  = $this->request->param();
            
			$name=$data['name'];
			$name_en=$data['name_en'];

			if($name==""){
				$this->error("请填写中文名称");
			}
            
            $isexit=DB::name("video_report_classify")->where(['name'=>$name])->find();	
			if($isexit){
				$this->error('中文名称已存在');
			}

			if($name_en==""){
				$this->error("请填写英文名称");
			}
            
            $isexit=DB::name("video_report_classify")->where(['name_en'=>$name_en])->find();	
			if($isexit){
				$this->error('英文名称已存在');
			}
			
            $data['addtime']=time();
            
			$id = DB::name('video_report_classify')->insertGetId($data);
            if(!$id){
                $this->error("添加失败！");
            }
			
			$action="视频管理-添加举报分类ID: ".$id;
			setAdminLog($action);
            $this->resetCache();
            $this->success("添加成功！");
		}
	}

    function edit(){
        
        $id   = $this->request->param('id', 0, 'intval');
        $data=Db::name('video_report_classify')
            ->where("id={$id}")
            ->find();
        if(!$data){
            $this->error("信息错误");
        }
        
        $this->assign('data', $data);
        return $this->fetch();
	}
	
	function editPost(){
		if ($this->request->isPost()) {
            
            $data = $this->request->param();
            
			$name=$data['name'];
			$name_en=$data['name_en'];
			$id=$data['id'];

			if($name==""){
				$this->error("请填写中文名称");
			}
            $where=[];
            $where[]=['id','<>',$id];
            $where[]=['name','=',$name];
            $isexit=Db::name("video_report_classify")->where($where)->find();	
			if($isexit){
				$this->error('该中文名称已存在');
			}

			if($name_en==""){
				$this->error("请填写英文名称");
			}
            $where=[];
            $where[]=['id','<>',$id];
            $where[]=['name_en','=',$name_en];
            $isexit=Db::name("video_report_classify")->where($where)->find();	
			if($isexit){
				$this->error('该英文名称已存在');
			}

			$rs = DB::name('video_report_classify')->update($data);
            if($rs===false){
                $this->error("修改失败！");
            }

			$action="视频管理-编辑举报分类ID: ".$id;
			setAdminLog($action);
            $this->resetCache();
            $this->success("修改成功！");
		}
	}

    function resetCache(){
        $key='getVideoRepcat';
        $rules= Db::name("video_report_classify")
            ->order('list_order asc,id desc')
            ->select();
        if($rules){
            setcaches($key,$rules);
        }else{
            delcache($key);
        }

        return 1;
    }
    
}
