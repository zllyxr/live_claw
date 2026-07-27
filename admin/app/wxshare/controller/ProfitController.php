<?php
/**
 * 我的收益
 */
namespace app\wxshare\controller;

use app\common\controller\HomeBaseController;
use think\facade\Db;

class ProfitController extends HomebaseController {

	function index(){       
		$uid=session('uid');
		if(!$uid){
			return $this->redirect('/wxshare/user/login');
		}

		
		$userinfo=getUserPrivateInfo($uid);
		$this->assign('userinfo',$userinfo);
		
		$configpri=getConfigPri();
		$this->assign('configpri',$configpri);
		

		// 星币提现固定 1:1，不再使用第二套比例。
		$cash_rate=1;
		$cash_start=$configpri['cash_start'];
		$cash_end=$configpri['cash_end'];
		$cash_max_times=$configpri['cash_max_times'];
		$cash_take=0;
		
		// 统一钱包后，coin 为唯一可提现/消费余额。
		$votes=$userinfo['coin'];
		$votestotal=$userinfo['votestotal'];
			
		//总可提现星币
		$total=floor($votes);
        if($cash_max_times){
            $tips='每月'.$cash_start.'-'.$cash_end.'号可进行提现申请，每月只可提现'.$cash_max_times.'次';
        }else{
            $tips='每月'.$cash_start.'-'.$cash_end.'号可进行提现申请';
        }
		$rs=array(
			"votes"=>$votes,
			"votestotal"=>$votestotal,
			"todaycash"=>$votes,
			"total"=>$total,
			"cash_rate"=>$cash_rate,
			"cash_take"=>$cash_take,
			"tips"=>$tips,
		);
		$zlist=Db::name('cash_account')->where("uid={$uid}")
                ->order("id desc")
                ->select()
                ->toArray();
		$type=array(
			'1'=>"支付宝",
			'2'=>"微信",
			'3'=>"银行卡",
		);
		foreach($zlist as $k=>$v){
			$zlist[$k]['type_account']=$type[$v['type']]."-".$v['account'];
		}
	
		

	 	$this->assign("zlist",$zlist);
	 	$this->assign("rs",$rs);
		
		
		
		return $this->fetch();
	    
	}
	
	
	/*提现请求*/
	public function edit_exchange(){
		$rs=['code'=>0,'msg'=>'提现成功','data'=>[]];
		
		$uid=(int)session("uid");
		if(!$uid){
			$rs['code']='700';
			$rs['msg']='您的登陆状态失效，请重新登陆！';
			echo json_encode($rs);
			exit;
		}

		//判断用户的状态
        $userinfo=getUserInfo($uid);
        if($userinfo['user_status']==0){
            session('uid',null);        
            session('token',null);
            session('user',null);
			$rs['code']='1001';
			$rs['msg']='该账号已被拉黑';
			echo json_encode($rs);
			exit;
        }

        if($userinfo['end_bantime']>time()){
            session('uid',null);        
            session('token',null);
            session('user',null);
			$rs['code']='1001';
			$rs['msg']='该账号已被禁用';
			echo json_encode($rs);
			exit;
        }

		
        $where['uid']=$uid;
        
		$isrz=Db::name("user_auth")->field("status")->where($where)->find();
		if(!$isrz || $isrz['status']!=1){
			
			$rs['code']='1001';
			$rs['msg']='请先进行身份认证';
			echo json_encode($rs);
			exit;
	
		}
		
		$info=getUserPrivateInfo($uid);	
		
		$nowtime=time();
        $data = $this->request->param();
        $accountid= $data['accountid'] ?? '';
        $accountid=(int)checkNull($accountid);
        
        $cashvote= $data['cashvote'] ?? '';
        $cashvote=(int)checkNull($cashvote);
        
        
        if($accountid <1 || $cashvote<=0){
					
			$rs['code']='1001';
			$rs['msg']='信息错误';
			echo json_encode($rs);
			exit;
        }
        
        $config=getConfigPri();
        $cash_start=$config['cash_start'];
        $cash_end=$config['cash_end'];
        $cash_max_times=$config['cash_max_times'];
        
        $day=(int)date("d",$nowtime);
        if($day < $cash_start || $day > $cash_end){
			$rs['code']='1001';
			$rs['msg']='不在提现期限内，不能提现';
			echo json_encode($rs);
			exit;
        }
        
        //本月第一天
        $month=date('Y-m-d',strtotime(date("Ym",$nowtime).'01'));
        $month_start=strtotime(date("Ym",$nowtime).'01');

        //本月最后一天
        $month_end=strtotime("{$month} +1 month");      

        if($cash_max_times){
            $isexist=Db::name('cash_record')
                    ->where("uid={$uid} and addtime > {$month_start} and addtime < {$month_end}")
                    ->count();
            if($isexist > $cash_max_times){
				$rs['code']='1001';
				$rs['msg']="每月只可提现{$cash_max_times}次,已达上限";
				echo json_encode($rs);
				exit;
			
            }   
        }

		
		// 钱包信息
		$accountinfo=Db::name('cash_account')
				->where("id={$accountid} and uid={$uid}")
				->find();
        if(!$accountinfo){
			$rs['code']='1001';
			$rs['msg']="该钱包不存在";
			echo json_encode($rs);
			exit;

        }
		
		$votes=$info['coin'];
        
        if($cashvote > $votes){
			
			$rs['code']='1001';
			$rs['msg']="余额不足";
			echo json_encode($rs);
			exit;
        }
        

		// 最低额度
		$cash_min=$config['cash_min'];
		
		// 星币 1:1 提现
        $money=$cashvote;
		
		if($money < $cash_min){
			$rs['code']='1001';
			$rs['msg']="提现最低额度为{$cash_min}星币";
			echo json_encode($rs);
			exit;
			

		}
		
		$cashvotes=$money;
        $ifok=Db::name("user")
            ->where([['id','=',$uid],['coin','>=',$cashvotes]])
            ->dec('coin',$cashvotes)
            ->update();
        if(!$ifok){
			$rs['code']='1001';
			$rs['msg']="余额不足";
			echo json_encode($rs);
			exit;
        }

		$data=array(
			"uid"=>$uid,
			"money"=>$money,
			"votes"=>$cashvote,
			"orderno"=>$uid.'_'.$nowtime.rand(100,999),
			"status"=>0,
			"addtime"=>$nowtime,
			"uptime"=>$nowtime,
			"type"=>$accountinfo['type'],
			"account_bank"=>$accountinfo['account_bank'],
			"account"=>$accountinfo['account'],
			"name"=>$accountinfo['name'],
		);
		
		$add=Db::name("cash_record")->insert($data);
		if(!$add){
			$rs['code']='1001';
			$rs['msg']="提现失败，请重试";
			echo json_encode($rs);
			exit;

		}
			
		echo json_encode($rs);
		exit;
	}
	protected function getStatus($k=''){
        $status=array(
            '0'=>'审核中',
            '1'=>'成功',
            '2'=>'失败',
        );
        if($k===''){
            return $status;
        }
		return $status[$k];
    } 
	
	public function cash_list(){
		$uid=(int)session("uid");
		if(!$uid){
			return $this->redirect('/wxshare/user/login');
		}

		//判断用户的状态
        $userinfo=getUserInfo($uid);
         if($userinfo['user_status']==0){
            session('uid',null);        
            session('token',null);
            session('user',null);
			$rs['code']='1001';
			$rs['msg']='该账号已被拉黑';
			echo json_encode($rs);
			exit;
        }

        if($userinfo['end_bantime']>time()){
            session('uid',null);        
            session('token',null);
            session('user',null);
			$rs['code']='1001';
			$rs['msg']='该账号已被禁用';
			echo json_encode($rs);
			exit;
        }
		
		$info=getUserPrivateInfo($uid);	
		$where=[];
		$where['uid']=$uid;
		$list=Db::name("cash_record")
			->where($where)
			->order("id desc")
			->limit(0,50)
            ->select()
            ->toArray(); 
        foreach($list as $k=>$v){
			$v['addtime']=date('Y.m.d',$v['addtime']);
            $v['status_name']=$this->getStatus($v['status']);
            $list[$k]=$v; 
		}
            


    	$this->assign('list', $list);
		$this->assign("info",$info);
		
		
		$configpri=getConfigPri();
		$this->assign('configpri',$configpri);

		return $this->fetch();
	}
	
	public function cash_list_more()
	{
		$data = $this->request->param();
        $uid= $data['uid'] ?? '';
        $p=$data['page'] ?? '1';
        $uid=(int)checkNull($uid);
        $p=checkNull($p);
		$result=array(
			'data'=>array(),
			'nums'=>0,
			'isscroll'=>0,
		);
		
		$pnums=50;
		$start=($p-1)*$pnums;
		
		$where=[];
		$where['uid']=$uid;
		$list=Db::name("cash_record")
			->where($where)
			->order("id desc")
            ->limit($start,$pnums)
            ->select()
            ->toArray();
		foreach($list as $k=>$v){
			$v['addtime']=date('Y.m.d',$v['addtime']);
            $v['status_name']=$this->getStatus($v['status']);
            $list[$k]=$v; 
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
