<?php
namespace App\Model;

use PhalApi\Model\NotORMModel as NotORM;


class Login extends NotORM {

	protected $fields='id,user_nickname,avatar,avatar_thumb,sex,signature,coin,consumption,votestotal,province,city,birthday,user_status,end_bantime,login_type,last_login_time,location';

	/* 会员登录 */   	
    public function userLogin($country_code,$user_login,$user_pass) {

		$user_pass=\App\setPass($user_pass);
		
		$info=\PhalApi\DI()->notorm->user
				->select($this->fields.',user_pass')
				->where('country_code=? and user_login=? and user_type="2"',$country_code,$user_login) 
				->fetchOne();

		if(!$info || $info['user_pass'] != $user_pass){
			return 1001;
		}
		unset($info['user_pass']);
        
        if($info['user_status']=='0'){
			return 1003;					
		}
		
        
		if($info['end_bantime']>time()){
			return 1002;					
		}


		if($info['user_status']=='3'){
			return 1004;
		}

		$uid=$info['id'];

		//判断用户是否在语音聊天室上麦
		$voicelive_micinfo=\PhalApi\DI()->notorm->voicelive_mic->where("uid=?",$uid)->fetchOne();
		if($voicelive_micinfo){
			$islive = \PhalApi\DI()->notorm->live->where(['uid'=>$voicelive_micinfo['liveuid'],'islive'=>1])->fetchOne();
			if($islive){
				return 1005;
			}
			
			\PhalApi\DI()->notorm->voicelive_mic->where(['liveuid'=>$voicelive_micinfo['liveuid']])->delete();
		}

		unset($info['user_status']);

		unset($info['end_bantime']);
		
		$info['isreg']='0';
		
        
		if($info['last_login_time']==0){
			$info['isreg']='1';
		}
		
        
        if($info['birthday']){
            $info['birthday']=date('Y-m-d',$info['birthday']);   
        }else{
            $info['birthday']='';
        }
        
		$info['level']=\App\getLevel($info['consumption']);
		$info['level_anchor']=\App\getLevelAnchor($info['votestotal']);

		$token=md5(md5($uid.$user_login.time()));
		
		$info['token']=$token;
		$info['avatar']=\App\get_upload_path($info['avatar']);
		$info['avatar_thumb']=\App\get_upload_path($info['avatar_thumb']);


		$this->updateToken($uid,$token);

		$info['id']=(string)$info['id'];
		$info['sex']=(string)$info['sex'];
		$info['coin']=(string)$info['coin'];
		$info['consumption']=(string)$info['consumption'];
		$info['votestotal']=(string)$info['votestotal'];
        return $info;
    }	
	
	public function getUserban($user_login){
		$userinfo=\PhalApi\DI()->notorm->user
				->select('id,end_bantime')
				->where('user_login=? and user_type="2"',$user_login) 
				->fetchOne();
		 return  $this->baninfo($userinfo['id'],$userinfo['end_bantime']);
	
	}
	public function baninfo($uid,$end_bantime){
		$rs=array("ban_long"=>0,"ban_lon1g"=>0,"ban_reason"=>"","end_bantime"=>0,"ban_tip"=>'');
		$baninfo=\PhalApi\DI()->notorm->user_banrecord
				->select('*')
				->where('uid=? ',$uid) 
				->fetchOne();
		if($baninfo){
			$rs['ban_long']=\App\getBanSeconds($baninfo['ban_long']-time());
			$rs['ban_lon1g']=$baninfo['ban_long'];
			$rs['ban_reason']=$baninfo['ban_reason'];
			$rs['end_bantime']=date("Y-m-d",$end_bantime);
			$rs['ban_tip']=\PhalApi\T("本次封禁时间为{long}，账号将于{time}解除封禁。",['long'=>$rs['ban_long'],'time'=>$rs['end_bantime']]);
		}		
		return $rs;
	}
	public function getThirdUserban($openid,$type){
		
		$userinfo=\PhalApi\DI()->notorm->user
				->select('id,end_bantime')
				  ->where('openid=? and login_type=? ',$openid,$type)
				->fetchOne();
				
		$rs=$this->baninfo($userinfo['id'],$userinfo['end_bantime']);
		return $rs;
	}
	/* 会员注册 */
    public function userReg($country_code,$user_login,$user_pass,$source,$email) {

		$user_pass=\App\setPass($user_pass);
		
		$configpri=\App\getConfigPri();
		$reg_reward=$configpri['reg_reward'];
		$data=array(
			'country_code'=>$country_code,
			'user_login' => $user_login,
			'mobile' =>$user_login,
			'user_email' =>$email,
			'user_nickname' =>\PhalApi\T('手机用户').substr($user_login,-4),
			'user_pass' =>$user_pass,
			'signature' =>\PhalApi\T('这家伙很懒，什么都没留下'),
			'avatar' =>'/default.jpg',
			'avatar_thumb' =>'/default_thumb.jpg',
			'last_login_ip' =>$_SERVER['REMOTE_ADDR'],
			'create_time' => time(),
			'user_status' => 1,
			"user_type"=>2,//会员
			"source"=>$source,
			"coin"=>$reg_reward,
			'bg_img' =>'/default.jpg',
		);

		$isexist=\PhalApi\DI()->notorm->user
				->select('id')
				->where('country_code=? and user_login=?',$country_code,$user_login) 
				->fetchOne();
		if($isexist){
			return 1006;
		}

		$isEmailExist=\PhalApi\DI()->notorm->user
				->select('id')
				->where('user_email=? and user_type="2"',$email)
				->fetchOne();
		if($isEmailExist){
			return 1008;
		}

		$rs=\PhalApi\DI()->notorm->user->insert($data);	
		if(!$rs){
			return 1007;
		}
        $uid=$rs['id'];
        if($reg_reward>0){
            $insert=array("type"=>'1',"action"=>'11',"uid"=>$uid,"touid"=>$uid,"giftid"=>0,"giftcount"=>1,"totalcoin"=>$reg_reward,"showid"=>0,"addtime"=>time() );
            \PhalApi\DI()->notorm->user_coinrecord->insert($insert);
        }
		$code=$this->createCode();
		$code_info=array('uid'=>$uid,'code'=>$code);
		$isexist=\PhalApi\DI()->notorm->agent_code
					->select("*")
					->where('uid = ?',$uid)
					->fetchOne();
		if($isexist){
			\PhalApi\DI()->notorm->agent_code->where('uid = ?',$uid)->update($code_info);	
		}else{
			\PhalApi\DI()->notorm->agent_code->insert($code_info);	
		}
		return 1;
    }	

	/* 找回密码 */
	public function userFindPass($country_code,$user_login,$user_pass,$email=''){
		$isexist=\PhalApi\DI()->notorm->user
				->select('id')
				->where('country_code=? and user_login=? and user_email=? and user_type="2"',$country_code,$user_login,$email)
				->fetchOne();
		if(!$isexist){
			return 1006;
		}		
		$user_pass=\App\setPass($user_pass);

		return \PhalApi\DI()->notorm->user
				->where('id=?',$isexist['id']) 
				->update(array('user_pass'=>$user_pass));
		
	}	
		
	/* 第三方会员登录 */
    public function userLoginByThird($openid,$type,$nickname,$avatar,$source) {			
        $info=\PhalApi\DI()->notorm->user
            ->select($this->fields)
            ->where('openid=? and login_type=? ',$openid,$type)
            ->fetchOne();

		$configpri=\App\getConfigPri();
		if(!$info){
			/* 注册 */
			$user_pass='yunbaokeji';
			$user_pass=\App\setPass($user_pass);
			$user_login=$type.'_'.time().rand(100,999);

			if(!$nickname){
				$nickname=$type.\PhalApi\T('用户-').substr($openid,-4);
			}else{
				$nickname=urldecode($nickname);
			}
			
			if(!$avatar){
				$avatar='/default.jpg';
				$avatar_thumb='/default_thumb.jpg';
			}else{
				$avatar=urldecode($avatar);
				$avatar_thumb=$avatar;
			}

			$reg_reward=$configpri['reg_reward'];
			$data=array(
				'user_login' => $user_login,
				'user_nickname' =>$nickname,
				'user_pass' =>$user_pass,
				'signature' =>\PhalApi\T('这家伙很懒，什么都没留下'),
				'avatar' =>$avatar,
				'avatar_thumb' =>$avatar_thumb,
				'bg_img' =>$avatar,
				'last_login_ip' =>$_SERVER['REMOTE_ADDR'],
				'create_time' => time(),
				'user_status' => 1,
				'openid' => $openid,
				'login_type' => $type, 
				"user_type"=>2,//会员
				"source"=>$source,
				"coin"=>$reg_reward,
			);
			
			$rs=\PhalApi\DI()->notorm->user->insert($data);
            
            $uid=$rs['id'];

            if($reg_reward>0){

                $insert=array(
                	"type"=>'1',
                	"action"=>'11',
                	"uid"=>$uid,
                	"touid"=>$uid,
                	"giftid"=>0,
                	"giftcount"=>1,
                	"totalcoin"=>$reg_reward,
                	"showid"=>0,
                	"addtime"=>time()
                );

                \PhalApi\DI()->notorm->user_coinrecord->insert($insert);
            }

			$code=$this->createCode();
			$code_info=array('uid'=>$uid,'code'=>$code);
			$isexist=\PhalApi\DI()->notorm->agent_code
					->select("*")
					->where('uid = ?',$uid)
					->fetchOne();

			if($isexist){
				\PhalApi\DI()->notorm->agent_code->where('uid = ?',$uid)->update($code_info);	
			}else{
				\PhalApi\DI()->notorm->agent_code->insert($code_info);	
			}
            
			$info['id']=$uid;
			$info['user_nickname']=$data['user_nickname'];
			$info['avatar']=\App\get_upload_path($data['avatar']);
			$info['avatar_thumb']=\App\get_upload_path($data['avatar_thumb']);
			$info['sex']='2';
			$info['signature']=$data['signature'];
			$info['coin']='0';
			$info['login_type']=$data['login_type'];
			$info['province']='';
			$info['city']='';
			$info['birthday']='';
			$info['consumption']='0';
			$info['user_status']=1;
			$info['last_login_time']=0;
		}else{

			if($info['user_status']=='0'){
				return 1003;					
			}
			
			
			if($info['end_bantime']>time()){
				return 1002;					
			}

			//判断用户是否注销
			if($info['user_status']==3){
				return 1004;
			}

			//判断用户是否在语音聊天室上麦
			$voicelive_micinfo=\PhalApi\DI()->notorm->voicelive_mic->where("uid=?",$info['id'])->fetchOne();
			if($voicelive_micinfo){
				return 1005;
			}

            // 更新头像
			 if(!$avatar){
				$avatar='/default.jpg';
				$avatar_thumb='/default_thumb.jpg';
				$info['avatar']=$avatar;
				$info['avatar_thumb']=$avatar_thumb;
			}
			
		}
        
        $uid=$info['id'];

		unset($info['user_status']);

		unset($info['end_bantime']);
		
		$info['isreg']='0';
		
		if($info['last_login_time']==0 ){
			$info['isreg']='1';
		}

        
        if($info['birthday']){
            $info['birthday']=date('Y-m-d',$info['birthday']);   
        }else{
            $info['birthday']='';
        }
        
		unset($info['last_login_time']);
		
		$info['level']=\App\getLevel($info['consumption']);

		$info['level_anchor']=\App\getLevelAnchor($info['votestotal']);

		$token=md5(md5($uid.$openid.time()));
		
		$info['token']=$token;
		$info['avatar']=\App\get_upload_path($info['avatar']);
		$info['avatar_thumb']=\App\get_upload_path($info['avatar_thumb']);

		$info['id']=(string)$info['id'];
		
		$this->updateToken($uid,$token);
		
        return $info;
    }		
	
	/* 更新token 登陆信息 */
    public function updateToken($uid,$token,$data=array()) {
        $nowtime=time();
		$expiretime=$nowtime+60*60*24*300;

		\PhalApi\DI()->notorm->user
			->where('id=?',$uid)
			->update(array('last_login_time' => $nowtime, "last_login_ip"=>$_SERVER['REMOTE_ADDR'] ));
            
        $isok=\PhalApi\DI()->notorm->user_token
			->where('user_id=?',$uid)
			->update(array("token"=>$token, "expire_time"=>$expiretime ,'create_time' => $nowtime ));
        if(!$isok){
            \PhalApi\DI()->notorm->user_token
			->insert(array("user_id"=>$uid,"token"=>$token, "expire_time"=>$expiretime ,'create_time' => $nowtime, ));
        }

		$token_info=array(
			'uid'=>$uid,
			'token'=>$token,
			'expire_time'=>$expiretime,
		);
		
		\App\setcaches("token_".$uid,$token_info);		
        
		return 1;
    }	
	
	/* 生成邀请码 */
	public function createCode($len=6,$format='ALL2'){
        $is_abc = $is_numer = 0;
        $password = $tmp =''; 
        switch($format){
            case 'ALL':
                $chars='ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789';
                break;
            case 'ALL2':
                $chars='ABCDEFGHJKLMNPQRSTUVWXYZ0123456789';
                break;
            case 'CHAR':
                $chars='ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz';
                break;
            case 'NUMBER':
                $chars='0123456789';
                break;
            default :
                $chars='ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789';
                break;
        }
        
        while(strlen($password)<$len){
            $tmp =substr($chars,(mt_rand()%strlen($chars)),1);
            if(($is_numer <> 1 && is_numeric($tmp) && $tmp > 0 )|| $format == 'CHAR'){
                $is_numer = 1;
            }
            if(($is_abc <> 1 && preg_match('/[a-zA-Z]/',$tmp)) || $format == 'NUMBER'){
                $is_abc = 1;
            }
            $password.= $tmp;
        }
        if($is_numer <> 1 || $is_abc <> 1 || empty($password) ){
            $password = $this->createCode($len,$format);
        }
        if($password!=''){
            
            $oneinfo=\PhalApi\DI()->notorm->agent_code
	            ->select("uid")
	            ->where("code=?",$password)
	            ->fetchOne();
	        
            if(!$oneinfo){
                return $password;
            }            
        }
        $password = $this->createCode($len,$format);
        return $password;
    }
    

    //获取注销账号条件
    public function getCancelCondition($uid){

    	$res=array('list'=>array(),'can_cancel'=>'0');

    	$list=array(
    		'0'=>array(
    				'title'=>\PhalApi\T('1、账号内无大额未消费或未提现的财产'),
    				'content'=>\PhalApi\T('你账号内无未结清的欠款、资金和虚拟权益，无正在处理的提现记录；注销后，账户中的虚拟权益等将作废无法恢复。'),
    				'is_ok'=>'0'
    			),
	    		'1'=>array(
	    				'title'=>\PhalApi\T('2、账号无其它正在进行中的业务及争议纠纷'),
	    				'content'=>\PhalApi\T('本账号内已无正在处理的业务、申诉或争议纠纷'),
	    				'is_ok'=>'0'
	    			)
    	);

		//获取用户的星币余额
    	$userinfo=\PhalApi\DI()->notorm->user->where("id=?",$uid)->select("coin,votes,balance")->fetchOne();

		//获取用户星币提现未处理记录
    	$votes_cashlist=\PhalApi\DI()->notorm->cash_record->where("uid=? and status=0",$uid)->fetchAll();
    	//获取余额提现记录未处理记录
    	$balance_cashlist=\PhalApi\DI()->notorm->user_balance_cashrecord->where("uid=? and status=0",$uid)->fetchAll();
    	
		//星币小于100，余额为0
    	if($userinfo['coin']<100 && $userinfo['votes']<100 && $userinfo['balance']==0 && !$votes_cashlist && !$balance_cashlist){
    		$list[0]['is_ok']='1';
    	}

    	$list[1]['is_ok']='1';

    	if($list[0]['is_ok']==1&&$list[1]['is_ok']==1){
    		$res['can_cancel']='1';
    	}

    	$res['list']=$list;

    	return $res;

    }

    //注销账号
    public function cancelAccount($uid){

    	

    	$condition=$this->getCancelCondition($uid);

    	if(!$condition['can_cancel']){
    		return 1001;
    	}

    	
    	$now=time();

			//修改用户昵称
		\PhalApi\DI()->notorm->user->where("id=?",$uid)->update(array('user_nickname'=>'用户已注销','user_status'=>3));
		//未审核的视频改为拒绝
		\PhalApi\DI()->notorm->video->where("uid=? and status=0",$uid)->update(array('status'=>2));
		//上架的视频改为下架
		\PhalApi\DI()->notorm->video->where("uid=? and status=1 and isdel=0",$uid)->update(array('isdel'=>1));
		//未审核的动态拒绝
		\PhalApi\DI()->notorm->dynamic->where("uid=? and status=0",$uid)->update(array('status'=>2));
		//已经审核通过的动态下架
		\PhalApi\DI()->notorm->dynamic->where("uid=? and status=1 and isdel=0",$uid)->update(array('isdel'=>1));

        \App\delcache("userinfo_".$uid);
		return 1;

    }
}
