<?php

/**
 * 禁播列表
 */
namespace app\admin\controller;

use cmf\controller\AdminBaseController;
use think\facade\Db;

class LivebanController extends AdminBaseController {

    /**
     * @desc 禁播列表
     * @return mixed
     * @throws \think\db\exception\DbException
     */
    function index(){
        $data = $this->request->param();
        $map=[];
        
        $start_time= $data['start_time'] ?? '';
        $end_time= $data['end_time'] ?? '';
        
        if($start_time!=""){
           $map[]=['addtime','>=',strtotime($start_time)];
        }

        if($end_time!=""){
           $map[]=['addtime','<=',strtotime($end_time) + 60*60*24];
        }
        
        $uid= $data['uid'] ?? '';
        if($uid!=''){
            $lianguid=getLianguser($uid);
            if($lianguid){
                
                array_push($lianguid,$uid);
                $map[]=['liveuid','in',$lianguid];
            }else{
                $map[]=['liveuid','=',$uid];
            }
        }
		
        
    	$lists = Db::name("live_ban")
            ->where($map)
            ->order("addtime DESC")
            ->paginate(20);

        $ban_rules = getLiveBanRules();
        
        $lists->each(function($v,$k) use ($ban_rules){
			$v['liveinfo']=getUserInfo($v['liveuid']);
			$v['superinfo']=getUserInfo($v['superid']);

            if($v['type']=='all'){
                $v['ban_time']='永久封禁';
            }else{

                foreach ($ban_rules as $k1 => $v1) {
                    if($v1['type']==$v['type']){
                        $v['ban_time']=$v1['name'];
                        break;
                    }
                }

            }

            return $v;           
        });
            
        $lists->appends($data);
        $page = $lists->render();

    	$this->assign('lists', $lists);
    	$this->assign("page", $page);
    	
    	return $this->fetch();
    }

    /**
     * @desc 删除禁播记录
     * @return void
     * @throws \think\db\exception\DbException
     */
    function del(){
        $id = $this->request->param('id', 0, 'intval');

        $rs = DB::name('live_ban')->where("liveuid={$id}")->delete();
        if(!$rs){
            $this->error("删除失败！");
        }
        $action="直播管理-禁播管理删除主播ID：{$id}";
		setAdminLog($action);
        $this->success("删除成功！");
    }


    
}
