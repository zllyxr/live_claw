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
 * 直播页面
 */
class ShowController extends HomebaseController {
  //首页
	public function index(){
		$anchorid = $this->request->param('roomnum', 0, 'intval');
		return redirect('/h5/#/pages/live/player?liveuid=' . $anchorid);
	}
	/*
	二维码=====
	value 二维码连接地址
	*/
	function qrcode(){
        
		$roomid = $this->request->param('roomid', 0, 'intval');
        
        require_once CMF_ROOT.'sdk/phpqrcode/phpqrcode.php';
        
		$a= new \QRcode();
		$value = get_upload_path('/wxshare/Share/show?roomnum='.$roomid);
		$errorCorrectionLevel = 'L';//容错级别 
		$matrixPointSize = 6;//生成图片大小 
		//生成二维码图片 
        $qrcode=WEB_ROOT.'upload/qr/qrcode_'.$roomid.'.png';
		$a->png($value, $qrcode, $errorCorrectionLevel, $matrixPointSize, 2); 
		$logo = 'logo.png';//准备好的logo图片
		$QR = $qrcode;//已经生成的原始二维码图 
		if ($logo !== FALSE) { 
			$QR = imagecreatefromstring(file_get_contents($QR)); 
			$logo = imagecreatefromstring(file_get_contents($logo)); 
			$QR_width = imagesx($QR);//二维码图片宽度 
			$QR_height = imagesy($QR);//二维码图片高度 
			$logo_width = imagesx($logo);//logo图片宽度 
			$logo_height = imagesy($logo);//logo图片高度 
			$logo_qr_width = $QR_width / 5; 
			$scale = $logo_width/$logo_qr_width; 
			$logo_qr_height = $logo_height/$scale; 
			$from_width = ($QR_width - $logo_qr_width) / 2; 
			//重新组合图片并调整大小 
			imagecopyresampled($QR, $logo, $from_width, $from_width, 0, 0, $logo_qr_width, 
			$logo_qr_height, $logo_width, $logo_height); 
		} 
		//输出图片 
		Header("Content-type: image/png");
		ImagePng($QR);
	}	
	//开播设置
	public function createRoom(){
        
		$uid=(int)session("uid");
		$token=session("token");
        
        $data = $this->request->param();
        $stream= $data['stream'] ?? '';
        $title= $data['title'] ?? '';
        $type= $data['type'] ?? '';
        $type_val= $data['stand'] ?? '';
        $live_class_id= $data['live_class_id'] ?? '';
        
        $type=(int)checkNull($type);
        $stream=checkNull($stream);
        $title=checkNull($title);
        $type_val=checkNull($type_val);
        $live_class_id=checkNull($live_class_id);
        
        
        if($uid<1){
        	session('uid',null);		
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            echo '{"state":"1000","data":"","msg":"您的登陆状态失效，请重新登陆！"}';
            return;
        }
        
        $now=time();
        
        $where['user_id']=$uid;
        
		$userinfo= Db::name("user_token")
                ->field('token,expire_time')
                ->where($where)
                ->find();
        
		if($userinfo['token']!=session("token") || $userinfo['expire_time']<$now){
			session('uid',null);		
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
			echo '{"state":"1002","data":"","msg":"您的登陆状态失效，请重新登陆！"}';
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
			echo '{"state":"1002","data":"","msg":"该账号已被拉黑"}';
            return;
		}

		if($userinfo['end_bantime']>$now){
			session('uid',null);		
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
			echo '{"state":"1002","data":"","msg":"该账号已被禁用"}';
            return;
		}

		//判断用户直播是否被封禁
		$liveban_info=Db::name('live_ban')->where(['liveuid'=>$uid])->find();

		if($liveban_info){
			$ban_endtime=$liveban_info['endtime'];
			$ban_type=$liveban_info['type'];
			if($ban_endtime==0 && $ban_type=='all'){
				echo '{"state":"1003","data":"","msg":"你已经被永久封禁直播,请联系管理员"}';
                return;
			}

			if($ban_endtime>0){
				if($ban_endtime<=$now){
					Db::name('live_ban')->where(['liveuid'=>$uid])->delete();
				}else{
					echo '{"state":"1003","data":"","msg":"你已经被封禁直播,请联系管理员"}';
                    return;
				}
			}
		}
        
		$configpri=getConfigPri();
		$stream_a=explode("_",$stream);
		$showid=$stream_a[1];
		
		$liang=getUserLiang($uid);
		$goodnum=0;
		if($liang['name']!=0){
			$goodnum=$liang['name'];
		}
        
        $nowtime=time();
        $user=Db::name("user")->field('ishot,isrecommend')->where(['id'=>$uid])->find();

		$data=array(
			"ishot"     	=>$user['ishot'],
			"isrecommend"	=>$user['isrecommend'],
			"showid"    	=>$showid,
			"starttime" 	=>$nowtime,
			"title"     	=>$title,
			"province"  	=>"",
			"city"      	=>"好像在火星",
			"stream"    	=>$stream,
			"lng"       	=>"",
			"lat"       	=>"",
			"goodnum"   	=>$goodnum,
			"type"      	=>$type,
			"type_val"  	=>$type_val,
			"hotvotes"  	=>0,
			"isvideo"   	=>0,
			"anyway"    	=>1,
			"liveclassid"	=>$live_class_id
		);

		$cdn_switch=$configpri['cdn_switch']; //1 腾讯云 2 声网

		$cdn_switch=2;

		if($cdn_switch==1){
			$data['islive']=1;
		}

		$ifok=Db::name("live")->where('uid='.$uid)->update($data);
		if(!$ifok){

			$data['uid']=$uid;
			$data['pull']='';
			$ifok=Db::name("live")->insert($data);
		}
		
		if($ifok){

			//声网创建云端播放器
			if($cdn_switch==2){
				
				$sw_uid=(int)$uid;
				$sw_appid = $configpri['sw_app_id'];
				$sw_channelName = $stream;
				$sw_token=getShengWangRtcToken($sw_uid,$sw_channelName);
				$sw_pull_url = PrivateKeyA('http',$sw_channelName,0);
				$sw_region=$configpri['sw_media_region'];

				if($sw_region=='cn'){
					$sw_request_url='api.sd-rtn.com';
				}else{
					$sw_request_url='api.agora.io';
				}

				$sw_url="https://{$sw_request_url}/{$sw_region}/v1/projects/".$sw_appid."/cloud-player/players";

				$authorization = getSwHttpAuthorization();

				$sw_header=[
					"Authorization"=>$authorization,
					"Content-Type"=>"application/json"
				];

				$sw_params=[
					"player"=>[
						"channelName"=>$sw_channelName,
						"uid"=>$sw_uid,
						"token"=>$sw_token,
						"name"=>$sw_channelName."_live",
						"audioOptions"=>[
							"volume"=>100,
							"Profile"=>0
						],
						"videoOptions"=>[
							"width"=>1280,
							"height"=>720,
							"widthHeightAdaption"=>false,
						],
						"streamUrl"=>$sw_pull_url,
						"playTs"=>0,
						"idleTimeout"=>5

					]
				];

				$curl_response = swCurlPost($sw_url,$sw_header,$sw_params);

				//file_put_contents("aa.txt", $curl_response);
				
			}
			

			$result['city']="好像在火星";
			
			setcaches($token,$result);

			//开播直播计时---用于每日任务--记录主播开播
			$key='open_live_daily_tasks_'.$uid;
			$room_time=time();
			setcaches($key,$room_time);
			
			echo '{"state":"0","data":"'.$type.'","msg":""}';
		}else{
			echo '{"state":"1000","data":"","msg":"直播信息处理失败"}';
		}

	}
    
    
    /* 关播 */
    public function stopRoom(){
        
        $rs=array('code'=>0,'msg'=>'关播成功','info'=>array());
        $uid=(int)session("uid");
        /*$cookie_uid=$_COOKIE['uid'];

        if($cookie_uid && !$uid){

        	$uid=$cookie_uid;
        	session('uid',$uid);
        }*/
        
        $data = $this->request->param();
        $stream= $data['stream'] ?? '';
        
        $stream=checkNull($stream);
        
        if(!$uid || !$stream){
            $rs['code']=1001;
            $rs['msg']='参数错误';
            echo json_encode($rs);
            return;
        }
        
        $where['islive']=1;
        $where['uid']=$uid;
        $where['stream']=$stream;

		$info=Db::name('live')
				->field("uid,showid,starttime,title,province,city,stream,lng,lat,type,type_val,liveclassid,live_type,sw_player_id")
				->where($where)
				->find();

		if($info){
			Db::name("live")->where("uid={$uid}")->delete();
			$nowtime=time();
			$info['endtime']=$nowtime;
			$info['time']=date("Y-m-d",$info['showid']);
			$votes=Db::name("user_voterecord")
				->where("uid={$uid} and showid={$info['showid']}")
				->sum('total');
			$info['votes']=0;
			if($votes){
				$info['votes']=$votes;
			}

			$nums=zSize('user_'.$stream);			
			hDel("livelist",$uid);
			delcache($uid.'_zombie');
			delcache($uid.'_zombie_uid');
			delcache('attention_'.$uid);
			delcache('user_'.$stream);
			$info['nums']=$nums;
			
			$sw_player_id=$info['sw_player_id'];

			unset($info['sw_player_id']);
			$result=Db::name('live_record')->insert($info);

			//解除本场禁言
			$list2=Db::name("live_shut")
                ->field('uid')
                ->where("liveuid={$uid} and showid!=0")
                ->select();

            Db::name("live_shut")->where("liveuid={$uid} and showid!=0")->delete();

            foreach($list2 as $k=>$v){
                hDel($uid . 'shutup',$v['uid']);
            }

            //游戏处理
            $game=Db::name("game")
				->where("stream='{$stream}' and liveuid={$uid} and state=0")
				->find();

			$total=array();
			if($game){
				$total=Db::name("gamerecord")
					->field("uid,sum(coin_1 + coin_2 + coin_3 + coin_4 + coin_5 + coin_6) as total")
					->where("gameid={$game['id']}")
					->group('uid')
					->select();

				foreach($total as $k=>$v){
					Db::name("user")
						->where("id={$v['uid']}")
						->inc("coin",$v['total'])
                        ->update();
					
					$insert=array(
                        "type"      =>'1',
                        "action"    =>'20',
                        "uid"       =>$v['uid'],
                        "touid"     =>$v['uid'],
                        "giftid"    =>$game['id'],
                        "giftcount" =>1,
                        "totalcoin" =>$v['total'],
                        "showid"    =>0,
                        "addtime"   =>$nowtime
                    );
					Db::name("user_coinrecord")->insert($insert);
				}

				Db::name("game")
					->where("id={$game['id']}")
					->update(array('state' =>'3','endtime' => $nowtime ) );

				$brandToken=$stream."_".$game["action"]."_".$game['starttime']."_Game";
				delcache($brandToken);

			}

			//主播直播奖励---每日任务
			$key='open_live_daily_tasks_'.$uid;
			$starttime=getcaches($key);
			if($starttime){ 
				$endtime=$nowtime;  //当前时间
				$data=[
					'type'=>'3',
					'starttime'=>$starttime,
					'endtime'=>$endtime,
				];
				dailyTasks($uid,$data);
				//删除当前存入的时间
				delcache($key);
			}


			$configpri=getConfigPri();
			$cdn_switch = $configpri['cdn_switch'];

			//声网，销毁云端播放器
			//https://doc.shengwang.cn/api-ref/media-pull/restful/media-pull/operations/delete-region-v1-projects-appId-cloud-player-players-id
			if($cdn_switch==2 && $sw_player_id){

				$sw_appid=$configpri['sw_app_id'];
				$sw_region=$configpri['sw_media_region'];
				if($sw_region=='cn'){
					$sw_request_url='api.sd-rtn.com';
				}else{
					$sw_request_url='api.agora.io';
				}
				$sw_url="https://{$sw_request_url}/{$sw_region}/v1/projects/{$sw_appid}/cloud-player/players/{$sw_player_id}";

				$authorization = getSwHttpAuthorization();

				$sw_header=[
					"Authorization"=>$authorization,
					"Content-Type"=>"application/json"
				];

				$sw_params=[
					'appId'=>$sw_appid,
					'id'=>$sw_player_id,
					'region'=>$sw_region
				];

				swCurlDelete($sw_url,$sw_header,$sw_params);

			}

		}
        
        echo json_encode($rs);
        
    }
    
	/*
	用户列表弹出信息
		uid_admin 50本房间主播 60超管 40管理员 30观众
		touid_admin 50本房间主播 60超管 40管理员 30观众
	*/
	public function popupInfo(){
		$uid=(int)session("uid");
        
        $data = $this->request->param();
        $touid= $data['touid'] ?? '';
        $roomid= $data['roomid'] ?? '';
        
        $touid=(int)checkNull($touid);
        $roomid=(int)checkNull($roomid);
        
        $where['id']=$touid;
        
		$info=Db::name("user")
                ->field("id,avatar,avatar_thumb,user_nickname")
                ->where($where)
                ->find();
		if($uid>0 &&$uid!=null)
		{
			$uid_admin=getIsAdmin($uid,$roomid);
			$isBlack=isBlack($uid,$touid);
		}
		else
		{
			$uid_admin=10;
			$isBlack="";
		}
		if($touid>0 &&$touid!=null)
		{
			$touid_admin=getIsAdmin($touid,$roomid);
		}
		else
		{
			$touid_admin=10;
		}
		
        $info['avatar']=get_upload_path($info['avatar']);
        $info['avatar_thumb']=get_upload_path($info['avatar_thumb']);
        
		$popupInfo=array(
			"state"=>"0",
			"uid_admin"=>$uid_admin,
			"touid_admin"=>$touid_admin,
			"info"=>$info,
			"isBlack"=>$isBlack
		);
		$popupInfo=json_encode($popupInfo);
		echo $popupInfo;
		
	}
	
	/* 直播信息 */
	public function live(){

        $uid = $this->request->param('uid', 0, 'intval');
        
        
        $where['uid']=$uid;

        $where=[
        	'uid'=>$uid
        ];

        $configpri=getConfigPri();
        $cdn_switch=$configpri['cdn_switch'];
        if($cdn_switch==1){
        	$where['islive']=1;
        }
        
		$liveinfo=Db::name("live")->where($where)->find();

        if($liveinfo){

            if($liveinfo['isvideo']==0){
                $liveinfo['pull']=PrivateKeyA('http',$liveinfo['stream'].'.flv',0);
            }
        }
		$data=array(
			'error'=>0,
			'data'=>$liveinfo,
			'msg'=>'',
		);
		echo json_encode($data);
		
	}

	/* 排行榜 */
	public function rank(){
        
        $data = $this->request->param();
        $touid= $data['touid'] ?? '';
        $showid= $data['showid'] ?? '';
        
        $touid=(int)checkNull($touid);
        $showid=(int)checkNull($showid);
        
		$list=array();

		if(!$touid){
			echo json_encode($list);
            return;
		}
		
		//本房间魅力榜
        
		$now=Db::name("user_voterecord")
                ->field("fromid as uid,sum(total) as total")
                ->where("action in (1,2) and uid='{$touid}'")
                ->group("fromid")
                ->order("total desc")
                ->limit(0,10)
                ->select()
                ->toArray();
		foreach($now as $k=>$v){
			$userinfo=getUserInfo($v['uid']);
			$now[$k]['userinfo']=$userinfo;
			$now[$k]['total']=NumberFormat($v['total']);
		}	
		$list['now']=$now;
		//全站魅力榜
		$all=Db::name("user_voterecord")
                ->field("fromid as uid,sum(total) as total")
                ->where("action in (1,2)")
                ->group("fromid")
                ->order("total desc")
                ->limit(0,10)
                ->select()
                ->toArray();
		foreach($all as $k=>$v){
			$userinfo=getUserInfo($v['uid']);
			$all[$k]['userinfo']=$userinfo;
			$all[$k]['total']=NumberFormat($v['total']);
		}	
		$list['all']=$all;		
		echo json_encode($list);
		
	}

	/*进入直播间检查房间类型*/
	public function checkLive()
	{
		$configpub=getConfigPub();
		$rs = array('code' => 0, 'msg' => '', 'info' => array());
		$uid=(int)session("uid");
        
        $data = $this->request->param();
        $liveuid= $data['liveuid'] ?? '';
        $stream= $data['stream'] ?? '';
        
        $liveuid=(int)checkNull($liveuid);
        $stream=checkNull($stream);
        
		$rs['land']=0;
        
        $where['uid']=$liveuid;
        $where['stream']=$stream;
        
		$islive=Db::name('live')
                ->field("islive,type,type_val,starttime")
                ->where($where)
                ->find();
		if($islive['type']==2)
		{
			if($uid>0){
				$rs['land']=0;
			}else{
				$rs['land']=1;
				$rs['type']=$islive['type'];
				$rs['type_msg']='当前房间为付费房间，请先登陆';
				echo json_encode($rs);
                return;
			}
		}
		if(!$islive ||$islive['islive']==0)
		{
			$rs['code'] = 1005;
			$rs['land']=1;
			$rs['msg'] = '直播已结束，请刷新';
			echo json_encode($rs);
            return;
		}

			$rs['type']=$islive['type'];
			$rs['type_msg']='';
			$rs['type_value']=$islive['type_val'];
			if($uid>0){
				$userinfo=Db::name("user")
						->field("issuper")
						->where("id={$uid}")
						->find();
				if($userinfo && $userinfo['issuper']==1){
					
					if($islive['type']==6){
						
						$rs['code'] = 1007;
						$rs['land']=1;
						$rs['msg'] = '超管不能进入1v1房间';
						echo json_encode($rs);
                        return;
					}
					$rs['type']='0';
					$rs['type_val']='0';
					$rs['type_msg']='';
					
					echo json_encode($rs);
                    return;
				}
			}

			
			if($islive['type']==1){
				$rs['type_msg']='本房间为密码房间，请输入密码';
				$rs['type_value']=md5($islive['type_val']);
			}
			else if($islive['type']==2)
			{
                $where2['uid']=$uid;
                $where2['touid']=$liveuid;
                $where2['showid']=$islive['starttime'];
                $where2['action']='6';
                $where2['type']='0';
				$rs['type_msg']='本房间为收费房间，需支付'.$islive['type_val'].$configpub['name_coin'];
				$isexist=Db::name("user_coinrecord")->field('id')->where($where2)->find();
				if($isexist)
				{
					$rs['type']='0';
					$rs['type_msg']='';
				}
			}
			else if($islive['type']==3)
			{
				$rs['type_msg']='本房间为计时房间，每分钟需支付'.$islive['type_val'].$configpub['name_coin'];
			}

		echo  json_encode($rs);
		
	}

	/* 直播/回放 结束后推荐 */
	public function endRecommend(){
		/* 推荐列表 */
			
		$list=Db::name("live")->where("islive='1'")->orderRaw("rand()")->limit(0,4)->select()->toArray();
		foreach($list as $k=>$v){
			$list[$k]['userinfo']=getUserInfo($v['uid']);
		}
		$data=array(
				'error'=>0,
				'data'=>$list,
		 );
		echo  json_encode($data);
		
	}

	/* 关注 */
	public function attention(){
		$uid=(int)session("uid");
		if($uid <1){
			$data=array(
				'error'=> 1,
				'msg'=> "请登录",
				'data'=> "请登录",
			);	
			
		}else{

			//判断用户的状态
			$userinfo=getUserInfo($uid);
			if($userinfo['user_status']==0){
				session('uid',null);		
	            session('token',null);
	            session('user',null);
	            cookie('uid',null);
	            cookie('token',null);
	            $data=array(
					'error'=> 700,
					'msg'=> "该账号已被拉黑",
					'data'=> "",
				);
				echo json_encode($data);
                return;
			}

			if($userinfo['end_bantime']>time()){
				session('uid',null);		
	            session('token',null);
	            session('user',null);
	            cookie('uid',null);
	            cookie('token',null);
				$data=array(
					'error'=> 700,
					'msg'=> "该账号已被禁用",
					'data'=> "",
				);
				echo json_encode($data);
                return;
			}


            $anchorid = $this->request->param('roomnum', 0, 'intval');
			if($uid == $anchorid){
				$data=array(
					'error'=> 1,
					'msg'=> "不能关注自己",
					'data'=> "不能关注自己"
				);						
			}else{
				$add=array(
					'uid'	=>	$uid,
					'touid'	=>	$anchorid
				);			
				if(isAttention($uid,$anchorid)){
					$check=DB::name('user_attention')->where($add)->delete();
					if($check !== false){
						$data=array(
							'error'=> 0,
							'msg'=> "+关注",
							'data'=> getFansnums($anchorid)
						);	
					}else{
						$data=array(
							'error'=> 1,
							'data'=> '',
							'msg'=> "取消关注失败",
						);					
					}
				}else{
					$check=DB::name('user_attention')->insert(array("uid"=>$uid,"touid"=>$anchorid,"addtime"=>time()));
					$black=DB::name('user_black')->where($add)->delete();
					if($check !== false){
						$data=array(
							'error'=> 0,
							'msg'=> "已关注",
							'data'=> getFansnums($anchorid)
						);
					}else{
						$data=array(
							'error'=> 1,
							'msg'=> "关注失败",
							'data'=> "",
						);					
					}	
				}
			}
		}
		echo  json_encode($data);
		
	}
	/*主播页面特殊直播弹窗*/
	public function selectplay()
	{
	    $config=getConfigPub();
	    $this->assign("config",$config);
        $live_class=Db::name('live_class')->order("list_order asc")->select()->toArray();
        $this->assign("live_class",$live_class);
	    return $this->fetch();
	}
	
	/*切换费用弹窗*/
	public function selectplay2()
	{
		$config=getConfigPub();
		$this->assign("config",$config);
		return $this->fetch();
	}
	public function getUserList(){
        
        $data = $this->request->param();
        $showid= $data['showid'] ?? '';
        $p= $data['p'] ?? '';
        
        $showid=(int)checkNull($showid);
        $p=(int)checkNull($p);

        $where['islive']=1;
        $where['uid']=$showid;
		$live=Db::name("live")->where($where)->find();
		if($live){
			$stream=$live['stream'];
		}else
		{
			$stream=$showid."_".$showid;
		}

		$nums=zSize('user_'.$stream);
        
        if($p<1){
            $p=1;
        }
		$pnum=20;
		$start=($p-1)*$pnum;
        
		/*$key="getUserLists_".$showid.'_'.$p;
		$list=getcaches($key);
		
		if(!$list){ */
            $list=array();

            $uidlist=zRevRange('user_'.$stream,$start,$pnum,true);
            
            foreach($uidlist as $k=>$v){
                $userinfo=getUserInfo($k);
                $info=explode(".",$v);
                $userinfo['contribution']=(string)$info[0];
                $list[]=$userinfo;
            }
            
            // if($list){
            //     setcaches($key,$list,50);
            // }
		//}
        
        if(!$list){
            $list=array();
        }

        
		$data['list']=$list;
		$data['nums']=$nums;
		echo  json_encode($data);	
		
	} 	

	/* 切换直播类型 */
	public function changeTypeValue() {
		$rs = array('code' => 0, 'msg' => '', 'info' => array());
		
		$uid=(int)session("uid");
        $data = $this->request->param();
        $type= $data['type'] ?? '';
        $type_value= $data['type_value'] ?? '';
        
        $type=checkNull($type);
        $type_value=checkNull($type_value);

        if($uid<1){
        	session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            $rs['code']=700;
			$rs['msg']='您的登陆状态失效，请重新登陆！';
			echo  json_encode($rs);
            return;
        }

		$result['type_value']=$type_value;
		$result['type']=(string)$type;
		$data=array(
			'type'=>$type,
			'type_val'=>$type_value,
		);
        
		$liveinfo=Db::name("live")->field('type')->where("uid={$uid} and islive=1")->find();
		if(!$liveinfo){
			$rs['code'] = 1001;
			$rs['msg'] = '直播信息不存在';
			echo json_encode($rs);
            return;
		}

		$result2=Db::name("live")->where("uid={$uid}")->update($data);
		if($result2===false){
			$rs['code'] = 1003;
			$rs['msg'] = '开启收费房间失败，请重试';
			echo json_encode($rs);
            return;
		}

		$rs['info']=$result;
		echo json_encode($rs);
		
	}

	//超管关播
	public function superStopRoom(){
		$rs = array('code' => 0, 'msg' => '关闭成功', 'info' => array());
		$data=$this->request->param();
		$uid=checkNull($data['uid']);
		$token=checkNull($data['token']);
		$liveuid=checkNull($data['liveuid']);
		$type=checkNull($data['type']);

		$checkToken=checkToken($uid,$token);
		if($checkToken==700){
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

		//判断用户是否为超管
		$issuper=isSuper($uid);
		if(!$issuper){
			$rs['code'] = 1001;
            $rs['msg'] = '你不是超管，无权操作';
            echo json_encode($rs);
            return;
		}

		if($type==1){
			//禁播列表
			$isexist=Db::name("live_ban")->where("liveuid={$liveuid}")->find();
			if($isexist){
				$rs['code'] = 1002;
            	$rs['msg'] = '该主播已被禁播';
            	echo json_encode($rs);
                return;
			}

			Db::name("live_ban")->insert(['liveuid'=>$liveuid,'superid'=>$uid,'addtime'=>time(),'type'=>'all']);
		}

		$info=Db::name("live")->field("stream")->where("uid={$liveuid} and islive=1")->find();
		if($info){
			$this->superStopRoomHandle($liveuid,$info['stream']);
		}

		echo json_encode($rs);
	}

	private function superStopRoomHandle($liveuid,$stream){
		

		$info=Db::name("live")
			->field("uid,showid,starttime,title,province,city,stream,lng,lat,type,type_val,liveclassid,live_type")
			->where("uid={$liveuid} and stream='{$stream}' and islive=1")
			->find();

		if($info){
			$isdel=Db::name("live")->where("uid={$liveuid}")->delete();
			if(!$isdel){
				$rs['code'] = 1002;
            	$rs['msg'] = '关播失败';
            	echo json_encode($rs);
                return;
			}

			$nowtime=time();
			$info['endtime']=$nowtime;
			$info['time']=date("Y-m-d",$info['showid']);
			$votes=Db::name("user_voterecord")
				->where("uid ={$liveuid} and showid={$info['showid']}")
				->sum('total');
			$info['votes']=0;
			if($votes){
				$info['votes']=$votes;
			}
			$nums=zSize('user_'.$stream);
			hDel("livelist",$liveuid);
			delcache($liveuid.'_zombie');
			delcache($liveuid.'_zombie_uid');
			delcache('attention_'.$liveuid);
			delcache('user_'.$stream);
			$info['nums']=$nums;
			$result=Db::name("live_record")->insert($info);

			//解除本场禁言
			$list2=Db::name("live_shut")
                ->field('uid')
                ->where("liveuid={$liveuid} and showid!=0")
                ->select();

            Db::name("live_shut")->where("liveuid={$liveuid} and showid!=0")->delete();

            foreach($list2 as $k=>$v){
                hDel($liveuid . 'shutup',$v['uid']);
            }

            //游戏处理
            $game=Db::name("game")
				->where("stream='{$stream}' and liveuid={$liveuid} and state=0")
				->find();

			$total=array();
			if($game){
				$total=Db::name("gamerecord")
					->field("uid,sum(coin_1 + coin_2 + coin_3 + coin_4 + coin_5 + coin_6) as total")
					->where("gameid={$game['id']}")
					->group('uid')
					->select();

				foreach($total as $k=>$v){
					Db::name("user")
						->where("id={$v['uid']}")
						->inc("coin",$v['total'])
                        ->update();
					
					$insert=array(
                        "type"=>'1',
                        "action"=>'20',
                        "uid"=>$v['uid'],
                        "touid"=>$v['uid'],
                        "giftid"=>$game['id'],
                        "giftcount"=>1,
                        "totalcoin"=>$v['total'],
                        "showid"=>0,
                        "addtime"=>$nowtime
                    );
					Db::name("user_coinrecord")->insert($insert);
				}

				Db::name("game")
					->where("id={$game['id']}")
					->update(array('state' =>'3','endtime' => $nowtime ) );

				$brandToken=$stream."_".$game["action"]."_".$game['starttime']."_Game";
				delcache($brandToken);
			}

			//主播直播奖励---每日任务
			$key='open_live_daily_tasks_'.$liveuid;
			$starttime=getcaches($key);
			if($starttime){ 
				$endtime=$nowtime;  //当前时间
				$data=[
					'type'=>'3',
					'starttime'=>$starttime,
					'endtime'=>$endtime,
				];
				dailyTasks($liveuid,$data);
				//删除当前存入的时间
				delcache($key);
			}
		}

	}

	public function reloadLive(){
		$rs=array('code'=>0,'msg'=>'','info'=>array());
		$data=$this->request->param();
		$stream=checkNull($data['stream']);
		$live_info=Db::name("live")->where(['stream'=>$stream])->find();
		if($live_info){
			if($live_info['isvideo']==1){
				$url=$live_info['pull'];
			}else{
				$url=PrivateKeyA('http',$live_info['stream'].'.flv',0);
			}

			$rs['info'][0]['pull']=$url;

			echo json_encode($rs);
			
		}else{
			$rs['code']=1001;
			$rs['msg']='直播中断,看看其他直播吧';
			echo json_encode($rs);
		}
		
	}

	public function getLinkMicSwRtcToken(){
		$rs=array('code'=>0,'msg'=>'','info'=>[]);
		$data=$this->request->param();
		$uid=$data['uid'];
		$stream=$data['stream'];
		if(!$uid || !$stream){
			$rs['code']=1001;
			$rs['msg']='参数错误';
			echo json_encode($rs);
			return;
		}

		$token = getShengWangRtcToken($uid,$stream);

		$rs['info'][0]['token']=$token;

		echo json_encode($rs);
	}			
}
