<?php
/**
 * 会员等级
 */
namespace app\wxshare\controller;

use app\common\controller\HomeBaseController;
use think\facade\Db;

class LevelController extends HomebaseController {
	
	function index(){       
		$uid=session('uid');
		if(!$uid){
			return $this->redirect('/wxshare/user/login');
		}

		
		$userinfo=Db::name('user')->field("avatar,consumption,votestotal")->where(["id"=>$uid])->find();
        
        $userinfo['avatar']=get_upload_path($userinfo['avatar']);
        
		$this->assign("userinfo",$userinfo);
		/* 用户等级 */
		
		$levelinfo=Db::name("level")
            ->where("level_up>='{$userinfo['consumption']}'")
            ->order("levelid asc")
            ->find();
		if(!$levelinfo){
			$levelinfo=Db::name("level")->order("levelid desc")->find();
		}
		$cha=$levelinfo['level_up']+1-$userinfo['consumption'];
		if($cha>0)
		{
            if($levelinfo['level_up']>0){
                $baifen=floor($userinfo['consumption']/$levelinfo['level_up']*100);
            }else{
                $baifen='0';
            }
            
			
			$type="1";
		}else{
			$baifen=100;
			$type="0";
		}

		$this->assign("baifen",$baifen);
		$this->assign("levelinfo",$levelinfo);
		$this->assign("cha",$cha);
		$this->assign("type",$type);
		
		/* 主播等价 */
		$levelinfo_a=Db::name("level_anchor")
            ->where("level_up>='{$userinfo['votestotal']}'")
            ->order("levelid asc")
            ->find();
		if(!$levelinfo_a){
			$levelinfo_a=Db::name("level_anchor")->order("levelid desc")->find();
		}
		$cha_a=$levelinfo_a['level_up']+1-$userinfo['votestotal'];
		if($cha_a>0)
		{
            if($levelinfo_a['level_up']>0){
                $baifen_a=floor($userinfo['votestotal']/$levelinfo_a['level_up']*100);
            }else{
                $baifen_a='0';
            }
			
			$type_a="1";
		}else{
			$baifen_a=100;
			$type_a="0";
		}

		$this->assign("cha_a",$cha_a);
		$this->assign("type_a",$type_a);
		$this->assign("baifen_a",$baifen_a);
		$this->assign("levelinfo_a",$levelinfo_a);
		
		return $this->fetch();
	    
	}
	
	function level(){
		$list=Db::name("level")->order("levelid asc")->select()->toArray();
		foreach($list as $k=>$v){
			$list[$k]['level_up']=number_format($v['level_up']);
			$list[$k]['thumb']=get_upload_path($v['thumb']);
		}
		$this->assign("list",$list);
		return $this->fetch();
	}

	function level_a(){
		$list=Db::name("level_anchor")->order("levelid asc")->select()->toArray();
		foreach($list as $k=>$v){
			$list[$k]['level_up']=number_format($v['level_up']);
			$list[$k]['thumb']=get_upload_path($v['thumb']);
		}
		$this->assign("list",$list);
		return $this->fetch();
	}
}