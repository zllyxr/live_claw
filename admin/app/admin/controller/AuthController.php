<?php

/**
 * 认证
 */
namespace app\admin\controller;

use cmf\controller\AdminBaseController;
use think\facade\Db;

class AuthController extends AdminBaseController {
    
    protected function getStatus($k=''){
        $status=array(
            '0'=>'处理中',
            '1'=>'审核成功',
            '2'=>'审核失败',
        );
        if($k===''){
            return $status;
        }
        
        return $status[$k] ?? '';
    }

    /**
     * @desc 认证列表
     * @return mixed
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
        
        $status= $data['status'] ?? '';
        if($status!=''){
            $map[]=['status','=',$status];
        }

        $uid=$data['uid'] ?? '';
        if($uid!=''){
            $lianguid=getLianguser($uid);

            if($lianguid){
                
                array_push($lianguid,$uid);
                $map[]=['uid','in',$lianguid];
            }else{
                $map[]=['uid','=',$uid];
            }
        }
        
        $keyword= $data['keyword'] ?? '';
        if($keyword!=''){
            $map[]=['real_name|mobile','like','%'.$keyword.'%'];
        }

    	$lists = Db::name("user_auth")
                ->where($map)
                ->order("addtime DESC")
                ->paginate(20);
        
        $lists->each(function($v,$k){
			$v['userinfo']=getUserInfo($v['uid']);
			$v['mobile']=m_s($v['mobile']);
			$v['cer_no']=m_s($v['cer_no']);
            return $v;           
        });
        
        $lists->appends($data);
        $page = $lists->render();

    	$this->assign('lists', $lists);
    	$this->assign("page", $page);
        $this->assign('status', $this->getStatus());
    	return $this->fetch();
    }
    
	function del(){
        
        $uid = $this->request->param('uid', 0, 'intval');
        
        $rs = DB::name('user_auth')->where("uid={$uid}")->delete();
        if(!$rs){
            $this->error("删除失败！");
        }
        
        $action="删除会员认证信息：{$uid}";
        setAdminLog($action);
        $this->success("删除成功！");
	}
    
    function edit(){
        
        $uid   = $this->request->param('uid', 0, 'intval');
        $data=Db::name('user_auth')
            ->where("uid={$uid}")
            ->find();
        if(!$data){
            $this->error("信息错误");
        }
        
        $data['userinfo']=getUserInfo($data['uid']);
        $data['mobile']=m_s($data['mobile']);
        $data['cer_no']=m_s($data['cer_no']);

        $status=$this->getStatus();
        if($data['status']!=0){ //已经处理过的不显示处理中
            unset($status[0]);
        }
        
        $this->assign('status', $status);
        $this->assign('data', $data);
        return $this->fetch();
	}
	function editPost(){
		if ($this->request->isPost()) {
            
            $data = $this->request->param();
            
			$status=$data['status'];
			$uid=$data['uid'];
            $reason=$data['reason'];
            $reason_en=$data['reason_en'];

			if($status=='0'){
				$this->success("修改成功！");
			}

            $data['uptime']=time();
            
			$rs = DB::name('user_auth')->update($data);
            if($rs===false){
                $this->error("修改失败！");
            }
            
            if($status=='2'){
                $action="修改会员认证信息：{$uid} - 拒绝";
                //发送系统消息
                $title="你的身份认证未通过";
                $title_en="Your identity authentication failed";

                if($reason){
                    $title.=",拒绝原因：".$reason;
                }

                if($reason_en){
                    $title_en.=",Denial Reason:".$reason_en;
                }

                addSysytemInfo($uid,$title,$title_en,2);
                //发送腾讯云推送
                //type=2 非直播开播消息
                


                txMessageTpns('身份认证',$title,'single',$uid,[],json_encode(['type'=>2]),'zh-cn');
                sleep(2);
                txMessageTpns('Authentication',$title_en,'single',$uid,[],json_encode(['type'=>2]),'en');

            }else if($status=='1'){

                $action="修改会员认证信息：{$uid} - 同意";
                //发送系统消息
                $title="你的身份认证通过";
                $title_en="Your identity verification passed";
                addSysytemInfo($uid,$title,$title_en,2);
                //发送腾讯云推送
                //type=2 非直播开播消息
                

                txMessageTpns('身份认证',$title,'single',$uid,[],json_encode(['type'=>2]),'zh-cn');
                sleep(2);
                txMessageTpns('Authentication',$title_en,'single',$uid,[],json_encode(['type'=>2]),'en');
            }
            
            setAdminLog($action);
            
            $this->success("修改成功！");
		}
	}
	
    
}
