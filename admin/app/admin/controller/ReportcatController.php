<?php

/**
 * 直播举报类型
 */
namespace app\admin\controller;

use cmf\controller\AdminBaseController;
use think\facade\Db;

class ReportcatController extends AdminBaseController {
    function index(){
        
        $data = $this->request->param();
        $map=[];
        $keyword= $data['keyword'] ?? '';
        if($keyword!=''){
            $map[]=['name','like',"%".$keyword."%"];
        }
        
        $lists = Db::name("report_classify")
                ->where($map)
                ->order("list_order asc")
                ->paginate(20);
                
        $lists->appends($data);
        $page = $lists->render();

    	$this->assign('lists', $lists);
    	$this->assign("page", $page);
    	
    	return $this->fetch();
    }
    
    //排序
    public function listOrder() { 
		
        $model = DB::name('report_classify');
        parent::listOrders($model);
		
		$action="更新直播举报类型排序";
        setAdminLog($action);
        
        $this->success("排序更新成功！");
        
    }
    
	function del(){
        
        $id = $this->request->param('id', 0, 'intval');
        
        $rs = DB::name('report_classify')->where("id={$id}")->delete();
        if(!$rs){
            $this->error("删除失败！");
        }
        
        $action="删除直播举报类型：{$id}";
        setAdminLog($action);
        
        $this->success("删除成功！",url("reportcat/index"));
	}	

		
    function add(){
        return $this->fetch();
    }
    
	function addPost(){
		if ($this->request->isPost()) {
            
            $data  = $this->request->param();
            
            $data['name']=trim($data['name']);
            $data['name_en']=trim($data['name_en']);
			$name=$data['name'];
            $name_en=$data['name_en'];

			if($name==""){
				$this->error("中文名称不能为空");
			}
            
            $isexit=DB::name('report_classify')->where(["name"=>$name])->find();	
            if($isexit){
                $this->error('该中文名称已存在');
            }

            if($name_en==""){
                $this->error("英文名称不能为空");
            }

            $isexit=DB::name('report_classify')->where(["name_en"=>$name_en])->find();    
            if($isexit){
                $this->error('该中文名称已存在');
            }
            
            $data['addtime']=time();
            
			$id = DB::name('report_classify')->insertGetId($data);
            if(!$id){
                $this->error("添加失败！");
            }
			
			$action="添加直播举报类型：{$id}";
			setAdminLog($action);
            
            $this->success("添加成功！");
		}			
	}

    function edit(){
        $id  = $this->request->param('id', 0, 'intval');
        
        $data=Db::name('report_classify')
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
            
            $data['name']=trim($data['name']);
            $data['name_en']=trim($data['name_en']);
			$name=$data['name'];
            $name_en=$data['name_en'];
			$id=$data['id'];

			if($name==""){
				$this->error("中文名称不能为空");
			}
            
            $isexit=DB::name('report_classify')->where([['id','<>',$id],['name','=',$name]])->find();	
            if($isexit){
                $this->error('该中文名称已存在');
            }

            if($name_en==""){
                $this->error("英文名称不能为空");
            }
            
            $isexit=DB::name('report_classify')->where([['id','<>',$id],['name_en','=',$name_en]])->find();   
            if($isexit){
                $this->error('该英文名称已存在');
            }
            
			$rs = DB::name('report_classify')->update($data);
            if($rs===false){
                $this->error("修改失败！");
            }
			
			$action="修改直播举报类型：{$id}";
			setAdminLog($action);
            
            $this->success("修改成功！");
		}	
	}
}
