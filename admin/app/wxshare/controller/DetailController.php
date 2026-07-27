<?php
/**
 * 我的明细
 */
namespace app\wxshare\controller;

use app\common\controller\HomeBaseController;
use think\facade\Db;

class DetailController extends HomebaseController {

	function index(){       
		$uid=session('uid');
		$token=session('token');
		if(checkToken($uid,$token)==700){
			return $this->redirect('/wxshare/user/login');
		}

		
		$list=Db::name('user_voterecord')->field("fromid,actionid,sum(nums) as num,sum(total) as totalall")->where(["action"=>'1',"uid"=>$uid])->group("fromid,actionid,showid")->order("addtime desc")->limit(0,50)->select()->toArray();
		foreach($list as $k=>$v){
			$giftinfo=Db::name('gift')->field("giftname")->where("id={$v['actionid']}")->find();
			if(!$giftinfo){
				$giftinfo=array(
					"giftname"=>'礼物已删除'
				);
			}
			$list[$k]['giftinfo']=$giftinfo;
			$list[$k]['totalall']=number_format($v['totalall']);
			$userinfo=getUserInfo($v['fromid']);
			if(!$userinfo){
				$userinfo=array(
					"user_nickname"=>'用户已删除'
				);
			}
			$list[$k]['userinfo']=$userinfo;
		}
		
		$this->assign("list",$list);
		
		$list_live=Db::name('live_record')
            ->field("starttime,endtime")
            ->where(["uid"=>$uid])
            ->order("starttime desc")
            ->limit(0,50)
            ->select()
            ->toArray();
		foreach($list_live as $k=>$v){
            
			$cha=$v['endtime']-$v['starttime'];
			$list_live[$k]['length']=getSeconds($cha,1);
            $list_live[$k]['starttime']=date("Y-m-d H:i",$v['starttime']);
			$list_live[$k]['endtime']=date("Y-m-d H:i",$v['endtime']);
		}

		$this->assign("list_live",$list_live);
		$this->assign("uid",$uid);
		$this->assign("token",$token);
		
		return $this->fetch();
	    
	}
	
	public function receive_more()
	{
		$data = $this->request->param();
        $uid= $data['uid'] ?? '';
        $token=$data['token'] ?? '';
        $p=$data['page'] ?? '1';
        $uid=(int)checkNull($uid);
        $token=checkNull($token);
        $p=checkNull($p);
		
		$result=array(
			'data'=>array(),
			'nums'=>0,
			'isscroll'=>0,
		);
	
		if(checkToken($uid,$token)==700){
			echo json_encode($result);
            return;
		} 
		
		$pnums=50;
		$start=($p-1)*$pnums;
		
		$list=Db::name('user_voterecord')
            ->field("fromid,actionid,sum(nums) as num,sum(total) as totalall")
            ->where(["action"=>'1',"uid"=>$uid])
            ->group("fromid,actionid,showid")
            ->order("addtime desc")
            ->limit($start,$pnums)
            ->select()
            ->toArray();
		foreach($list as $k=>$v){
			$giftinfo=Db::name('gift')
                ->field("giftname")
                ->where("id={$v['actionid']}")
                ->find();
			if(!$giftinfo){
				$giftinfo=array(
					"giftname"=>'礼物已删除'
				);
			}
			$list[$k]['giftinfo']=$giftinfo;
            $list[$k]['totalall']=number_format($v['totalall']);
			$userinfo=getUserInfo($v['fromid']);
			if(!$userinfo){
				$userinfo=array(
					"user_nickname"=>'用户已删除'
				);
			}
			$list[$k]['userinfo']=$userinfo;
		}
		
		$nums=count($list);
		if($nums<$pnums){
			$isscroll=0;
		}else{
			$isscroll=1;
		}
		
		$result=array(
			'data'=>$list,
			'nums'=>$nums,
			'isscroll'=>$isscroll,
		);

		echo json_encode($result);
		
	}
	
	public function liverecord_more()
	{
		$data = $this->request->param();
        $uid= $data['uid'] ?? '';
        $token=$data['token'] ?? '';
        $p=$data['page'] ?? '1';
        $uid=(int)checkNull($uid);
        $token=checkNull($token);
        $p=checkNull($p);
		
		$result=array(
			'data'=>array(),
			'nums'=>0,
			'isscroll'=>0,
		);
	
		if(checkToken($uid,$token)==700){
			echo json_encode($result);
            return;
		} 
		
		$pnums=50;
		$start=($p-1)*$pnums;
		
		$list=Db::name('live_record')
            ->field("starttime,endtime")
            ->where(["uid"=>$uid])
            ->order("starttime desc")
            ->limit($start,$pnums)
            ->select()
            ->toArray();
		foreach($list as $k=>$v){
			$list[$k]['starttime']=date("Y-m-d H:i",$v['starttime']);
			$list[$k]['endtime']=date("Y-m-d H:i",$v['endtime']);
			$cha=$v['endtime']-$v['starttime'];
			$list[$k]['length']=getSeconds($cha,1);
		}

		$nums=count($list);
		if($nums<$pnums){
			$isscroll=0;
		}else{
			$isscroll=1;
		}
		
		$result=array(
			'data'=>$list,
			'nums'=>$nums,
			'isscroll'=>$isscroll,
		);

		echo json_encode($result);
		
	}
	

}