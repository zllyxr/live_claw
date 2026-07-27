<?php
// +----------------------------------------------------------------------
// | ThinkCMF [ WE CAN DO IT MORE SIMPLE ]
// +----------------------------------------------------------------------
// | Copyright (c) 2013-2014 http://www.thinkcmf.com All rights reserved.
// +----------------------------------------------------------------------
// | Author: Dean <zxxjjforever@163.com>
// +----------------------------------------------------------------------
namespace app\home\controller;

use app\common\controller\HomeBaseController;
use think\facade\Db;
/**
 * 会员相关
 */
class UserController extends HomebaseController {
    
    protected $fields='id,user_nickname,avatar,avatar_thumb,sex,signature,coin,consumption,votestotal,province,city,birthday,user_status,login_type,last_login_time,end_bantime';

    //首页
	public function index() {

    }

	/* 手机验证码 */
	public function getCode(){
		
        $rs=['errno'=>0,'errmsg'=>'','data'=>[]];
        
        $data = $this->request->param();
        $captcha= $data['captcha'] ?? '';
        $country_code= $data['country_code'] ?? '';
        $mobile= $data['mobile'] ?? '';
        $captcha=checkNull($captcha);
        $mobile=checkNull($mobile);

        if (!cmf_captcha_check($captcha)) {
            
            $rs['errno']=1120;
            $rs['errmsg']='图片验证码不正确';
            echo json_encode($rs);
            return;
        }
        
        $where['country_code']=$country_code;
        $where['user_login']=$mobile;

        //判断账号是否被禁用
        $user_status=Db::name("user")->where($where)->value("user_status");
        if($user_status==3){
        	$rs['errno']=1120;
            $rs['errmsg']='该账号已注销';
            echo json_encode($rs);
            return;
        }
        
		$checkuser = checkUser($where);	
        
        if($checkuser){
            $rs['errno']=1006;
            $rs['errmsg']='该手机号已注册，请登录';
            echo json_encode($rs);
            return;
        }

		if(session("?mobile") && session("mobile")==$mobile && session("?login_country_code") && session("login_country_code")==$country_code  && session("?mobile_expiretime") && session("mobile_expiretime")> time() ){
            $rs['errno']=1007;
            $rs['errmsg']='验证码5分钟有效，勿多发';
            echo json_encode($rs);
            return;
		}
        
        
		$limit = ip_limit();	
		if( $limit == 1){
            $rs['errno']=1003;
            $rs['errmsg']='您已当日发送次数过多';
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
            
            $rs['errno']=667;
            $rs['errmsg']='验证码为：'.$result['msg'];
            echo json_encode($rs);
            return;
		}else{
            $rs['errno']=1004;
            $rs['errmsg']=$result['msg'];
            echo json_encode($rs);
            return;
		} 
        $rs['errmsg']='验证码已发送';
        echo json_encode($rs);
	}

	/* 图片验证码 */
	public function getCaptcha(){
        $rs=['errno'=>0,'errmsg'=>'','data'=>[]];
            $rs['data']['captcha']='/new_captcha.html?height=34&width=100&font_size=14&time=0.18649817613402453';
            $rs['errmsg']='请求成功';
            echo json_encode($rs);
	}		
		
	/* 登录 */

	public function userLogin(){
        $rs=['errno'=>0,'errmsg'=>'','data'=>[]];
        
        $data = $this->request->param();
        $country_code= $data['country_code'] ?? '';
        $user_login= $data['mobile'] ?? '';
        $pass= $data['pass'] ?? '';
        $user_login=checkNull($user_login);
        $pass=checkNull($pass);

		$user_pass=cmf_password($pass);
		$where['country_code']=$country_code;
        $where['user_login']=$user_login;
		$userinfo=Db::name('user')->where($where)->where("user_type='2'")->find();
		
		if(!$userinfo || $userinfo['user_pass'] != $user_pass){
            $rs['errno']=1120;
            $rs['errmsg']='账号或密码错误';
            echo json_encode($rs);
            return;
		}else if($userinfo['user_status']==0){
            $rs['errno']=1120;
            $rs['errmsg']='账号已被拉黑';
            echo json_encode($rs);
            return;
		}else if($userinfo['user_status']==3){
            $rs['errno']=1120;
            $rs['errmsg']='账号已注销';
            echo json_encode($rs);
            return;
		}else if($userinfo['user_status']==1 && $userinfo['end_bantime']>time()){
            $rs['errno']=1120;
            $rs['errmsg']='账号已被禁用';
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

        $rs['userid']=$userinfo['id'];
        $rs['errmsg']='登陆成功';
        echo json_encode($rs);

	} 	
		
	/* 注册 */
	public function userReg(){
        
        $rs=['errno'=>0,'errmsg'=>'','data'=>[]];
        
        $data = $this->request->param();
        $country_code= $data['country_code'] ?? '';
        $user_login= $data['mobile'] ?? '';
        $pass= $data['pass'] ?? '';
        $code= $data['code'] ?? '';
        
        $user_login=checkNull($user_login);
        $pass=checkNull($pass);
        $code=checkNull($code);
        
        if( !session("?mobile") || !session("?login_country_code") || !session("?mobile_code") ){
            $rs['errno']=1120;
            $rs['errmsg']='请先获取验证码';
            echo json_encode($rs);
            return;
        }
		
		if($user_login!=session("mobile")){	
            $rs['errno']=1120;
            $rs['errmsg']='手机号码不一致';
            echo json_encode($rs);
            return;
		}

		if($code!=session("mobile_code")){
            $rs['errno']=1120;
            $rs['errmsg']='验证码错误';
            echo json_encode($rs);
            return;
		}	
		$check = passcheck($pass);

		if(!$check){
            $rs['errno']=1120;
            $rs['errmsg']='密码为6-20位数字与字母组合';
            echo json_encode($rs);
            return;
		}

		$user_pass=cmf_password($pass);
		
		$where['country_code']=$country_code;
		$where['user_login']=$user_login;

		$ifreg=Db::name("user")->field("id")->where($where)->find();
		if($ifreg){
            $rs['errno']=1120;
            $rs['errmsg']='该手机号已被注册';
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
			'user_nickname' =>'PC用户'.substr($user_login,-4),
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
        
		/*$token=md5(md5($userinfo['id'].$userinfo['user_login'].time()));
		$userinfo['token']=$token;
		$userinfo['level']=getLevel($userinfo['consumption']);
        $this->updateToken($userinfo['id'],$userinfo['token']);

		session('uid',$userinfo['id']);
		session('token',$userinfo['token']);
		session('user',$userinfo);*/

        $rs['errmsg']='注册成功';
        $rs['userid']=$userinfo['id'];
        echo json_encode($rs);

	}
	
	/* 手机验证码 */
	public function getForgetCode(){
		
        $rs=['errno'=>0,'errmsg'=>'','data'=>[]];
        
        $data = $this->request->param();
        $captcha= $data['captcha'] ?? '';
        $country_code= $data['country_code'] ?? '';
        $mobile= $data['mobile'] ?? '';
        $captcha=checkNull($captcha);
        $mobile=checkNull($mobile);

        if (!cmf_captcha_check($captcha)) {
            $rs['errno']=1120;
            $rs['errmsg']='图片验证码不正确';
            echo json_encode($rs);
            return;
        }
        
        $where['country_code']=$country_code;
        $where['user_login']=$mobile;

        //判断账号是否被禁用
        $user_status=Db::name("user")->where($where)->value("user_status");
        if($user_status==3){
        	$rs['errno']=1120;
            $rs['errmsg']='该账号已注销';
            echo json_encode($rs);
            return;
        }
        
		$checkuser = checkUser($where);	
        
        if(!$checkuser){
            $rs['errno']=1006;
            $rs['errmsg']='该手机号未注册，请先注册';
            echo json_encode($rs);
            return;
        }

		if(session("?mobile") && 
            session("mobile")==$mobile &&
            session("?forget_country_code") &&
            session("forget_country_code")==$country_code &&
            session("?mobile_expiretime") &&
            session("mobile_expiretime")> time()
        ){
            $rs['errno']=1007;
            $rs['errmsg']='验证码5分钟有效，勿多发';
            echo json_encode($rs);
            return;
		}
        
		$limit = ip_limit();	
		if( $limit == 1){
            $rs['errno']=1003;
            $rs['errmsg']='您已当日发送次数过多';
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
            
            $rs['errno']=667;
            $rs['errmsg']='验证码为：'.$result['msg'];
            echo json_encode($rs);
            return;
		}else{
            $rs['errno']=1004;
            $rs['errmsg']=$result['msg'];
            echo json_encode($rs);
            return;
		} 
        $rs['errmsg']='验证码已发送';
        echo json_encode($rs);

	}			    
    public function forget(){
        
        $rs=['errno'=>0,'errmsg'=>'','data'=>[]];
        
        $data = $this->request->param();
        $country_code= $data['country_code'] ?? '';
        $user_login= $data['mobile'] ?? '';
        $pass= $data['pass'] ?? '';
        $code= $data['code'] ?? '';
        $user_login=checkNull($user_login);
        $pass=checkNull($pass);
        $code=checkNull($code);

        if( !session("?mobile") || !session("?forget_country_code") || !session("?mobile_code") ){
            $rs['errno']=1120;
            $rs['errmsg']='请先获取验证码';
            echo json_encode($rs);
            return;
        }
        
		if($user_login!=session("mobile")){	
            $rs['errno']=1001;
            $rs['errmsg']='手机号码不一致';
            echo json_encode($rs);
            return;
		}

		if($code!=session("mobile_code")){
            $rs['errno']=1001;
            $rs['errmsg']='验证码错误';
            echo json_encode($rs);
            return;
		}
        
        $check = passcheck($pass);

		if(!$check){
            $rs['errno']=1120;
            $rs['errmsg']='密码为6-20位数字与字母组合';
            echo json_encode($rs);
            return;
		}

		$user_pass=cmf_password($pass);
		$where['country_code']=$country_code;
		$where['user_login']=$user_login;
		$ifreg=DB::name('user')->field("id")->where($where)->find();
		if(!$ifreg){
            $rs['errno']=1001;
            $rs['errmsg']='该帐号不存在';
            echo json_encode($rs);
            return;
		}				
		$result=DB::name('user')->where("id='{$ifreg['id']}'")->update(['user_pass'=>$user_pass]);
		if($result===false){
            $rs['errno']=10001;
            $rs['errmsg']='该帐号不存在';
            echo json_encode($rs);
            return;
		}
        echo json_encode($rs);
	}

	/* 退出 */
	public function logout(){
        
        $rs=['errno'=>0,'errmsg'=>'','data'=>[]];
		session('uid',null);		
		session('token',null);
		session('user',null);

		/*echo $_COOKIE['uid'];
		echo $_COOKIE['token'];
		echo $_COOKIE['user'];
		die;*/

        $rs['errmsg']='退出登录';
        echo json_encode($rs);

	}	
	/* 获取用户信息 */
	public function getLoginUserInfo(){
        $rs=['errno'=>0,'errmsg'=>'','data'=>[]];
        
		$uid=session("uid");			
		if($uid){
            $rs['data']['user']=json_encode(getUserPrivateInfo($uid));
            echo json_encode($rs);
		}else{
            $rs['errno']=1001;
            $rs['errmsg']='未登录';
            echo json_encode($rs);
		}
	}		

	/**
	 * 检测拉黑状态
	 * @desc 用于私信聊天时判断私聊双方的拉黑状态
	 * @return int code 操作码，0表示成功
	 * @return array info 
	 * @return string info.u2t  是否拉黑对方,0表示未拉黑，1表示已拉黑
	 * @return string info.t2u  是否被对方拉黑,0表示未拉黑，1表示已拉黑
	 * @return string msg 提示信息
	 */
	function checkBlack() {
        $rs = array('code' => 0, 'msg' => '', 'info' => array());
            
        $data = $this->request->param();
        $uid= $data['uid'] ?? '';
        $touid= $data['touid'] ?? '';

        $uid=(int)checkNull($uid);
        $touid=(int)checkNull($touid);

        $u2t = isBlack($uid,$touid);
        $t2u = isBlack($touid,$uid);

        $rs['info']['u2t']=$u2t;
        $rs['info']['t2u']=$t2u;
        echo json_encode($rs);
	}

	//三方开启判断
	public function threeparty(){
    
		$data=array(
			"login_type"=>$this->configpub['login_type'],
		);
		echo json_encode($data);
	}

	//qq第三方登录========
	public function qq(){
		$href=$_SERVER['HTTP_REFERER'];
		cookie('href',$href,3600000);
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
        	$this->assign("jumpUrl",'/');
            $this->error('该账号已被拉黑');
            return;
        }

        if($userinfo==1002){
        	$this->assign("jumpUrl",'/');
            $this->error('该账号已被注销');
            return;

        }

        if($userinfo==1003){
        	$this->assign("jumpUrl",'/');
            $this->error('该账号已被禁用');
            return;
        }

		session('uid',$userinfo['id']);
		session('token',$userinfo['token']);
		session('user',$userinfo);
		$href=cookie('href');
		echo "<meta http-equiv=refresh content='0; url=$href'>"; 		
	}

	/**
	微信登陆 
	**/
	public function weixin(){
		$getConfigPri=getConfigPri();	
		$getConfigPub=getConfigPub();	
		$pay_url=$getConfigPub['site'];
	    //-------配置
		$href=$_SERVER['HTTP_REFERER'];
		cookie('href',$href,3600000);
		$AppID = $getConfigPri['login_wx_pc_appid'];
		$AppSecret = $getConfigPri['login_wx_pc_appsecret'];
		$callback  = $pay_url.'/home/User/weixin_callback'; //回调地址
		//微信登录
		if (!session_id()) session_start();
		//-------生成唯一随机串防CSRF攻击
		$state  = md5(uniqid(rand(), TRUE));
		session("wx_state",$state); //存到SESSION
		$callback = urlencode($callback);
		$wxurl = "https://open.weixin.qq.com/connect/qrconnect?appid=".$AppID."&redirect_uri={$callback}&response_type=code&scope=snsapi_login&state={$state}#wechat_redirect";
		header("Location: $wxurl");
	}

	/**
	微信登陆回调
	**/
	public function weixin_callback(){
		
		$getConfigPri=getConfigPri();
        $code= $_GET['code'] ?? '';
		if($code!="") {
			$AppID = $getConfigPri['login_wx_pc_appid'];
			$AppSecret = $getConfigPri['login_wx_pc_appsecret'];
			$url='https://api.weixin.qq.com/sns/oauth2/access_token?appid='.$AppID.'&secret='.$AppSecret.'&code='.$code.'&grant_type=authorization_code';
			$ch = curl_init();
			curl_setopt($ch, CURLOPT_SSL_VERIFYPEER, FALSE);
			curl_setopt($ch, CURLOPT_RETURNTRANSFER, TRUE);
			curl_setopt($ch, CURLOPT_URL, $url);
			$json =  curl_exec($ch);
			curl_close($ch);
			$arr=json_decode($json,1);
            
            if(isset($arr['errcode'])){
                echo $arr['errmsg'];
                return;
            }
            
			//得到 access_token 与 openid
			$url='https://api.weixin.qq.com/sns/userinfo?access_token='.$arr['access_token'].'&openid='.$arr['openid'].'&lang=zh_CN';
			$ch = curl_init();
			curl_setopt($ch, CURLOPT_SSL_VERIFYPEER, FALSE);
			curl_setopt($ch, CURLOPT_RETURNTRANSFER, TRUE);
			curl_setopt($ch, CURLOPT_URL, $url);
			$json =  curl_exec($ch);
			curl_close($ch);
			$arr=json_decode($json,1);
			//得到 用户资料
			//$openid=$arr['openid'];
			$openid=$arr['unionid'];
            
            $type='wx';
            $openid=$openid;
            $nickname=$arr['nickname'];
            $avatar=$arr['headimgurl'];
            
            $userinfo=$this->loginByThird($type,$openid,$nickname,$avatar);
            if($userinfo==1001){
            	$this->assign("jumpUrl",'/');
                $this->error('该账号已被禁用');
                return;
            }

            if($userinfo==1002){
	        	$this->assign("jumpUrl",'/');
	            $this->error('该账号已被注销');
	            return;
	        }

	        if($userinfo==1003){
	        	$this->assign("jumpUrl",'/');
	            $this->error('该账号已被禁用');
	            return;
	        }

			session('uid',$userinfo['id']);
			session('token',$userinfo['token']);
			session('user',$userinfo);
			$href=cookie('href');
		 	echo "<meta http-equiv=refresh content='0; url=$href'>"; 
		}
	}
	/**
	微博登陆
	**/
	public function weibo(){
		
		$href=$_SERVER['HTTP_REFERER'];
		cookie('href',$href,3600000);
		$getConfigPri=getConfigPri();	
		$getConfigPub=getConfigPub();	
		$WB_AKEY=$getConfigPri['login_sina_pc_akey'];
		$WB_SKEY=$getConfigPri['login_sina_pc_skey'];
		$pay_url=$getConfigPub['site'];
		$WB_CALLBACK_URL=$pay_url."/home/User/weibo_callback";
        
        require_once CMF_ROOT.'sdk/weibo/saetv2.ex.class.php';

		$o = new \SaeTOAuthV2($WB_AKEY,$WB_SKEY);
		$code_url = $o->getAuthorizeURL( $WB_CALLBACK_URL );
		header("location:".$code_url); 
	}
	/**
	微博登陆回调
	**/
	public function weibo_callback(){
        $code= $_GET['code'] ?? '';
		if($code!=""){
            require_once CMF_ROOT.'sdk/weibo/saetv2.ex.class.php';
			$getConfigPri=getConfigPri();	
			$getConfigPub=getConfigPub();	
			$WB_AKEY=$getConfigPri['login_sina_pc_akey'];
			$WB_SKEY=$getConfigPri['login_sina_pc_skey'];
			$pay_url=$getConfigPub['site'];
			$WB_CALLBACK_URL=$pay_url."/home/User/weibo_callback";
			$o = new \SaeTOAuthV2( $WB_AKEY , $WB_SKEY );
			$keys = array();
			$keys['code'] = $code;
			$keys['redirect_uri'] = $WB_CALLBACK_URL;
			$token = $o->getAccessToken( 'code', $keys ); 
			$c = new \SaeTClientV2( $WB_AKEY , $WB_SKEY ,$token["access_token"]);
			$ms = $c->home_timeline(); 
			$uid_get = $c->get_uid();
			$uid =  $token['uid'];
			$user_message = $c->show_user_by_id( $token['uid']);

            $type='sina';
            $openid=$user_message['id'];
            $nickname=$user_message['screen_name'];
            $avatar=$user_message['profile_image_url'];
            
            $userinfo=$this->loginByThird($type,$openid,$nickname,$avatar);
            if($userinfo==1001){
            	$this->assign("jumpUrl",'/');
                $this->error('该账号已被禁用');
                return;
            }

            if($userinfo==1002){
	        	$this->assign("jumpUrl",'/');
	            $this->error('该账号已被注销');
	            return;
	        }

	        if($userinfo==1003){
	        	$this->assign("jumpUrl",'/');
	            $this->error('该账号已被禁用');
	            return;
	        }
            
			session('uid',$userinfo['id']);
			session('token',$userinfo['token']);
			session('user',$userinfo);
			$href=$_COOKIE['AJ1sOD_href'];
		 	echo "<meta http-equiv=refresh content='0; url=$href'>";
		} 

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


