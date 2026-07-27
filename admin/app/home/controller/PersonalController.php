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
use cmf\lib\Upload;

class PersonalController extends HomebaseController {
    protected function initialize(){
        parent::initialize();
        $personal='';
        $this->assign("personal",$personal);
    }
  /**个人中心-首页方法**/
	public function index() {
		LogIn();
		$uid=session("uid");

		if($uid<1){
			session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            $this->assign("jumpUrl",'/');
            $this->error('您的登陆状态失效，请重新登陆！');
            return;
		}

		//判断用户的状态
        $userinfo=getUserInfo($uid);
        if($userinfo['user_status']==0){
            session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            $this->assign("jumpUrl",'/');
            $this->error('该账号已被拉黑');
            return;
        }

        if($userinfo['end_bantime']>time()){
            session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            $this->assign("jumpUrl",'/');
            $this->error('该账号已被禁用');
            return;
        }
		
		$info=getUserPrivateInfo($uid);	
		$this->assign("info",$info);
		$getgif=getgif($uid);
		$this->assign("getgif",$getgif[0]);

		return $this->fetch();
    }

	/**个人中心-头部修改昵称**/
	public function edit_name(){
		
		$uid=(int)session("uid");
        
        $data = $this->request->param();
        $name= $data['name'] ?? '';
        $name=checkNull($name);
        
        if($uid<1){
        	session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            echo '{"state":"700","msg":"您的登陆状态失效，请重新登陆！"}';
            return;
        }

        //判断用户的状态
        $userinfo=getUserInfo($uid);
        if($userinfo['user_status']==0){
            session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            echo '{"state":"700","msg":"该账号已被拉黑"}';
            return;
        }

        if($userinfo['end_bantime']>time()){
            session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            echo '{"state":"700","msg":"该账号已被禁用"}';
            return;
        }

        $where['id']=$uid;
		$userinfo= Db::name("user")->where($where)->update(['user_nickname'=>$name]);
		
		if($userinfo){
            $_SESSION['user']['user_nickname']=  $name;
			echo '{"state":"0","msg":"修改完成"}';
		}else{
			echo '{"state":"1","修改失败"}';
		}
	}

	 /**
	个人中心-基本资料展示
	**/
	public function modify() {
		LogIn();
		$uid=session("uid");

		if($uid<1){
			session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            $this->assign("jumpUrl",'/');
            $this->error('您的登陆状态失效，请重新登陆！');
            return;
		}

		$info=getUserPrivateInfo($uid);
		$this->assign("info",$info);
		$this->assign("personal",'Set');
		return $this->fetch();
    }
	 /**
	个人中心-基本资料修改
	**/
	public function edit_modify(){
	  
	  	$uid=(int)session("uid");
	  	$token=session("token");

	  	if($uid<1){
			session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            echo '{"state":"700","msg":"登录失效,请重新登录"}';
            return;
		}

		$checkToken=checkToken($uid,$token);
		if($checkToken==700){
			echo '{"state":"700","msg":"登录失效,请重新登录"}';
            return;
		}

		//判断用户的状态
        $userinfo=getUserInfo($uid);
        if($userinfo['user_status']==0){
            session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            echo '{"state":"700","msg":"该账号已被拉黑"}';
            return;
        }

        if($userinfo['end_bantime']>time()){
            session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            echo '{"state":"700","msg":"该账号已被禁用"}';
            return;
        }
        
        $data = $this->request->param();
        $birthday= $data['birthday'] ?? '';
        $birthday=checkNull($birthday);
        
        $user_nickname= $data['nickName'] ?? '';
        $user_nickname=checkNull($user_nickname);
        
        $sex= $data['sex'] ?? '';
        $sex=(int)checkNull($sex);
        
        $signature= $data['signature'] ?? '';
        $signature=checkNull($signature);

		 $up=array(
			"user_nickname"=>$user_nickname,
			"sex"=> $sex,
			"signature"=>$signature
		 );
         
         if($birthday){
             $birthday=strtotime($birthday);
             $up['birthday']=$birthday;
         }
		$result=Db::name("user")->where(['id'=>$uid])->update($up);
		if($result===false)
		{
            echo '{"state":"1","msg":"修改失败"}';
            return;
		}
        
        $_SESSION['user']['user_nickname']= $user_nickname;
        $_SESSION['user']['sex']= $sex;
        $_SESSION['user']['signature']= $signature;
        echo '{"state":"0","msg":"修改成功"}';

   }
    /**
	个人中心-头像展示
	**/
	public function photo(){
		LogIn();
		$uid=session("uid");

		if($uid<1){
			session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            $this->assign("jumpUrl",'/');
            $this->error('您的登陆状态失效，请重新登陆！');
            return;
		}

		//判断用户的状态
        $userinfo=getUserInfo($uid);
        if($userinfo['user_status']==0){
            session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            $this->assign("jumpUrl",'/');
            $this->error('该账号已被拉黑');
            return;
        }

        if($userinfo['end_bantime']>time()){
            session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            $this->assign("jumpUrl",'/');
            $this->error('该账号已被禁用');
            return;
        }

		$info= Db::name("user")
                ->field('id,user_login,user_nickname,avatar,avatar_thumb')
                ->where(['id'=>$uid])
                ->find();
        if($info){
            $info['avatar_s']=$info['avatar'];
            $info['avatar']=get_upload_path($info['avatar']);
            $info['avatar_thumb_s']=$info['avatar_thumb'];
            $info['avatar_thumb']=get_upload_path($info['avatar_thumb']);
        }
		$this->assign("info",$info);
		$this->assign("personal",'Set');
		return $this->fetch();
	}
	/**个人中心-修改头像**/
	public function edit_photo(){
		
		$uid=session("uid");
		$token=session("token");

		if($uid<1){
			session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            echo '{"error":"700","msg":"登录失效,请重新登录"}';
            return;
		}

		$checkToken=checkToken($uid,$token);
		if($checkToken==700){
			$callback = array(
				'error' => 0,
				'type'  => "登录失效,请重新登录"
				);
			echo json_encode($callback);
            return;
		}

		//判断用户的状态
        $userinfo=getUserInfo($uid);
        if($userinfo['user_status']==0){
            session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            echo '{"error":"700","msg":"该账号已被拉黑"}';
            return;
        }

        if($userinfo['end_bantime']>time()){
            session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            echo '{"error":"700","msg":"该账号已被禁用"}';
            return;
        }
        
        $data = $this->request->param();
        $url= $data['avatar'] ?? '';
        $url=checkNull($url);

		if (empty($url)) {
            $callback = array(
				'error' => 0,
				'type'  => "图片处理失败"
			);
            echo json_encode($callback);
            return;
        }

        $url=set_upload_path($url);

        $configpri=getConfigPri();
        $cloudtype=$configpri['cloudtype'];

        if($cloudtype==1){ //七牛云
        	$avatar=  $url.'?imageView2/2/w/600/h/600'; //600 X 600
        	$avatar_thumb=  $url.'?imageView2/2/w/200/h/200'; // 200 X 200
        }else{ //亚马逊
        	$avatar=  $url;
        	$avatar_thumb=  $url;
        }

        $data=array(
            "avatar"=>$avatar,
            "avatar_thumb"=>$avatar_thumb,
        );
        
        $result=Db::name('user')->where(['id'=>$uid])->update($data); 
        
        if($result===false){
            $callback = array(
                'error' => 0,
                'type'  => "头像修改失败"
            );
            echo json_encode($callback);
            return;
        }
        
        $_SESSION['user']['avatar']=get_upload_path($avatar);
        $_SESSION['user']['avatar_thumb']=get_upload_path($avatar_thumb);
        $callback = array(
            'error' => 1,
            'type'  => "头像修改成功"
            );
		echo json_encode($callback);
	}

	/**个人中心-我的认证**/
	public function card(){

		LogIn();
		$uid=(int)session("uid");

		if($uid<1){
			session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            $this->assign("jumpUrl",'/');
            $this->error('您的登陆状态失效，请重新登陆！');
            return;
		}

		//判断用户的状态
        $userinfo=getUserInfo($uid);
        if($userinfo['user_status']==0){
            session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            $this->assign("jumpUrl",'/');
            $this->error('该账号已被拉黑');
            return;
        }

        if($userinfo['end_bantime']>time()){
            session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            $this->assign("jumpUrl",'/');
            $this->error('该账号已被禁用');
            return;
        }

		$this->assign("uid",$uid);
        $where['uid']=$uid;
		$auth= Db::name("user_auth")->where($where)->find();
        if(!$auth){
            $auth['status'] =-1;
        }
		$info=getUserPrivateInfo($uid);	
		$this->assign("info",$info);
		$this->assign("auth",$auth);
		$this->assign("personal",'card');
		return $this->fetch();
	}
	/**
	个人中心-我的认证-身份证上传
	$info判断上传状态
	**/
	function upload(){
        
        $file= $_FILES['file'] ?? '';
        if($file){
            $name=$file['name'];
            $pathinfo = pathinfo($name);
            if(!isset($pathinfo['extension'])){
                $_FILES['file']['name']=$name.'.jpg';
            }
        }

        $configpri=getConfigPri();
        $cloudtype=$configpri['cloudtype'];
        if($cloudtype==1){ //七牛云存储
        	$uploader = new Upload();
	        $uploader->setFileType('image');
	        $res = $uploader->upload();

	        if ($res === false) {
	            echo json_encode(array("ret"=>0,'file'=>'','msg'=>$uploader->getError()));
                return;
	        }

	        $result=array(
                'url'=>$res['url'],
                'filepath'=>$res['filepath']
            );

        }else{
        	$res=adminUploadFiles($file,$cloudtype);
            if($res===false){
               echo json_encode(array("ret"=>0,'file'=>'','msg'=>'文件上传失败'));
                return;
            }

            $result=array(
                "url"=>$res['url'],
                "filepath"=>$res['storage_path']
            );
        }

        /* $result=[
            'filepath'    => $arrInfo["file_path"],
            "name"        => $arrInfo["filename"],
            'id'          => $strId,
            'preview_url' => cmf_get_root() . '/upload/' . $arrInfo["file_path"],
            'url'         => cmf_get_root() . '/upload/' . $arrInfo["file_path"],
        ]; */
        
        echo json_encode(array("ret"=>200,'data'=>array("url"=>$result['url'],"file_name"=>$result['filepath']),'msg'=>''));
	}

	/**
	个人中心-我的认证-认证信息写入数据库
	**/
	function authsave(){ 

		$uid=(int)session("uid");

        $data = $this->request->param();
        $real_name= $data['real_name'] ?? '';
        $real_name=checkNull($real_name);
        
        $mobile= $data['mobile'] ?? '';
        $mobile=checkNull($mobile);
        
        $cer_no= $data['cer_no'] ?? '';
        $cer_no=checkNull($cer_no);
        
        $front_view= $data['front_view'] ?? '';
        $front_view=checkNull($front_view);
        
        $back_view= $data['back_view'] ?? '';
        $back_view=checkNull($back_view);
        
        $handset_view= $data['handset_view'] ?? '';
        $handset_view=checkNull($handset_view);
        
        
        if($uid<1){
            echo json_encode(array("ret"=>0,'data'=>array(),'msg'=>'参数错误'));
            return;
        }
        
        if($real_name==''){
            echo json_encode(array("ret"=>0,'data'=>array(),'msg'=>'请填写您的真实姓名'));
            return;
        }
        
        if($mobile==''){
            echo json_encode(array("ret"=>0,'data'=>array(),'msg'=>'请填写您的手机号'));
            return;
        }
        
        if($cer_no==''){
            echo json_encode(array("ret"=>0,'data'=>array(),'msg'=>'请填写您的身份证号'));
            return;
        }
        
        if($front_view==''){
            echo json_encode(array("ret"=>0,'data'=>array(),'msg'=>'请上传证件相关照片'));
            return;
        }
        
        if($back_view==''){
            echo json_encode(array("ret"=>0,'data'=>array(),'msg'=>'请上传证件相关照片'));
            return;
        }
        
        if($handset_view==''){
            echo json_encode(array("ret"=>0,'data'=>array(),'msg'=>'请上传证件相关照片'));
            return;
        }
        
        $info=[
            'uid'           =>$uid,
            'real_name'     =>$real_name,
            'mobile'        =>$mobile,
            'cer_no'        =>$cer_no,
            'front_view'    =>set_upload_path($front_view),
            'back_view'     =>set_upload_path($back_view),
            'handset_view'  =>set_upload_path($handset_view),
            'status'        =>0,
            'addtime'       =>time(),
        ];
        
		$result=Db::name("user_auth")->where("uid='{$uid}'")->update($info);
		if(!$result)
		{
			$result=Db::name("user_auth")->insert($info);
		}
        
        if($result===false) {
			echo json_encode(array("ret"=>0,'data'=>array(),'msg'=>'提交失败，请重新提交'));
            return;
		}
			
        echo json_encode(array("ret"=>200,'data'=>array(),'msg'=>''));
	}

	/**
	个人中心-我关注的
	**/
  public function follow(){
		LogIn();
		$uid=(int)session("uid");

		if($uid<1){
			session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            $this->assign("jumpUrl",'/');
            $this->error('您的登陆状态失效，请重新登陆！');
            return;
		}

		//判断用户的状态
        $userinfo=getUserInfo($uid);
        if($userinfo['user_status']==0){
            session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            $this->assign("jumpUrl",'/');
            $this->error('该账号已被拉黑');
            return;
        }

        if($userinfo['end_bantime']>time()){
            session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            $this->assign("jumpUrl",'/');
            $this->error('该账号已被禁用');
            return;
        }

		$info=getUserPrivateInfo($uid);	
		$this->assign("info",$info);
        $where['uid']=$uid;
		$attention=Db::name("user_attention")->where($where)->order("addtime desc")->select()->toArray();
		foreach($attention as $k=>$v)
		{
			$users=getUserInfo($v['touid']);
			$attention[$k]['users']=$users;
            $attention[$k]['follow']=getFollownums($v['touid']);
            $attention[$k]['fans']=getFansnums($v['touid']);
		}
		$this->assign("attention",$attention);
		$this->assign("personal",'follow');
		return $this->fetch();
	}
	/**
	个人中心-我关注的-取消关注
	**/
	public function follow_dal(){
		
		$uid=(int)session("uid");
        if($uid<1){
			session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            echo '{"state":"700","msg":"您的登陆状态失效，请重新登陆！"}';
            return;
		}

		//判断用户的状态
        $userinfo=getUserInfo($uid);
        if($userinfo['user_status']==0){
            session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            echo '{"state":"700","msg":"该账号已被拉黑"}';
            return;
        }

        if($userinfo['end_bantime']>time()){
            session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
			echo '{"state":"700","msg":"该账号已被禁用"}';
            return;
        }
        $data = $this->request->param();
        $followID= $data['followID'] ?? '';
        $touid=(int)checkNull($followID);
        
        if($uid<1 || $touid<1){
			echo '{"state":"1","msg":"参数错误"}';
            return;
        }
        $where['touid']=$touid;
        $where['uid']=$uid;
        
		$del_follow=Db::name("user_attention")->where($where)->delete();
		if($del_follow===false)
		{
            echo '{"state":"1","msg":"取消失败"}';
            return;
		}
        echo '{"state":"0","msg":"取消关注成功"}';
	}


	public function follow_add(){
		
		$uid=(int)session("uid");
        if($uid<1){
			session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            echo '{"state":"700","msg":"您的登陆状态失效，请重新登陆！"}';
            return;
		}

		//判断用户的状态
        $userinfo=getUserInfo($uid);
        if($userinfo['user_status']==0){
            session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            echo '{"state":"700","msg":"该账号已被拉黑"}';
            return;
        }

        if($userinfo['end_bantime']>time()){
            session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
			echo '{"state":"700","msg":"该账号已被禁用"}';
            return;
        }
        $data = $this->request->param();
        $touid= $data['touid'] ?? '';
        $touid=(int)checkNull($touid);
        
        if($uid<1 || $touid<1){
			echo '{"state":"1","msg":"参数错误"}';
            return;
        }
		$data=array(
			"uid"=>$uid,
			"touid"=>$touid
		);
		$result=Db::name("user_attention")->insert(array("uid"=>$uid,"touid"=>$touid,"addtime"=>time()));
		if(!$result){

			echo '{"state":"1","msg":"关注失败"}';
            return;
		}
        
        Db::name("user_black")->where($data)->delete();
        echo '{"state":"0","msg":"关注成功"}';

	}


	/**
	个人中心-我的粉丝
	**/
	public function fans(){
		LogIn();
		$uid=(int)session("uid");
		if($uid<1){
			session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            $this->assign("jumpUrl",'/');
            $this->error('您的登陆状态失效，请重新登陆！');
            return;
		}

		//判断用户的状态
        $userinfo=getUserInfo($uid);
        if($userinfo['user_status']==0){
            session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            $this->assign("jumpUrl",'/');
            $this->error('该账号已被拉黑');
            return;
        }

        if($userinfo['end_bantime']>time()){
            session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            $this->assign("jumpUrl",'/');
            $this->error('该账号已被禁用');
            return;
        }
		$info=getUserPrivateInfo($uid);	
		$this->assign("info",$info);
        
        $where['touid']=$uid;
        
		$attention=Db::name("user_attention")->where($where)->order("addtime desc")->select()->toArray();
		foreach($attention as $k=>$v)
		{
			$users=getUserInfo($v['uid']);
			$attention[$k]['users']=$users;
      		$attention[$k]['follow']=getFollownums($v['uid']);
      		$attention[$k]['fans']=getFansnums($v['uid']);
			$isAttention=isAttention($uid,$v['uid']);
			$attention[$k]['attention']=$isAttention;
			$attention[$k]['isblack']=isBlack($uid,$v['uid']);
			
		}
		$this->assign("attention",$attention);
		$this->assign("personal",'follow');
		return $this->fetch();
	}

	/*黑名单*/
	public function namelist(){
		LogIn();
		$uid=(int)session("uid");
		if($uid<1){
			session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            $this->assign("jumpUrl",'/');
            $this->error('您的登陆状态失效，请重新登陆！');
            return;
		}

		//判断用户的状态
        $userinfo=getUserInfo($uid);
        if($userinfo['user_status']==0){
            session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            $this->assign("jumpUrl",'/');
            $this->error('该账号已被拉黑');
            return;
        }

        if($userinfo['end_bantime']>time()){
            session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            $this->assign("jumpUrl",'/');
            $this->error('该账号已被禁用');
            return;
        }
		$info=getUserPrivateInfo(session("uid"));	
		$this->assign("info",$info);
        
        $where['uid']=$uid;
        
		$attention=Db::name("user_black")->where($where)->select()->toArray();
		foreach($attention as $k=>$v)
		{
			$users=getUserInfo($v['touid']);
			$attention[$k]['users']=$users;
            $attention[$k]['follow']=getFollownums($v['touid']);
            $attention[$k]['fans']=getFansnums($v['touid']);
			$isAttention=isAttention($uid,$v['touid']);
			$attention[$k]['attention']=$isAttention;
		}
		$this->assign("attention",$attention);
		$this->assign("personal",'follow');
		return $this->fetch();
	}
	/*删除黑名单*/
	public function list_del(){
		$uid=(int)session("uid");
		if($uid<1){
			session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            echo '{"state":"700","msg":"您的登陆状态失效，请重新登陆！"}';
            return;
		}

		//判断用户的状态
        $userinfo=getUserInfo($uid);
        if($userinfo['user_status']==0){
            session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            echo '{"state":"700","msg":"该账号已被拉黑"}';
            return;
        }

        if($userinfo['end_bantime']>time()){
            session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
			echo '{"state":"700","msg":"该账号已被禁用"}';
            return;
        }
		$data = $this->request->param();
        $touid= $data['touid'] ?? '';
        $touid=(int)checkNull($touid);
        
        if($uid<1 || $touid<1){
            echo '{"state":"1000","msg":"参数错误"}';
            return;
        }
        
		$isBlack=isBlack($uid,$touid);
		if($isBlack==0)
		{
			echo '{"state":"1000","msg":"该用户不在你的黑名单内"}';
            return;
		}

        $where['touid']=$touid;
        $where['uid']=$uid;
        
        $attention=Db::name("user_black")->where($where)->delete();
        if($attention===false)
        {
            echo '{"state":"1001","msg":"移除失败"}';
            return;
        }
        
        echo '{"state":"0","msg":"移除成功"}';
	}



	public function blacklist(){
		$uid=(int)session("uid");
        if($uid<1){
			session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            echo '{"state":"700","msg":"您的登陆状态失效，请重新登陆！"}';
            return;
		}

		//判断用户的状态
        $userinfo=getUserInfo($uid);
        if($userinfo['user_status']==0){
            session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            echo '{"state":"700","msg":"该账号已被拉黑"}';
            return;
        }

        if($userinfo['end_bantime']>time()){
            session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
			echo '{"state":"700","msg":"该账号已被禁用"}';
            return;
        }
		$data = $this->request->param();
        $touid= $data['touid'] ?? '';
        $touid=(int)checkNull($touid);
        
        if($uid<1 || $touid<1){
            echo '{"state":"1000","msg":"参数错误"}';
            return;
        }
        
        
		$isBlack=isBlack($uid,$touid);
		if($isBlack==1)
		{
			echo '{"state":"1000","msg":"你已经将该用户拉黑"}';
            return;
		}
        
        $isAttention=isAttention($uid,$touid);
        if($isAttention)
        {
            $where['touid']=$touid;
            $where['uid']=$uid;
            
            Db::name('user_attention')->where($where)->delete();
        }
        
        $data=array(
            "uid"=>$uid,
            "touid"=>$touid
        );

        $result=Db::name('user_black')->insert($data);

        if(!$result)
        {
            echo '{"state":"1001","msg":"拉黑失败"}';
            return;
        }
        
        echo '{"state":"0","msg":"拉黑成功"}';
	}

	/**
	个人中心-管理员管理中心
	**/
	public function admin(){
		LogIn();
		$uid=(int)session("uid");
		if($uid<1){
			session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            $this->assign("jumpUrl",'/');
            $this->error('您的登陆状态失效，请重新登陆！');
            return;
		}

		//判断用户的状态
        $userinfo=getUserInfo($uid);
        if($userinfo['user_status']==0){
            session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            $this->assign("jumpUrl",'/');
            $this->error('该账号已被拉黑');
            return;
        }

        if($userinfo['end_bantime']>time()){
            session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            $this->assign("jumpUrl",'/');
            $this->error('该账号已被禁用');
            return;
        }

		$info=getUserPrivateInfo($uid);	
		$this->assign("info",$info);
        
        $where['liveuid']=$uid;
        
		$admin=Db::name("live_manager")->where($where)->select()->toArray();
		foreach($admin as $k=>$v)
		{
			$users=getUserInfo($v['uid']);
			$admin[$k]['users']=$users;
            $admin[$k]['follow']=getFollownums($v['uid']);
            $admin[$k]['fans']=getFansnums($v['uid']);
			$isAttention=isAttention($uid,$v['uid']);
			$admin[$k]['attention']=$isAttention;
		}
		$this->assign("admin",$admin);
		$this->assign("personal",'follow');
		return $this->fetch();
	}

	/**
	个人中心-管理员管理中心-取消管理员
	live_manager管理员记录表
	**/
	function admin_del(){ 
		$uid=(int)session("uid");
        if($uid<1){
			session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            echo '{"state":"700","msg":"您的登陆状态失效，请重新登陆！"}';
            return;
		}

		//判断用户的状态
        $userinfo=getUserInfo($uid);
        if($userinfo['user_status']==0){
            session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            echo '{"state":"700","msg":"该账号已被拉黑"}';
            return;
        }

        if($userinfo['end_bantime']>time()){
            session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
			echo '{"state":"700","msg":"该账号已被禁用"}';
            return;
        }
        $data = $this->request->param();
        $touid= $data['touid'] ?? '';
        $touid=(int)checkNull($touid);
        
        if($uid<1 || $touid<1){
            echo '{"state":"1000","msg":"参数错误"}';
            return;
        }
        
        $where['uid']=$touid;
        $where['liveuid']=$uid;
        
        $rst = Db::name("live_manager")->where($where)->delete();
        if (!$rst){
            echo '{"state":"1000","msg":"管理取消失败"}';
            return;
        }
        
        echo '{"state":"0","msg":"管理取消成功"}';
  }
	/**
	个人中心-提现中心
	
	**/
	public function exchange(){
		LogIn();
		$uid=(int)session("uid");
		$token=session("token");

		if($uid<1){
			session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            $this->assign("jumpUrl",'/');
            $this->error('您的登陆状态失效，请重新登陆！');
            return;
		}

		//判断用户的状态
        $userinfo=getUserInfo($uid);
        if($userinfo['user_status']==0){
            session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            $this->assign("jumpUrl",'/');
            $this->error('该账号已被拉黑');
            return;
        }

        if($userinfo['end_bantime']>time()){
            session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            $this->assign("jumpUrl",'/');
            $this->error('该账号已被禁用');
            return;
        }

		$info=getUserPrivateInfo($uid);	

		$config=getConfigPri();
		// 星币提现固定 1:1，不再使用第二套比例。
		$cash_rate=1;
		$cash_start=$config['cash_start'];
		$cash_end=$config['cash_end'];
		$cash_max_times=$config['cash_max_times'];
		$cash_take=0;
		// 统一钱包后，coin 为唯一可提现/消费余额。
		$votes=$info['coin'];
		$votestotal=$info['votestotal'];
			
		//总可提现星币
		$total=floor($votes);
        
        if($cash_max_times){
            //$tips='每月'.$cash_start.'-'.$cash_end.'号可进行提现申请，收益将在'.($cash_end+1).'-'.($cash_end+5).'号统一发放，每月只可提现'.$cash_max_times.'次';
            $tips='每月'.$cash_start.'-'.$cash_end.'号可进行提现申请，每月只可提现'.$cash_max_times.'次';
        }else{
            //$tips='每月'.$cash_start.'-'.$cash_end.'号可进行提现申请，收益将在'.($cash_end+1).'-'.($cash_end+5).'号统一发放';
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
		$this->assign("token",$token);
		
		$this->assign("uid",$uid);
	 	$this->assign("zlist",$zlist);
	 	$this->assign("info",$info);
	 	$this->assign("rs",$rs);
		$this->assign("personal",'card');
		return $this->fetch();
	}
	/**
	个人中心-提现中心开始提现
	**/
	public function edit_exchange(){
		
		
		$uid=(int)session("uid");
		$token=session("token");

		if($uid<1){
			session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            echo '{"code":"700","msg":"您的登陆状态失效，请重新登陆！"}';
            return;
		}

		//判断用户的状态
        $userinfo=getUserInfo($uid);
        if($userinfo['user_status']==0){
            session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            echo '{"code":"700","msg":"该账号已被拉黑"}';
            return;
        }

        if($userinfo['end_bantime']>time()){
            session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
			echo '{"code":"700","msg":"该账号已被禁用"}';
            return;
        }

		$checkToken=checkToken($uid,$token);
		if($checkToken==700){
			echo '{"code":"1003","msg":"您的登陆状态失效，请重新登陆！"}';
            return;
		}
		
        $where['uid']=$uid;
        
		$isrz=Db::name("user_auth")->field("status")->where($where)->find();
		if(!$isrz || $isrz['status']!=1){
			echo '{"code":"1003","msg":"请先进行身份认证"}';
            return;
		}
		
		$info=getUserPrivateInfo($uid);	
		
		$nowtime=time();
        $data = $this->request->param();
        $accountid= $data['accountid'] ?? '';
        $accountid=(int)checkNull($accountid);
        
        $cashvote= $data['cashvote'] ?? '';
        $cashvote=(int)checkNull($cashvote);
        
        
        if($accountid <1 || $cashvote<=0){
            echo '{"code":"1001","msg":"信息错误"}';
            return;
        }
        
        $config=getConfigPri();
        $cash_start=$config['cash_start'];
        $cash_end=$config['cash_end'];
        $cash_max_times=$config['cash_max_times'];
        
        $day=(int)date("d",$nowtime);
        if($day < $cash_start || $day > $cash_end){
            echo '{"code":"1005","msg":"不在提现期限内，不能提现"}';
            return;
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
                echo '{"code":"1006","msg":"每月只可提现'.$cash_max_times.'次,已达上限"}';
                return;
            }   
        }

		
		// 钱包信息
		$accountinfo=Db::name('cash_account')
				->where("id={$accountid} and uid={$uid}")
				->find();
        if(!$accountinfo){
            echo '{"code":"1006","msg":"该钱包不存在"}';
            return;
        }
		
		$votes=$info['coin'];
        
        if($cashvote > $votes){
            echo '{"code":"1001","msg":"余额不足"}';
            return;
        }
        

		// 最低额度
		$cash_min=$config['cash_min'];
		
		// 星币 1:1 提现
        $money=$cashvote;
		
		if($money < $cash_min){
            echo '{"code":"1001","msg":"提现最低额度为'.$cash_min.'星币"}';
            return;
		}
		
		$cashvotes=$money;
        $ifok=Db::name("user")
            ->where([['id','=',$uid],['coin','>=',$cashvotes]])
            ->dec('coin',$cashvotes)
            ->update();
        if(!$ifok){
            echo '{"code":"1001","msg":"余额不足"}';
            return;
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
		
		$rs=Db::name("cash_record")->insert($data);
		if(!$rs){
			echo '{"code":"1002","msg":"提现失败，请重试"}';
            return;
		}
        echo '{"code":"0","msg":"提现成功"}';

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

        return isset($status[$k]) ? $status[$k]: '';
    }


	public function cash_list(){
        LogIn();
        
		$uid=(int)session("uid");

		if($uid<1){
			session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            $this->assign("jumpUrl",'/');
            $this->error('您的登陆状态失效，请重新登陆！');
            return;
		}

		//判断用户的状态
        $userinfo=getUserInfo($uid);
        if($userinfo['user_status']==0){
            session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            $this->assign("jumpUrl",'/');
            $this->error('该账号已被拉黑');
            return;
        }

        if($userinfo['end_bantime']>time()){
            session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            $this->assign("jumpUrl",'/');
            $this->error('该账号已被禁用');
            return;
        }
		
		$info=getUserPrivateInfo($uid);	
		$pagesize = 20; 
        $where['uid']=$uid;
		
		$list=Db::name("cash_record")
			->where($where)
			->order("id desc")
			->paginate(20);
            
        $list->each(function($v,$k){
            $v['addtime']=date('Y.m.d',$v['addtime']);
            $v['status_name']=$this->getStatus($v['status']);
            return $v; 
        });
        
        $page = $list->render();

    	$this->assign('list', $list);

    	$this->assign("page", $page);
        
		$this->assign("info",$info);

		return $this->fetch();
	}
	
	
		/**
	个人中心-账号管理
	
	**/
	public function account_list(){
		LogIn();
		$uid=(int)session("uid");
		$token=session("token");
		$pagesize = 20;

		if($uid<1){
			session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            $this->assign("jumpUrl",'/');
            $this->error('您的登陆状态失效，请重新登陆！');
            return;
		}

		//判断用户的状态
        $userinfo=getUserInfo($uid);
        if($userinfo['user_status']==0){
            session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            $this->assign("jumpUrl",'/');
            $this->error('该账号已被拉黑');
            return;
        }

        if($userinfo['end_bantime']>time()){
            session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            $this->assign("jumpUrl",'/');
            $this->error('该账号已被禁用');
            return;
        }

		$info=getUserPrivateInfo($uid);	
        $where['uid']=$uid;
		
        
		$list=Db::name('cash_account')
				->where($where)
                ->order("id desc")
			   ->paginate(20);
		
        $list->each(function($v,$k){
            
            if($v['type']==1){
				$v['type_account']="支付宝";
				$v['account_bank']="-----";
			}else if($v['type']==2){
				$v['type_account']="微信";
				$v['account_bank']="-----";
				$v['name']="-----";
			}else if($v['type']==3){
				$v['type_account']="银行卡";
			}
            return $v; 
        });
        
        
        $page = $list->render();

    	$this->assign('list', $list);
    	$this->assign("page", $page);
		$this->assign("token",$token);
		$this->assign("uid",$uid);
		$this->assign("info",$info);
		return $this->fetch();
	}
	/*修改密码*/
	public function updatepass(){
		LogIn();
        $uid=(int)session("uid");

        if($uid<1){
			session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            $this->assign("jumpUrl",'/');
            $this->error('您的登陆状态失效，请重新登陆！');
            return;
		}

		//判断用户的状态
        $userinfo=getUserInfo($uid);
        if($userinfo['user_status']==0){
            session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            $this->assign("jumpUrl",'/');
            $this->error('该账号已被拉黑');
            return;
        }

        if($userinfo['end_bantime']>time()){
            session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            $this->assign("jumpUrl",'/');
            $this->error('该账号已被禁用');
            return;
        }

		$info=getUserPrivateInfo($uid);	
		$this->assign("info",$info);
		$this->assign("personal",'Set');
		return $this->fetch();
	}
	/* 执行密码修改 */
	public function savepass() {
		$uid=(int)session("uid");
        
		if($uid<1){
			session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            $rs['code'] = 700;
			$rs['msg'] = '您的登陆状态失效，请重新登陆！';
			echo json_encode($rs);
            return;
		}

		//判断用户的状态
        $userinfo=getUserInfo($uid);
        if($userinfo['user_status']==0){
            session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            $rs['code'] = 700;
			$rs['msg'] = '该账号已被拉黑';
			echo json_encode($rs);
            return;
        }

        if($userinfo['end_bantime']>time()){
            session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            $rs['code'] = 700;
			$rs['msg'] = '该账号已被禁用';
			echo json_encode($rs);
            return;
        }

        $data = $this->request->param();
        $oldpass= $data['oldpass'] ?? '';
        $oldpass=checkNull($oldpass);
        
        $newpass= $data['newpass'] ?? '';
        $newpass=checkNull($newpass);
        
        $repass= $data['repass'] ?? '';
        $repass=checkNull($repass);
        
        if($oldpass==''){
            $rs['code'] = 1001;
			$rs['msg'] = '请输入旧密码';
			echo json_encode($rs);
            return;
        }
        
        if($newpass==''){
            $rs['code'] = 1001;
			$rs['msg'] = '请输入新密码';
			echo json_encode($rs);
            return;
        }
        
		$rs=array();
		if($newpass !== $repass)
		{
			$rs['code'] = 800;
			$rs['msg'] = '两次密码不一致';
			echo json_encode($rs);
            return;
		}
		
		$check =passcheck($newpass); 
		if(!$check)
		{
			$rs['code'] = 1001;
			$rs['msg'] = '密码为6-20位数字与字母组合';
			echo json_encode($rs);
            return;
		}
        
        $oldpass = cmf_password($oldpass);
		/* 密码判定 */
        $where['id']=$uid;
        $where['user_type']=2;
        
		$rt=Db::name("user")->where($where)->value('user_pass');
		if(!$rt || $rt!=$oldpass){
			$rs['code'] = 103;
			$rs['msg'] = '旧密码错误';
			echo json_encode($rs);
            return;
		}
        
        
		$pwd = cmf_password($newpass);

		$map['id'] =$uid;
		//保存昵称到数据库
		$result=Db::name("user")->where($map)->update(['user_pass'=>$pwd]);
		if($result===false){
			$rs['code'] = 1005;
			$rs['msg'] = '修改失败';
			echo json_encode($rs);
            return;
		}
		
		$rs['code'] = 0;
        $rs['msg'] = '修改成功';
        echo json_encode($rs);

  }
	/**
	个人中心-直播记录
	**/
	public function live(){
		LogIn();
		$uid=(int)session("uid");
		
		if($uid<1){
			session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            $this->assign("jumpUrl",'/');
            $this->error('您的登陆状态失效，请重新登陆！');
            return;
		}

		//判断用户的状态
        $userinfo=getUserInfo($uid);
        if($userinfo['user_status']==0){
            session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            $this->assign("jumpUrl",'/');
            $this->error('该账号已被拉黑');
            return;
        }

        if($userinfo['end_bantime']>time()){
            session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            $this->assign("jumpUrl",'/');
            $this->error('该账号已被禁用');
            return;
        }

	 	$where=array();
		$where['uid']=$uid;
        
        $data = $this->request->param();
        $map=[];
        
        $start_time= $data['start_time'] ?? '';
        $end_time= $data['end_time'] ?? '';
        
        if($start_time!=""){
           $map[]=['starttime','>=',strtotime($start_time)];
        }

        if($end_time!=""){
           $map[]=['starttime','<=',strtotime($end_time) + 60*60*24];
        }
        
		
		$info=getUserPrivateInfo(session("uid"));	
		$this->assign("info",$info);
        
        $where2['id']=$uid;
        
		$coin=Db::name('user')->where($where2)->value("coin");
		$this->assign('coin',$coin);
		
		$lists = Db::name('live_record')
                ->where($where)
                ->order("id desc")
                ->paginate(20);

        $lists->each(function($v,$k){
            $v['starttime']=date('Y-m-d H:is',$v['starttime']);
            if($v['endtime']){
                $v['endtime']=date('Y-m-d H:is',$v['endtime']);
            }else{
                $v['endtime']='';
            }
            
            $type=$v['type'];
            $type_s='一般直播';
            switch($type){
                case '3':
                    $type_s='计时直播';
                    break;
                case '2':
                    $type_s='门票直播';
                    break;
                case '1':
                    $type_s='私密直播';
                    break;
                case '0':
                    $type_s='一般直播';
                    break;
                default:
                    $type_s='一般直播';

            }
            $v['type_s']=$type_s;

            if($v['live_type']==0){
            	$v['live_type_name']='视频直播';
            }else{
            	$v['live_type_name']='语音聊天室';
            }
            
            return $v;
        });
        
        $lists->appends($data);
        $page = $lists->render();

    	$this->assign('lists', $lists);

    	$this->assign("page", $page);
        
		$this->assign('uid',$uid);
		$this->assign("personal",'follow');
		return $this->fetch();
	}	
}


