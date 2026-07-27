<?php

/**
 * 消费记录
 */
namespace app\admin\controller;

use cmf\controller\AdminBaseController;
use think\facade\Db;

class CoinrecordController extends AdminBaseController {
    
    protected function getTypes($k=''){
        $type=array(
            '0'=>'支出',
            '1'=>'收入',
        );
        if($k===''){
            return $type;
        }
        
        return $type[$k] ?? '';
    }
    
    protected function getAction($k=''){
        $action=array(
            '1'=>'赠送礼物',
            '2'=>'弹幕',
            '3'=>'登录奖励',
            '4'=>'购买VIP',
            '5'=>'购买坐骑',
            '6'=>'房间扣费',
            '7'=>'计时扣费',
            '8'=>'发送红包',
            '9'=>'抢红包',
            '10'=>'开通守护',
            '11'=>'注册奖励',
            '12'=>'礼物中奖',
            '13'=>'奖池中奖',
            '14'=>'缴纳保证金',
            '15'=>'退还保证金',
            '16'=>'转盘游戏',
            '17'=>'转盘中奖',
            '18'=>'购买靓号',
            '19'=>'游戏下注',
            '20'=>'游戏退还',
            '21'=>'每日任务',
            '22'=>'星球探宝下注',
            '23'=>'星球探宝中奖星币',
            '24'=>'幸运大转盘下注',
            '25'=>'幸运大转盘中奖星币',
            '29'=>'体育投注扣款',
            '30'=>'体育投注退款',
            '31'=>'体育投注派彩',
        );
        if($k===''){
            return $action;
        }
        
        return $action[$k] ?? '未知';
    }

    protected function getGame($k=''){
        $game=array(
            '1'=>'智勇三张',
			'2'=>'海盗船长',
			'3'=>'转盘',
			'4'=>'开心牛仔',
			'5'=>'二八贝',
        );
        if($k===''){
            return $game;
        }
        
        return $game[$k] ?? '';
    }
    
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

        $type= $data['type'] ?? '';
        if($type!=''){
            $map[]=['type','=',$type];
        }
        
        $action= $data['action'] ?? '';
        if($action!=''){
            $map[]=['action','=',$action];
        }
        
        $uid= $data['uid'] ?? '';
        if($uid!=''){
            $lianguid=getLianguser($uid);
            if($lianguid){
                
                array_push($lianguid,$uid);
                $map[]=['uid','in',$lianguid];
            }else{
                $map[]=['uid','=',$uid];
            }
        }
        
        $touid= $data['touid'] ?? '';
        if($touid!=''){
            $map[]=['touid','=',$touid];
        }
        
        $lists = Db::name("user_coinrecord")
            ->where($map)
			->order("id desc")
			->paginate(20);
        
        $lists->each(function($v,$k){
			$v['userinfo']=getUserInfo($v['uid']);
			$v['touserinfo']=getUserInfo($v['touid']);
            
            $action=$v['action'];
            if($action=='1'){
                $giftinfo=Db::name("gift")->field("giftname")->where("id='{$v['giftid']}'")->find();
                if(!$giftinfo){
                    $giftinfo['giftname']='礼物已删除';
                }
            }else if($action=='3'){
                $giftinfo['giftname']='第'.$v['giftid'].'天';
            }else if($action=='4'){
                $info=Db::name("vip")->field("name")->where("id='{$v['giftid']}'")->find();
                $giftinfo['giftname']=$info['name'];
            }else if($action=='5'){
                $info=Db::name("car")->field("name")->where("id='{$v['giftid']}'")->find();
                if(!$info){
                    $info['name']='坐骑已删除';
                }
                $giftinfo['giftname']=$info['name'];
            }else if($action=='18'){
                $info=Db::name("liang")->field("name")->where("id='{$v['giftid']}'")->find();
                if(!$info){
                    $info['name']='靓号已删除';
                }
                $giftinfo['giftname']=$info['name'];
            }else if($action=='10'){
                $info=Db::name("guard")->field("name")->where("id='{$v['giftid']}'")->find();
                $giftinfo['giftname']=$info['name'];
            }else if($action=='19' || $action=='20'){
                $info=Db::name("game")->field("action")->where("id='{$v['giftid']}'")->find();
                $giftinfo['giftname']=$this->getGame($info['action']);
            }else{
                $giftinfo['giftname']=$this->getAction($action);
                
            }
            $v['giftinfo']= $giftinfo;
                
            return $v;           
        });
    	
        $lists->appends($data);
        $page = $lists->render();

    	$this->assign('lists', $lists);
    	$this->assign("page", $page);
        $this->assign('action', $this->getAction());
        $this->assign('type', $this->getTypes());
        
    	return $this->fetch();
    
    }
		
    function del(){
        $id = $this->request->param('id', 0, 'intval');
        
        $rs = DB::name('user_coinrecord')->where("id={$id}")->delete();
        if(!$rs){
            $this->error("删除失败！");
        }
                    
        $this->success("删除成功！");
        							  			
    }    

	//星币消费记录
	function export(){

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

        $type= $data['type'] ?? '';
        if($type!=''){
            $map[]=['type','=',$type];
        }

        $action=$data['action'] ?? '';
        if($action!=''){
            $map[]=['action','=',$action];
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
        
        $touid= $data['touid'] ?? '';
        if($touid!=''){
            $map[]=['touid','=',$touid];
        }

        $xlsName  = "星币消费记录";
		$lists = Db::name("user_coinrecord")
            ->where($map)
			->order("id desc")
			->select()
            ->toArray();
        
        if(empty($lists)){
            $this->error("数据为空");
        }
      
		foreach($lists as $k=>$v){
			$userinfo=getUserInfo($v['uid']);
			$v['user_nickname']= $userinfo['user_nickname']."(".$v['uid'].")";
			
			$touserinfo=getUserInfo($v['touid']);
            $v['touser_nickname']= $touserinfo['user_nickname']."(".$v['touid'].")";
			
			
            $action=$v['action'];
            if($action=='1'){
                $giftinfo=Db::name("gift")->field("giftname")->where("id='{$v['giftid']}'")->find();
            }else if($action=='3'){
                $giftinfo['giftname']='第'.$v['giftid'].'天';
            }else if($action=='4'){
                $info=Db::name("vip")->field("name")->where("id='{$v['giftid']}'")->find();
                $giftinfo['giftname']=$info['name'];
            }else if($action=='5'){
                $info=Db::name("car")->field("name")->where("id='{$v['giftid']}'")->find();
                $giftinfo['giftname']=$info['name'];
            }else if($action=='18'){
                $info=Db::name("liang")->field("name")->where("id='{$v['giftid']}'")->find();
                $giftinfo['giftname']=$info['name'];
            }else if($action=='10'){
                $info=Db::name("guard")->field("name")->where("id='{$v['giftid']}'")->find();
                $giftinfo['giftname']=$info['name'];
            }else if($action=='19' || $action=='20'){
                $info=Db::name("game")->field("action")->where("id='{$v['giftid']}'")->find();
                $giftinfo['giftname']=$this->getGame($info['action']);
            }else{
                $giftinfo['giftname']=$this->getAction($action);
                
            }
    
            $v['giftname']= $giftinfo['giftname']."(".$v['giftid'].")";
           
            $v['type']= $this->getTypes($v['type']);
            $v['action']= $this->getAction($v['action']);
			$v['addtime']=date("Y-m-d H:i:s",$v['addtime']); 
             
            $lists[$k]=$v;     

		}

        $action="星币消费记录：".Db::name("user_coinrecord")->getLastSql();
        setAdminLog($action);
        
        $cellName = array('A','B','C','D','E','F','G','H','I','J');
        $xlsCell  = array(
            array('id','序号'),
            array('type','收支类型'),
            array('action','收支行为'),
            array('user_nickname','会员 (ID)'),
            array('touser_nickname','主播 (ID)'),
            array('giftname','行为说明 (ID)'),
            array('giftcount','数量'),
            array('totalcoin','总价'),
            array('showid','直播id'),
            array('addtime','时间')
        );
        exportExcel($xlsName,$xlsCell,$lists,$cellName);
    }
}
