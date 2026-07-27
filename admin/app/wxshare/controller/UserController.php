<?php
// +----------------------------------------------------------------------
// | ThinkCMF [ WE CAN DO IT MORE SIMPLE ]
// +----------------------------------------------------------------------
// | Copyright (c) 2013-2014 http://www.thinkcmf.com All rights reserved.
// +----------------------------------------------------------------------
// | Author: Dean <zxxjjforever@163.com>
// +----------------------------------------------------------------------
namespace app\wxshare\controller;

use app\common\controller\HomeBaseController;
use think\facade\Db;
/**
 * 会员相关
 */
class UserController extends HomebaseController {
    
    protected $fields='id,user_nickname,avatar,avatar_thumb,sex,signature,coin,consumption,votestotal,province,city,birthday,user_status,login_type,last_login_time,end_bantime';
    //登录页
	public function login() {
		$configpub = getConfigPub();
	
	
		$this->assign('configpub', $configpub);
		
		return $this->fetch();
    }
	/* 登录 */
	public function userLogin(){
        $rs=['code'=>0,'msg'=>'登陆成功','info'=>[]];
        
        $data = $this->request->param();
		$where=[];
		$where['user_type']=2;
        $where['country_code']= $data['code']? checkNull($data['code']):'';
        $where['user_login']= $data['mobile']? checkNull($data['mobile']):'';
        $where['user_pass']= $data['pws']? cmf_password(checkNull($data['pws'])):'';


		$userinfo=Db::name('user')
			->where($where)
			->find();
		
		if(!$userinfo){
            $rs['code']=1120;
            $rs['msg']='账号或密码错误';
            echo json_encode($rs);
            return;
		}else if($userinfo['user_status']==0){
            $rs['code']=1120;
            $rs['msg']='账号已被拉黑';
            echo json_encode($rs);
            return;
		}else if($userinfo['user_status']==3){
            $rs['code']=1120;
            $rs['msg']='账号已注销';
            echo json_encode($rs);
            return;
		}else if($userinfo['user_status']==1 && $userinfo['end_bantime']>time()){
            $rs['code']=1120;
            $rs['msg']='账号已被禁用';
            echo json_encode($rs);
            return;
		}
		$userinfo['level']=getLevel($userinfo['consumption']);
        $token=md5(md5($userinfo['id'].$userinfo['user_login'].time()));
        $userinfo['token']=$token;        
        $this->updateToken($userinfo['id'],$userinfo['token']);


		session('uid',$userinfo['id']);
		session('token',$userinfo['token']);
		session('user',$userinfo);

        echo json_encode($rs);

	} 	
	
	//注册页
	public function lreg() {
		
		return $this->fetch();
    }
	/* 手机验证码-注册 */
	public function getCode(){
        $rs=['code'=>0,'msg'=>'','info'=>[]];
        $data = $this->request->param();
        $country_code= $data['code']? checkNull($data['code']):'';
        $mobile= $data['mobile']? checkNull($data['mobile']):'';
        
        $where['country_code']=$country_code;
        $where['user_login']=$mobile;

        //判断账号是否被禁用
        $user_status=Db::name("user")
			->where($where)
			->value("user_status");
        if($user_status==3){
        	$rs['code']=1120;
            $rs['msg']='该账号已注销';
            echo json_encode($rs);
            return;
        }
		$checkuser = checkUser($where);	
        if($checkuser){
            $rs['code']=1006;
            $rs['msg']='该手机号已注册，请登录';
            echo json_encode($rs);
            return;
        }

		if(session("mobile") && session("mobile")==$mobile && session("login_country_code") && session("login_country_code")==$country_code  && session("mobile_expiretime") && session("mobile_expiretime")> time() ){
            $rs['code']=1007;
            $rs['msg']='验证码5分钟有效，勿多发';
            echo json_encode($rs);
            return;
		}

        
		$limit = ip_limit();	
		if( $limit == 1){
            $rs['code']=1003;
            $rs['msg']='您已当日发送次数过多';
            echo json_encode($rs);
            return;
		}	

		$mobile_code = random(6,1);

		//密码可以使用明文密码或使用32位MD5加密
		$result = sendCode($country_code,$mobile,$mobile_code); 
		if($result['code']===0){
			session("login_country_code",$country_code);
			session("mobile",$mobile);
			session("mobile_code",$mobile_code);
			session("mobile_expiretime",time() +60*5);	
		}else if($result['code']==667){
			session("login_country_code",$country_code);
			session("mobile",$mobile);
            session("mobile_code",$result['msg']);
            session("mobile_expiretime",time() +60*5);
            
            $rs['code']=667;
            $rs['msg']='验证码为：'.$result['msg'];
            echo json_encode($rs);
            return;
		}else{
            $rs['code']=1004;
            $rs['msg']=$result['msg'];
            echo json_encode($rs);
            return;
		} 
        $rs['msg']='验证码已发送';
        echo json_encode($rs);
	}
	
	/* 注册 */
	public function userReg(){
        
        $rs=['code'=>0,'msg'=>'','data'=>[]];
        
        $data = $this->request->param();
		$where=[];
		$where['user_type']=2;
        

        $country_code= $data['code']? checkNull($data['code']):'';
        $user_login=$data['mobile']? checkNull($data['mobile']):'';
        $pass=$data['pws']? checkNull($data['pws']):'';
        $code=$data['mcode']? checkNull($data['mcode']):'';
        
        if( !session("mobile") || !session("login_country_code") || !session("mobile_code") ){
            $rs['code']=1120;
            $rs['msg']='请先获取验证码';
            echo json_encode($rs);
            return;
        }
		
		if($user_login!=session("mobile")){	
            $rs['code']=1120;
            $rs['msg']='手机号码不一致';
            echo json_encode($rs);
            return;
		}

		if($code!=session("mobile_code")){
            $rs['code']=1120;
            $rs['msg']='验证码错误';
            echo json_encode($rs);
            return;
		}	
		$check = passcheck($pass);
		if(!$check){
            $rs['code']=1120;
            $rs['msg']='密码为6-20位数字与字母组合';
            echo json_encode($rs);
            return;
		}
		$user_pass=cmf_password($pass);
		$where['country_code']=$country_code;
		$where['user_login']=$user_login;
		$ifreg=Db::name("user")->field("id")->where($where)->find();
		if($ifreg){
            $rs['code']=1120;
            $rs['msg']='该手机号已被注册';
            echo json_encode($rs);
            return;
		}
		
		/* 无信息 进行注册 */
        $nowtime=time();
		$configPri=getConfigPri();
        $reg_reward=$configPri['reg_reward'];
		$data=array(
			'country_code'  => $country_code,
			'user_login'    => $user_login,
			'user_email'    => '',
			'mobile'        =>$user_login,
			'user_nickname' =>'h5用户'.substr($user_login,-4),
			'user_pass'     =>$user_pass,
			'signature'     =>'这家伙很懒，什么都没留下',
			'avatar'        =>'/default.jpg',
			'avatar_thumb'  =>'/default_thumb.jpg',
			'last_login_ip' =>get_client_ip(0,true),
			'create_time'   => $nowtime,
			'last_login_time'=> $nowtime,
			'user_status'   => 1,
			"user_type"     =>2,//会员
		);

		if($reg_reward>0){

			$data['coin']=$reg_reward;
		}

		$userid=Db::name("user")->insertGetId($data);

        if($reg_reward){
            $insert=array(
                "type"      =>'1',
                "action"    =>'11',
                "uid"       =>$userid,
                "touid"     =>$userid,
                "giftid"    =>0,
                "giftcount" =>1,
                "totalcoin" =>$reg_reward,
                "showid"    =>0,
                "addtime"   =>time()
            );
            Db::name('user_coinrecord')->insert($insert);
        }
        
        $code=createCode();
        $code_info=array('uid'=>$userid,'code'=>$code);
        $isexist=Db::name("agent_code")
                    ->where("uid = {$userid}")
                    ->update($code_info);
        if(!$isexist){
            Db::name("agent_code")->insert($code_info);
        }
            
		$userinfo=Db::name("user")->where("id='{$userid}'")->find();
		$token=md5(md5($userinfo['id'].$userinfo['user_login'].time()));
		$userinfo['token']=$token;
		$userinfo['level']=getLevel($userinfo['consumption']);
        $this->updateToken($userinfo['id'],$userinfo['token']);

		session('uid',$userinfo['id']);
		session('token',$userinfo['token']);
		session('user',$userinfo);

        $rs['msg']='注册成功';
        echo json_encode($rs);

	}
	
	//忘记密码
	public function lwreg() {
		
		return $this->fetch();
    }
	/* 手机验证码 */
	public function getForgetCode(){
		
        $rs=['code'=>0,'msg'=>'','info'=>[]];
        $data = $this->request->param();
        $country_code= $data['code']? checkNull($data['code']):'';
        $mobile= $data['mobile']? checkNull($data['mobile']):'';
        
        $where['country_code']=$country_code;
        $where['user_login']=$mobile;

        //判断账号是否被禁用
        $user_status=Db::name("user")
			->where($where)
			->value("user_status");
        if($user_status==3){
        	$rs['code']=1120;
            $rs['msg']='该账号已注销';
            echo json_encode($rs);
            return;
        }
        
		$checkuser = checkUser($where);	
        
        if(!$checkuser){
            $rs['code']=1006;
            $rs['msg']='该手机号未注册，请先注册';
            echo json_encode($rs);
            return;
        }

		if(session("mobile") && 
            session("mobile")==$mobile &&
            session("forget_country_code") &&
            session("forget_country_code")==$country_code &&
            session("mobile_expiretime") &&
            session("mobile_expiretime")> time()
        ){
            $rs['code']=1007;
            $rs['msg']='验证码5分钟有效，勿多发';
            echo json_encode($rs);
            return;
		}
        
		$limit = ip_limit();	
		if( $limit == 1){
            $rs['code']=1003;
            $rs['msg']='您已当日发送次数过多';
            echo json_encode($rs);
            return;
		}	

		$mobile_code = random(6,1);

		//密码可以使用明文密码或使用32位MD5加密
		$result = sendCode($country_code,$mobile,$mobile_code); 
		if($result['code']===0){
			session("forget_country_code",$country_code);
			session("mobile",$mobile);
			session("mobile_code",$mobile_code);
			session("mobile_expiretime",time() +60*5);	
		}else if($result['code']==667){
			session("forget_country_code",$country_code);
			session("mobile",$mobile);
            session("mobile_code",$result['msg']);
            session("mobile_expiretime",time() +60*5);
            
            $rs['code']=667;
            $rs['msg']='验证码为：'.$result['msg'];
            echo json_encode($rs);
            return;
		}else{
            $rs['code']=1004;
            $rs['msg']=$result['msg'];
            echo json_encode($rs);
            return;
		} 
        $rs['msg']='验证码已发送';
        echo json_encode($rs);

	}			    
    public function userForget(){
        
        $rs=['code'=>0,'msg'=>'修改成功，请前往登录','data'=>[]];
        
        $data = $this->request->param();
		$where=[];
		$where['user_type']=2;
        

        $country_code= $data['code']? checkNull($data['code']):'';
        $user_login=$data['mobile']? checkNull($data['mobile']):'';
        $pass=$data['pws']? checkNull($data['pws']):'';
        $code=$data['mcode']? checkNull($data['mcode']):'';

        if( !session("mobile") || !session("forget_country_code") || !session("mobile_code") ){
            $rs['code']=1120;
            $rs['msg']='请先获取验证码';
            echo json_encode($rs);
            return;
        }
        
		if($user_login!=session("mobile")){	
            $rs['code']=1001;
            $rs['msg']='手机号码不一致';
            echo json_encode($rs);
            return;
		}

		if($code!=session("mobile_code")){
            $rs['code']=1001;
            $rs['msg']='验证码错误';
            echo json_encode($rs);
            return;
		}
        
        $check = passcheck($pass);

		if(!$check){
            $rs['code']=1120;
            $rs['msg']='密码为6-20位数字与字母组合';
            echo json_encode($rs);
            return;
		}

		$user_pass=cmf_password($pass);
		$where['country_code']=$country_code;
		$where['user_login']=$user_login;
		$ifreg=DB::name('user')->field("id")->where($where)->find();
		if(!$ifreg){
            $rs['code']=1001;
            $rs['msg']='该帐号不存在';
            echo json_encode($rs);
            return;
		}				
		$result=DB::name('user')->where("id='{$ifreg['id']}'")->update(['user_pass'=>$user_pass]);
		if($result===false){
            $rs['code']=10001;
            $rs['msg']='该帐号不存在';
            echo json_encode($rs);
            return;
		}
        echo json_encode($rs);
	}
	
	
	//qq第三方登录========
	public function qqLogin(){
		$referer = $_SERVER['HTTP_REFERER'];
		session('login_referer', $referer);
        
        require_once CMF_ROOT.'sdk/qqApi/qqConnectAPI.class.php';
		$qc1 = new \QC();
		$qc1->qq_login();
	}

	public function qqCallback(){
        require_once CMF_ROOT.'sdk/qqApi/qqConnectAPI.class.php';
        
		$qc = new \QC();
		$token = $qc->qq_callback();
		$openid = $qc->get_openid();
		$qq = new \QC($token, $openid);
		$arr = $qq->get_user_info();

        $type='qq';
        $openid=$openid;
        $nickname=$arr['nickname'];
        $avatar=$arr['figureurl_qq_2'];
        
        $userinfo=$this->loginByThird($type,$openid,$nickname,$avatar);

        if($userinfo==1001){
			$reason='该账号已被拉黑';
			$this->assign('reason', $reason);
			return $this->fetch(':error');
        }

        if($userinfo==1002){

			$reason='该账号已被注销';
			$this->assign('reason', $reason);
			return $this->fetch(':error');

        }

        if($userinfo==1003){
			$reason='该账号已被禁用';
			$this->assign('reason', $reason);
			return $this->fetch(':error');
        }

		session('uid',$userinfo['id']);
		session('token',$userinfo['token']);
		session('user',$userinfo);
		$href='/wxshare/share/index';
		header("Location: $href");	
	}
	
	//微信登录
	public function wxLogin(){
        
        
		$configpri=getConfigPri();
		
		$AppID = $configpri['login_wx_appid'];
		$callback  = get_upload_path('/wxshare/user/wxLoginCallback'); //回调地址
		//微信登录
		if (!session_id()) session_start();
		//-------生成唯一随机串防CSRF攻击
		$state  = md5(uniqid(rand(), TRUE));
		session("wx_state",$state); //存到SESSION
		$callback = urlencode($callback);
		//snsapi_base 静默  snsapi_userinfo 授权
		$wxurl = "https://open.weixin.qq.com/connect/oauth2/authorize?appid={$AppID}&redirect_uri={$callback}&response_type=code&scope=snsapi_userinfo&state={$state}#wechat_redirect ";
		
		header("Location: $wxurl");
	}
	
	public function wxLoginCallback(){
        
        $code   = $this->request->param('code');
        
		if($code){
			$configpri=getConfigPri();
		
			$AppID = $configpri['login_wx_appid'];
			$AppSecret = $configpri['login_wx_appsecret'];
			/* 获取token */
			$url="https://api.weixin.qq.com/sns/oauth2/access_token?appid={$AppID}&secret={$AppSecret}&code={$code}&grant_type=authorization_code";
			$ch = curl_init();
			curl_setopt($ch, CURLOPT_SSL_VERIFYPEER, FALSE);
			curl_setopt($ch, CURLOPT_RETURNTRANSFER, TRUE);
			curl_setopt($ch, CURLOPT_URL, $url);
			$json =  curl_exec($ch);
			curl_close($ch);
			$arr=json_decode($json,1);
            
            if(isset($arr['errcode'])){
                $this->assign('reason',$arr['errmsg']);
                return $this->fetch('error');
            }
            
			/* 刷新token 有效期为30天 */
			$url="https://api.weixin.qq.com/sns/oauth2/refresh_token?appid={$AppID}&grant_type=refresh_token&refresh_token={$arr['refresh_token']}";
			$ch = curl_init();
			curl_setopt($ch, CURLOPT_SSL_VERIFYPEER, FALSE);
			curl_setopt($ch, CURLOPT_RETURNTRANSFER, TRUE);
			curl_setopt($ch, CURLOPT_URL, $url);
			$json =  curl_exec($ch);
			curl_close($ch);
			
			$url="https://api.weixin.qq.com/sns/userinfo?access_token={$arr['access_token']}&openid={$arr['openid']}&lang=zh_CN";
			$ch = curl_init();
			curl_setopt($ch, CURLOPT_SSL_VERIFYPEER, FALSE);
			curl_setopt($ch, CURLOPT_RETURNTRANSFER, TRUE);
			curl_setopt($ch, CURLOPT_URL, $url);
			$json =  curl_exec($ch);
			curl_close($ch);
			$wxuser=json_decode($json,1);

			/* 公众号绑定到 开放平台 才有 unionid  否则 用 openid  */
			$openid=$wxuser['unionid'];
			if(!$openid){
                $this->assign('reason','公众号未绑定到开放平台');
                return $this->fetch('error');
			}
            $type='wx';
			$openid=$openid;
			$nickname=$wxuser['nickname'];
			$avatar=$wxuser['headimgurl'];
        
			$userinfo=$this->loginByThird($type,$openid,$nickname,$avatar);
			
			if($userinfo==1001){
				$reason='该账号已被拉黑';
				$this->assign('reason', $reason);
				return $this->fetch(':error');
			}

			if($userinfo==1002){

				$reason='该账号已被注销';
				$this->assign('reason', $reason);
				return $this->fetch(':error');

			}

			if($userinfo==1003){
				$reason='该账号已被禁用';
				$this->assign('reason', $reason);
				return $this->fetch(':error');
			}

			session('uid',$userinfo['id']);
			session('token',$userinfo['token']);
			session('user',$userinfo);
			
			$href='/wxshare/share/index';
		 	header("Location: $href");
			
		}
		
	}

	
	//我的
	public function index(){
		$uid=session('uid');
		if(!$uid){
			return $this->redirect('/wxshare/user/login');
		}
		
		$userinfo=getUserPrivateInfo($uid);
		$userinfo['follownums']=getFollownums($uid);
		$userinfo['fansnums']=getFansnums($uid);
		$userinfo['votes']=(int)$userinfo['coin'];

		$this->assign('userinfo',$userinfo);
		
		$configpri=getConfigPri();
		$this->assign('configpri',$configpri);
		return $this->fetch();
	}
	
	
	//我的--编辑
	public function uedit(){
		$uid=session('uid');
		if(!$uid){
			return $this->redirect('/wxshare/user/login');
		}
		
		$where['id']=$uid;
        $info= Db::name("user")
			->field('id,user_login,user_nickname,avatar,avatar_thumb,sex,signature,consumption,votestotal,coin,votes,birthday')
			->where($where)
			->find();
        if($info){
            $info['level']=getLevel($info['consumption']);
            $info['level_anchor']=getLevelAnchor($info['votestotal']);
            $info['avatar_t']=$info['avatar'];
            $info['avatar']=get_upload_path($info['avatar']);
            $info['avatar_thumb']=get_upload_path($info['avatar_thumb']);
            if($info['birthday']){
                $info['birthday']=date('Y-m-d',$info['birthday']);
            }else{
                $info['birthday']='';
            }

        }
		$this->assign('userinfo',$info);
		
		
		//获取后台插件配置的七牛信息
        $qiniu_plugin=Db::name("plugin")
			->where("name='Qiniu'")
			->find();

        if(!$qiniu_plugin){
            $reason='请联系管理员确认配置信息';
            $this->assign('reason', $reason);
            return $this->fetch(':error');
        }
        $qiniu_config=json_decode($qiniu_plugin['config'],true);

        if(!$qiniu_config){
            $reason='请联系管理员确认配置信息';
            $this->assign('reason', $reason);
            return $this->fetch(':error');
        }

        $protocol=$qiniu_config['protocol']; //协议名称
        $domain=$qiniu_config['domain']; //七牛加速域名
        $zone=$qiniu_config['zone']; //存储区域代号
		
		$upload_url='';
        switch ($zone) {
            case 'z0': //华东
                $upload_url='up.qiniup.com';
                break;
            case 'z1': //华北
                $upload_url='up-z1.qiniup.com';
                break;
            case 'z2': //华南
                $upload_url='up-z2.qiniup.com';
                break;
            case 'na0': //北美
                $upload_url='up-na0.qiniup.com';
                break;
            case 'as0': //东南亚
                $upload_url='up-as0.qiniup.com';
                break;
            
            default:
                $upload_url='up.qiniup.com';
                break;
        }
		
		$this->assign("protocol",$protocol);
        $this->assign("domain",$domain);
        $this->assign("upload_url",$upload_url);
	
		return $this->fetch();
	}
	
	public function ueditpost(){
		$rs=['code'=>0,'msg'=>'','data'=>[]];
		$uid=session('uid');
		if(!$uid){
			$rs['code']='700';
			$rs['msg']='您的登陆状态失效，请重新登陆！';
			echo json_encode($rs);
			exit;
		}
        $data = $this->request->param();
		
		$avatar= $data['avatar'] ?? '';
		
        $name= $data['nickname'] ?? '';
        $name=checkNull($name);
		
		$sex= $data['sex'] ?? '';
        $sex=checkNull($sex);
		
		$signature= $data['signature'] ?? '';
        $signature=checkNull($signature);
		
		
		$birthday= $data['birthday'] ?? '';
        $birthday=checkNull($birthday);
		
		$save=[];
		$save['user_nickname']=$name;
		$save['avatar']=$avatar;
		$save['avatar_thumb']=$avatar;
		$save['sex']=$sex;
		$save['signature']=$signature;
		$save['birthday']=strtotime($birthday);
        


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

        $where['id']=$uid;
		
		$userinfo= Db::name("user")
			->where($where)
			->update($save);
		
		
		$rs['msg']='修改失败！';
		if($userinfo){
			$userinfo=getUserInfo($uid);
			session('user',$userinfo);
			$rs['msg']='修改成功！';	
		}
		echo json_encode($rs);
		exit;
	}
	
	/**我的粉丝**/
	public function fans(){
		$uid=session('uid');
		if(!$uid){
			return $this->redirect('/wxshare/user/login');
		}

        
        $where['touid']=$uid;
		$attention=Db::name("user_attention")
			->where($where)
			->order("addtime desc")
			->select()
			->toArray();
		foreach($attention as $k=>$v){
			$users=getUserInfo($v['uid']);
	
			$attention[$k]['users']=$users;
			$isAttention=isAttention($uid,$v['uid']);
			$attention[$k]['isattention']=$isAttention;

			
		}
		$this->assign("attention",$attention);

		return $this->fetch();
	}
	
	/**我的关注**/
	public function follows(){
		$uid=session('uid');
		if(!$uid){
			return $this->redirect('/wxshare/user/login');
		}

        
        $where['uid']=$uid;
		$attention=Db::name("user_attention")
			->where($where)
			->order("addtime desc")
			->select()
			->toArray();
		foreach($attention as $k=>$v){
			$users=getUserInfo($v['touid']);
			$attention[$k]['users']=$users;
			$isAttention=isAttention($uid,$v['touid']);
			$attention[$k]['isattention']=$isAttention;
			
		}
		$this->assign("attention",$attention);

		return $this->fetch();
	}
	
	
	

	/* 退出 */
	public function logout(){
        
       
		session('uid',null);		
		session('token',null);
		session('user',null);

		return $this->redirect('/wxshare/user/login');
       

	}	
	
	
    
    protected function loginByThird($type,$openid,$nickname,$avatar){
        $info=DB::name('user')
            ->field($this->fields)
            ->where("openid='{$openid}' and login_type='{$type}' and user_type=2")
            ->find();
            
		$configpri=getConfigPri();
		if(!$info){
			/* 注册 */
			$user_pass='yunbaokeji';
			$user_pass=cmf_password($user_pass);
			$user_login=$type.'_'.time().rand(100,999);

			if(!$nickname){
				$nickname=$type.'用户-'.substr($openid,-4);
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
			
			$data=array(
				'user_login'    => $user_login,
				'user_nickname' =>$nickname,
				'user_pass'     =>$user_pass,
				'signature'     =>'这家伙很懒，什么都没留下',
				'avatar'        =>$avatar,
				'avatar_thumb'  =>$avatar_thumb,
				'last_login_ip' =>get_client_ip(0,true),
				'create_time'   => time(),
				'user_status'   => 1,
				'openid'        => $openid,
				'login_type'    => $type,
				"user_type"     =>2,//会员
				"source"        =>'pc'
			);
            
            $reg_reward=$configpri['reg_reward'];
            if($reg_reward>0){
                $data['coin']=$reg_reward;
            }
			
            $uid=DB::name('user')->insertGetId($data);

            if($reg_reward){
                $insert=array(
                    "type"      =>'1',
                    "action"    =>'11',
                    "uid"       =>$uid,
                    "touid"     =>$uid,
                    "giftid"    =>0,
                    "giftcount" =>1,
                    "totalcoin" =>$reg_reward,
                    "showid"    =>0,
                    "addtime"   =>time()
                );
                DB::name('user_coinrecord')->insert($insert);
            }
        
			$code=createCode();
			$code_info=array('uid'=>$uid,'code'=>$code);
			$isexist=DB::name('agent_code')
						->where("uid = {$uid}")
						->update($code_info);

			if(!$isexist){
				DB::name('agent_code')->insert($code_info);	
			}
            
			$info['id']=$uid;
			$info['user_nickname']=$data['user_nickname'];
			$info['avatar']=$data['avatar'];
			$info['avatar_thumb']=$data['avatar_thumb'];
			$info['sex']='2';
			$info['signature']=$data['signature'];
			$info['coin']='0';
			$info['login_type']=$data['login_type'];
			$info['province']='';
			$info['city']='';
			$info['birthday']='';
			$info['consumption']='0';
			$info['votestotal']='0';
			$info['user_status']=1;
			$info['last_login_time']='';
            $info['end_bantime']='0';
		}else{
			if(!$avatar){
				$avatar='/default.jpg';
				$avatar_thumb='/default_thumb.jpg';
			}else{
				$avatar=urldecode($avatar);
                $avatar_thumb=$avatar;
			}
			
			$info['avatar']=$avatar;
			$info['avatar_thumb']=$avatar_thumb;
			
			$data=array(
				'avatar' =>$avatar,
				'avatar_thumb' =>$avatar_thumb,
			);
			
		}
		
		if($info['user_status']=='0'){
			return 1001;					
		}

		if($info['user_status']=='3'){
			return 1002;					
		}

		if($info['user_status']=='1'&& $info['end_bantime']>time()){
			return 1003;					
		}
		
		$info['isreg']='0';
		$info['isagent']='0';
		if($info['last_login_time']=='' ){
			$info['isreg']='1';
			$info['isagent']='1';
		}

        if($configpri['agent_switch']==0){
            $info['isagent']='0';
        }
		unset($info['last_login_time']);
		
		$info['level']=getLevel($info['consumption']);

		$info['level_anchor']=getLevelAnchor($info['votestotal']);
        
        if($info['birthday']){
            $info['birthday']=date('Y-m-d',$info['birthday']);   
        }else{
            $info['birthday']='';
        }
		$token=md5(md5($info['id'].$openid.time()));
		$info['token']=$token;
		$info['avatar']=get_upload_path($info['avatar']);
		$info['avatar_thumb']=get_upload_path($info['avatar_thumb']);
        $this->updateToken($info['id'],$info['token']);
        return $info;
    }
	/* 更新token 登陆信息 */
    protected function updateToken($uid,$token) {
        $nowtime=time();
        
		$expiretime=$nowtime+60*60*24*300;

		DB::name("user")
			->where("id={$uid}")
			->update(array('last_login_time' => $nowtime, "last_login_ip"=>get_client_ip(0,true) ));
            
        $isok=DB::name("user_token")
			->where("user_id={$uid}")
			->update(array("token"=>$token, "expire_time"=>$expiretime , "create_time"=>$nowtime ));

        if(!$isok){
            DB::name("user_token")
			->insert(array("user_id"=>$uid,"token"=>$token, "expire_time"=>$expiretime , "create_time"=>$nowtime ));
        }

		$token_info=array(
			'uid'=>$uid,
			'token'=>$token,
			'expire_time'=>$expiretime,
		);
		
		setcaches("token_".$uid,$token_info);
        
		return 1;
    }
}

