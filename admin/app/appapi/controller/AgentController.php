<?php
/**
 * 分销
 */
namespace app\appapi\controller;

use app\common\controller\HomeBaseController;
use think\facade\Db;

class AgentController extends HomebaseController {
	
	function index(){       
		$data = $this->request->param();
        $uid= $data['uid'] ?? '';
        $token=$data['token'] ?? '';
        $uid=(int)checkNull($uid);
        $token=checkNull($token);
        
        $checkToken=checkToken($uid,$token);
		if($checkToken==700){
			$reason=lang('您的登陆状态失效，请重新登陆！');
			$this->assign('reason', $reason);
			return $this->fetch(':error');
		}
		  
		$nowtime=time();

		$userinfo=getUserInfo($uid);
		$code=Db::name('agent_code')->where(["uid"=>$uid])->value('code');
		
		if(!$code){
			$code=createCode();
            $ifok=Db::name('agent_code')->where(["uid"=>$uid])->update(array("code"=>$code));
            if(!$ifok){
                Db::name('agent_code')->insert(array('uid'=>$uid,"code"=>$code));
            }
			
		}

		$code_a=str_split($code);

		$this->assign("code",$code);
		$this->assign("code_a",$code_a);
		$agentinfo=array();
        
        /* 是否是分销下级 */
        $users_agent=Db::name("agent")->where(["uid"=>$uid])->find();
		if($users_agent){
			$agentinfo= getUserInfo($users_agent['one_uid']);
		}

        $one_profit=0;
		
		$agentprofit=Db::name("agent_profit")->where(["uid"=>$uid])->find();
        if($agentprofit){
            $one_profit=$agentprofit['one_profit'];
        }

		$agnet_profit=array(
			'one_profit'=>number_format($one_profit),
		);

		$this->assign("uid",$uid);
		$this->assign("token",$token);
		$this->assign("userinfo",$userinfo);
		$this->assign("agentinfo",$agentinfo);
		$this->assign("agnet_profit",$agnet_profit);

		return $this->fetch();
	    
	}
	
	function agent(){
		$data = $this->request->param();
        $uid= $data['uid'] ?? '';
        $token=$data['token'] ?? '';
        $uid=(int)checkNull($uid);
        $token=checkNull($token);
        
        $checkToken=checkToken($uid,$token);
		if($checkToken==700){
			$reason=lang('您的登陆状态失效，请重新登陆！');
			$this->assign('reason', $reason);
			return $this->fetch(':error');
		}
		
		$agentinfo=array();
		
		$users_agent=Db::name('agent')->where(["uid"=>$uid])->find();
		if($users_agent){
			$agentinfo=getUserInfo($users_agent['one_uid']);
			
			$code=Db::name('agent_code')->where("uid={$users_agent['one_uid']}")->value('code');
			
			$agentinfo['code']=$code;
			$code_a=str_split($code);

			$this->assign("code_a",$code_a);
		}
	
		
		$this->assign("uid",$uid);
		$this->assign("token",$token);

		$this->assign("agentinfo",$agentinfo);

		return $this->fetch();
	}
	
	function setAgent(){
		$data = $this->request->param();
        $uid= $data['uid'] ?? '';
        $token= $data['token'] ?? '';
        $code= $data['code'] ?? '';
        $uid=(int)checkNull($uid);
        $token=checkNull($token);
        $code=checkNull($code);
		
		$rs=array('code'=>0,'info'=>array(),'msg'=>lang('设置成功'));
		
		if(checkToken($uid,$token)==700){
			$rs['code']=700;
			$rs['msg']=lang('您的登陆状态失效，请重新登陆！');
			echo json_encode($rs);
            return;
		} 

		if($code==""){
			$rs['code']=1001;
			$rs['msg']=lang('邀请码不能为空');
			echo json_encode($rs);
            return;
		}
		
		$isexist=Db::name('agent')->where(["uid"=>$uid])->find();
		if($isexist){
			$rs['code']=1001;
			$rs['msg']=lang('已设置');
			echo json_encode($rs);
            return;
		}
		
		$oneinfo=Db::name('agent_code')->field("uid")->where(["code"=>$code])->find();
		if(!$oneinfo){
			$rs['code']=1002;
			$rs['msg']=lang('邀请码错误');
			echo json_encode($rs);
            return;
		}
		
		if($oneinfo['uid']==$uid){
			$rs['code']=1003;
			$rs['msg']=lang('不能填写自己的邀请码');
			echo json_encode($rs);
            return;
		}
		
		$one_agent=Db::name('agent')->where("uid={$oneinfo['uid']}")->find();
		if(!$one_agent){
			$one_agent=array(
				'uid'=>$oneinfo['uid'],
				'one_uid'=>0,
			);
		}else{

			if($one_agent['one_uid']==$uid){
				$rs['code']=1004;
				$rs['msg']=lang('您已经是该用户的上级');
				echo json_encode($rs);
                return;
			}
		}
		
		$data=array(
			'uid'=>$uid,
			'one_uid'=>$one_agent['uid'],
			'addtime'=>time(),
		);
		Db::name('agent')->insert($data);

		echo json_encode($rs);
		
	}

	function quit(){
        $rs=array('code'=>0,'msg'=>'','info'=>array());
		$data = $this->request->param();
        $uid= $data['uid'] ?? '';
        $token=$data['token'] ?? '';
        $uid=(int)checkNull($uid);
        $token=checkNull($token);
        
        $checkToken=checkToken($uid,$token);
		if($checkToken==700){
			$reason=lang('您的登陆状态失效，请重新登陆！');
			$this->assign('reason', $reason);
			return $this->fetch(':error');
		}
		
		$isexist=Db::name('agent')->where(["uid"=>$uid])->delete();

		echo json_encode($rs);
		
	}
	
	function one(){
		$data = $this->request->param();
        $uid= $data['uid'] ?? '';
        $token= $data['token'] ?? '';
        $uid=(int)checkNull($uid);
        $token=checkNull($token);
		
		if(checkToken($uid,$token)==700){
			$this->assign("reason",lang('您的登陆状态失效，请重新登陆！'));
			return $this->fetch(':error');
		}
		
		$list=Db::name('agent_profit_recode')
            ->field("uid,sum(one_profit) as total")
            ->where(["one_uid"=>$uid])->group("uid")
            ->order("addtime desc")
            ->limit(0,50)
            ->select()
            ->toArray();
		foreach($list as $k=>$v){
			$list[$k]['userinfo']=getUserInfo($v['uid']);
			$list[$k]['total']=NumberFormat($v['total']);
		}
		$this->assign("uid",$uid);
		$this->assign("token",$token);
		$this->assign("list",$list);
		return $this->fetch();
	}

	function one_more(){
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
		
		$list=Db::name('agent_profit_recode')
            ->field("uid,sum(one_profit) as total")
            ->where(["one_uid"=>$uid])
            ->group("uid")
            ->order("addtime desc")
            ->limit($start,$pnums)
            ->select()
            ->toArray();
		foreach($list as $k=>$v){
			$list[$k]['userinfo']=getUserInfo($v['uid']);
			$list[$k]['total']=NumberFormat($v['total']);
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

		//短邀请链接：/invite?ref=CODE
		public function invite(){
			$data=$this->request->param();
			$code=$data['ref'] ?? ($data['code'] ?? '');
			return $this->renderInviteDownload($code);
		}

		//扫描app生成的分享二维码显示的下载页面，记录点击；有 OpenInstall 时继续传递安装参数
		public function downapp(){
			$data=$this->request->param();
			$code=$data['code'] ?? ($data['ref'] ?? '');
			return $this->renderInviteDownload($code);
		}

		private function renderInviteDownload($code){
			$code=$this->normalizeInviteCode($code);
			if($code===''){
				$this->assign("reason",lang('邀请码错误'));
				return $this->fetch(':error');
			}

			$code_info=Db::name("agent_code")->where(["code"=>$code])->find();
			if(!$code_info){
				$this->assign("reason",lang('邀请码不存在'));
				return $this->fetch(':error');
			}

			$configpub=getConfigPub();
			$configpri=getConfigPri();
			$site_name=$configpub['site_name'] ?? '';
			$openinstall_switch=$configpri['openinstall_switch'] ?? 0;
			$openinstall_appkey=($openinstall_switch && !empty($configpri['openinstall_appkey'])) ? $configpri['openinstall_appkey'] : '';
			$download_url=$this->getDownloadUrl($configpub);
			$click_id=$this->recordInviteClick($code, $code_info['uid'], $download_url);

			cookie('invite_click_id',$click_id,7*24*3600);
			cookie('invite_ref',$code,7*24*3600);

			$this->assign("site_name",$site_name);
			$this->assign("openinstall_appkey",$openinstall_appkey);
			$this->assign("invite_ref",$code);
			$this->assign("click_id",$click_id);
			$this->assign("download_url_android",$configpub['apk_url'] ?? '');
			$this->assign("download_url_ios",$configpub['ipa_url'] ?? '');
			$this->assign("download_url",$download_url);
			return $this->fetch('downapp');
		}

		private function recordInviteClick($code,$inviter_uid,$download_url){
			$now=time();
			$click_id=$this->makeClickId();
			$ip=$this->getClientIp();
			$ua=$_SERVER['HTTP_USER_AGENT'] ?? '';
			$referer=$_SERVER['HTTP_REFERER'] ?? '';
			$landing_url=$this->request->url(true);
			$platform=$this->detectPlatform($ua);

			Db::name('invite_click')->insert([
				'click_id'=>$click_id,
				'ref_code'=>$code,
				'inviter_uid'=>(int)$inviter_uid,
				'platform'=>$platform,
				'ip'=>substr($ip,0,45),
				'ip_hash'=>$ip!=='' ? sha1($ip) : '',
				'user_agent'=>substr($ua,0,512),
				'ua_hash'=>$ua!=='' ? sha1($ua) : '',
				'device_fingerprint'=>'',
				'referer'=>substr($referer,0,512),
				'landing_url'=>substr($landing_url,0,512),
				'download_url'=>substr($download_url,0,512),
				'status'=>0,
				'matched_uid'=>0,
				'created_at'=>$now,
				'updated_at'=>$now,
				'expires_at'=>$now+7*24*3600,
			]);

			return $click_id;
		}

		private function normalizeInviteCode($code){
			$code=strtoupper(trim((string)checkNull($code)));
			return preg_replace('/[^A-Z0-9]/','',$code);
		}

		private function makeClickId(){
			try{
				return bin2hex(random_bytes(16));
			}catch(\Exception $e){
				return sha1(uniqid('',true).mt_rand(100000,999999));
			}
		}

		private function getClientIp(){
			$headers=['HTTP_X_FORWARDED_FOR','HTTP_X_REAL_IP','REMOTE_ADDR'];
			foreach($headers as $header){
				if(empty($_SERVER[$header])){
					continue;
				}
				$value=$_SERVER[$header];
				if($header==='HTTP_X_FORWARDED_FOR'){
					$value=explode(',',$value)[0];
				}
				$value=trim($value);
				if($value!==''){
					return $value;
				}
			}
			return '';
		}

		private function detectPlatform($ua){
			$ua=strtolower((string)$ua);
			if(strpos($ua,'iphone')!==false || strpos($ua,'ipad')!==false || strpos($ua,'ios')!==false){
				return 'ios';
			}
			if(strpos($ua,'android')!==false){
				return 'android';
			}
			return 'web';
		}

		private function getDownloadUrl($configpub){
			$platform=$this->detectPlatform($_SERVER['HTTP_USER_AGENT'] ?? '');
			if($platform==='ios' && !empty($configpub['ipa_url'])){
				return $configpub['ipa_url'];
			}
			if($platform==='android' && !empty($configpub['apk_url'])){
				return $configpub['apk_url'];
			}
			if(!empty($configpub['apk_url'])){
				return $configpub['apk_url'];
			}
			if(!empty($configpub['ipa_url'])){
				return $configpub['ipa_url'];
			}
			return '/appapi/down/index';
		}

	}
