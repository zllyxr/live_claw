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
 * 消费相关
 */
class SpendController extends HomebaseController {
	/* 送礼物 */
	public function sendGift(){
		$uid=(int)session("uid");
        
        $data = $this->request->param();
        $touid= $data['touid'] ?? '';
        $giftid= $data['giftid'] ?? '';
        
        $touid=(int)checkNull($touid);
        $giftid=(int)checkNull($giftid);
        

		$giftcount=1;
        if($uid<1){
            session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            echo '{"errno":"1000","data":"","msg":"您的登陆状态失效，请重新登陆！"}';
            return;
        }
        if($touid<1){
            echo '{"errno":"1000","data":"","msg":"参数错误"}';
            return;
        }
		if($uid==$touid)
		{
			echo '{"errno":"1000","data":"","msg":"不允许给自己送礼物"}';
            return;
		}
        $where['user_id']=$uid;
		$userinfo= Db::name("user_token")->field('token,expire_time')->where($where)->find();
        
		if($userinfo['token']!=session("token") || $userinfo['expire_time']<time()){
            session('uid',null);		
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
			echo '{"errno":"700","data":"","msg":"您的登陆状态失效，请重新登陆！"}';
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
            echo '{"errno":"700","data":"","msg":"该账号已被拉黑"}';
            return;
        }

        if($userinfo['end_bantime']>time()){
            session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            echo '{"errno":"700","data":"","msg":"该账号已被禁用"}';
            return;
        }
        
		/* 礼物信息 */
        $where2['id']=$giftid;

        //语言包
		$giftinfo=Db::name("gift")
            ->field("giftname,giftname_en,gifticon,needcoin,type,mark,swftype,swf,swftime,sticker_id,isplatgift")
            ->where($where2)
            ->find();
		
		if($giftinfo['type']==3){
			echo '{"errno":"1001","data":"","msg":"此礼物为手绘礼物,请前去下载app体验~"}';
            return;
		}

		$total= $giftinfo['needcoin']*$giftcount;
		$addtime=time();
        
        if($giftinfo['mark']==2){
            /* 守护 */
            $guard_info=getUserGuard($uid,$touid);
            if($guard_info['type']!=1){
                echo '{"errno":"1002","data":"","msg":"该礼物是守护专属礼物奥~"}';
                return;
            }
        }

        $where3['uid']=$touid;
		$liveinfo=Db::name("live")->where("islive=1")->where($where3)->find();
		$showid=0;
		if($liveinfo){
			$showid=$liveinfo['starttime'];
		}
		/* 更新用户余额 消费 */
		$ifok=Db::name("user")
                ->where([['id','=',$uid],['coin','>=',$total]])
                ->dec('coin',$total)
                ->inc('consumption',$total)
                ->update();
		if(!$ifok){
            /* 余额不足 */
			echo '{"errno":"1001","data":"","msg":"余额不足"}';
            return;
        }
		

        $anthor_total=$total;
        /* 幸运礼物分成 */
        if($giftinfo['type']==0 && $giftinfo['mark']==3){
            $jackpotset=getJackpotSet();
            if($jackpotset['switch']=='1'){
                $anthor_total=floor($anthor_total*$jackpotset['luck_anchor']*0.01*100)/100;
            }
            
        }
        
        /* 幸运礼物分成 */
        
		/* 家族分成之后的金额 */
        if($anthor_total){
            $anthor_total=setFamilyDivide($touid,$anthor_total);  
        }

		/* 更新直播 星币 累计星币 */
        Db::name("user")
                ->where([['id','=',$touid]])
                ->inc('coin',$anthor_total)
                ->inc('votestotal',$total)
                ->update();
        
        if($anthor_total){
            $insert_votes=[
                'type'=>'1',
                'action'=>'1',
                'uid'=>$touid,
                'fromid'=>$uid,
                'actionid'=>$giftid,
                'nums'=>$giftcount,
                'total'=>$total,
                'showid'=>$showid,
                'votes'=>$anthor_total,
                'addtime'=>time(),
            ];
            Db::name('user_voterecord')->insert($insert_votes); 
        }
        
		/* 更新直播 星币 累计星币 */
		Db::name("user_coinrecord")
            ->insert(
                array(
                    "type"      =>'0',
                    "action"    =>'1',
                    "uid"       =>$uid,
                    "touid"     =>$touid,
                    "giftid"    =>$giftid,
                    "giftcount" =>$giftcount,
                    "totalcoin" =>$total,
                    "showid"    =>$showid,
                    "mark"      =>$giftinfo['mark'],
                    "addtime"   =>$addtime
                )
            );
        
        /* 更新主播热门 */
        if($giftinfo['mark']==1){
            
            Db::name('live')
                ->where('uid','=',$touid)
                ->inc('hotvotes',$total)
                ->update();
        }
        
        
        /* PK处理 */
        $key1='LivePK';
        $key2='LivePK_gift';
        
        $ispk='0';
        $pkuid1='0';
        $pkuid2='0';
        $pktotal1='0';
        $pktotal2='0';
        $liveuid=$touid;
        $pkuid= hGet($key1,$liveuid);
        if($pkuid){
            $ispk='1';
            hIncrBy($key2,$liveuid,$total);
            
            $gift_uid=hGet($key2,$liveuid);
            $gift_pkuid=hGet($key2,$pkuid);
            
            $pktotal1=$gift_uid;
            $pktotal2=$gift_pkuid;
            
            $pkuid1=$liveuid;
            $pkuid2=$pkuid;
            
        }
        
        
		/* 清除缓存 */
		delCache("userinfo_".$uid); 
		delCache("userinfo_".$touid); 
        
        $userinfo3=Db::name('user')
            ->field("votestotal,user_nickname")
            ->where("id='{$touid}'")
            ->find();
		$gifttoken=md5(md5('sendGift'.$uid.$touid.$giftid.$giftcount.$total.$showid.$addtime));
        $swf=$giftinfo['swf'] ? get_upload_path($giftinfo['swf']):'';

        $ifluck=0;
        $ifup=0;
        $ifwin=0;
        /* 幸运礼物 */
        if($giftinfo['type']==0 && $giftinfo['mark']==3){
            $ifup=1;
            $ifwin=1;
            $list=getLuckRate();
            /* 有中奖配置 才处理 */
            if($list){
                $rateinfo=[];
                foreach($list as $k=>$v){
                    if($v['giftid']==$giftid && $v['nums']==$giftcount){
                        $rateinfo[]=$v;
                    }
                }
                /* 有该礼物、该数量 中奖配置 才处理 */
                if($rateinfo){
                    $ifluck=1;
                }
            }
            
        }
//        file_put_contents(CMF_ROOT.'log/think/home/spend/sendgift_'.date('y-m-d').'.txt',date('Y-m-d H:i:s').' 提交参数信息 ifluck:'.json_encode($ifluck)."\r\n",FILE_APPEND);
//        file_put_contents(CMF_ROOT.'log/think/home/spend/sendgift_'.date('y-m-d').'.txt',date('Y-m-d H:i:s').' 提交参数信息 ifwin:'.json_encode($ifwin)."\r\n",FILE_APPEND);
//        file_put_contents(CMF_ROOT.'log/think/home/spend/sendgift_'.date('y-m-d').'.txt',date('Y-m-d H:i:s').' 提交参数信息 ifup:'.json_encode($ifup)."\r\n",FILE_APPEND);
//        file_put_contents(CMF_ROOT.'log/think/home/spend/sendgift_'.date('y-m-d').'.txt',date('Y-m-d H:i:s').' 提交参数信息 rateinfo:'.json_encode($rateinfo)."\r\n",FILE_APPEND);
        /* 幸运礼物中奖 */
        $isluck='0';
        $isluckall='0';
        $luckcoin='0';
        $lucktimes='0';
        if($ifluck ==1 ){
            $luckrate=rand(1,100000);
            //file_put_contents(CMF_ROOT.'log/think/home/spend/sendgift_'.date('y-m-d').'.txt',date('Y-m-d H:i:s').' 提交参数信息 luckrate:'.json_encode($luckrate)."\r\n",FILE_APPEND);
            $rate=0;
            foreach($rateinfo as $k=>$v){
                $rate+=floor($v['rate']*1000);
                //file_put_contents(CMF_ROOT.'log/think/home/spend/sendgift_'.date('y-m-d').'.txt',date('Y-m-d H:i:s').' 提交参数信息 rate:'.json_encode($rate)."\r\n",FILE_APPEND);
                if($luckrate <= $rate){
                    /* 中奖 */
                    $isluck='1';
                    $isluckall=$v['isall'];
                    $lucktimes=$v['times'];
                    $luckcoin= $total * $lucktimes;
                    
                    /* 用户加余额  写记录 */
                    Db::name('user')
                        ->where('id','=',$uid)
                        ->inc('coin',$luckcoin)
                        ->update();
                    
                    $insert=array(
                        "type"      =>'1',
                        "action"    =>'12',
                        "uid"       =>$uid,
                        "touid"     =>$uid,
                        "giftid"    =>$giftid,
                        "giftcount" =>$lucktimes,
                        "totalcoin" =>$luckcoin,
                        "showid"    =>$showid,
                        "mark"      =>$giftinfo['mark'],
                        "addtime"   =>$addtime
                    );
                    Db::name('user_coinrecord')->insert($insert);
                    break;
                }
            }
        }
        
        /* 幸运礼物中奖 */
        
        
        /* 奖池升级 */
        $isup='0';
        $uplevel='0';
        $upcoin='0';
        if($ifup == 1 ){
            //file_put_contents(CMF_ROOT.'log/think/home/spend/sendgift_'.date('y-m-d').'.txt',date('Y-m-d H:i:s').' 提交参数信息 ifup:'.json_encode($ifup)."\r\n",FILE_APPEND);
            //file_put_contents(CMF_ROOT.'log/think/home/spend/sendgift_'.date('y-m-d').'.txt',date('Y-m-d H:i:s').' 提交参数信息 jackpotset:'.json_encode($jackpotset)."\r\n",FILE_APPEND);
            if($jackpotset['switch']==1 && $jackpotset['luck_jackpot'] > 0){
                /* 开启奖池 */
                $jackpot_up=floor($total * $jackpotset['luck_jackpot'] * 0.01);
                
                //file_put_contents(CMF_ROOT.'log/think/home/spend/sendgift_'.date('y-m-d').'.txt',date('Y-m-d H:i:s').' 提交参数信息 jackpot_up:'.json_encode($jackpot_up)."\r\n",FILE_APPEND);
                if($jackpot_up){
                    
                    Db::name('jackpot')
                        ->where('id','=',1)
                        ->inc('total',$jackpot_up)
                        ->update();
                    
                    $jackpotinfo=getJackpotInfo();
                    
                    $jackpot_level=getJackpotLevel($jackpotinfo['total']);
                    //file_put_contents(CMF_ROOT.'log/think/home/spend/sendgift_'.date('y-m-d').'.txt',date('Y-m-d H:i:s').' 提交参数信息 jackpotinfo:'.json_encode($jackpotinfo)."\r\n",FILE_APPEND);
                    //file_put_contents(CMF_ROOT.'log/think/home/spend/sendgift_'.date('y-m-d').'.txt',date('Y-m-d H:i:s').' 提交参数信息 jackpot_level:'.json_encode($jackpot_level)."\r\n",FILE_APPEND);
                    if($jackpot_level>$jackpotinfo['level']){
                        $isok=Db::name('jackpot')
                            ->where("id = 1 and level < {$jackpot_level}")
                            ->update( array('level' => $jackpot_level ));
                        
                        //file_put_contents(CMF_ROOT.'log/think/home/spend/sendgift_'.date('y-m-d').'.txt',date('Y-m-d H:i:s').' 提交参数信息 isok:'.json_encode($isok)."\r\n",FILE_APPEND);
                        if($isok){
                            //file_put_contents(CMF_ROOT.'log/think/home/spend/sendgift_'.date('y-m-d').'.txt',date('Y-m-d H:i:s').' 提交参数信息 isup:'.json_encode($isup)."\r\n",FILE_APPEND);
                            $isup='1';
                            $uplevel=$jackpot_level;
                        }
                    }
                }
            }
        }
        /* 奖池升级 */
        
        /* 奖池中奖 */
        $iswin='0';
        $wincoin='0';
        if($ifwin ==1 ){
            if($jackpotset['switch']==1 ){
               /* 奖池开启 */
               $jackpotinfo=getJackpotInfo();
               //file_put_contents(CMF_ROOT.'log/think/home/spend/sendgift_'.date('y-m-d').'.txt',date('Y-m-d H:i:s').' 提交参数信息 jackpotinfo:'.json_encode($jackpotinfo)."\r\n",FILE_APPEND);
               if($jackpotinfo['level']>=1){
                    /* 至少达到第一阶段才能中奖 */
                    
                    $list=getJackpotRate();
                    //file_put_contents(CMF_ROOT.'log/think/home/spend/sendgift_'.date('y-m-d').'.txt',date('Y-m-d H:i:s').' 提交参数信息 list:'.json_encode($list)."\r\n",FILE_APPEND);
                    /* 有奖池中奖配置 才处理 */
                    if($list){
                        $rateinfo=[];
                        foreach($list as $k=>$v){
                            if($v['giftid']==$giftid && $v['nums']==$giftcount){
                                $rateinfo=$v;
                                break;
                            }
                        }
                        //file_put_contents(CMF_ROOT.'log/think/home/spend/sendgift_'.date('y-m-d').'.txt',date('Y-m-d H:i:s').' 提交参数信息 rateinfo:'.json_encode($rateinfo)."\r\n",FILE_APPEND);
                        /* 有该礼物中奖配置 才处理 */
                        if($rateinfo){
                            $winrate=rand(1,100000);
                            
                            $rate_jackpot=json_decode($rateinfo['rate_jackpot'],true);
                            
                            $rate=floor($rate_jackpot[$jackpotinfo['level']] * 1000);
                            //file_put_contents(CMF_ROOT.'log/think/home/spend/sendgift_'.date('y-m-d').'.txt',date('Y-m-d H:i:s').' 提交参数信息 winrate:'.json_encode($winrate)."\r\n",FILE_APPEND);
                            //file_put_contents(CMF_ROOT.'log/think/home/spend/sendgift_'.date('y-m-d').'.txt',date('Y-m-d H:i:s').' 提交参数信息 rate:'.json_encode($rate)."\r\n",FILE_APPEND);
                            if($winrate <= $rate){
                                /* 中奖 */
                                $wincoin2=$jackpotinfo['total'];
                                
                                $isok=Db::name('jackpot')
                                        ->where([['id','=','1'],['total','>=',$wincoin2]])
                                        ->dec('total',$wincoin2)
                                        ->update(['level'=>0]);
                                
                                
                                if($isok){
                                    //file_put_contents(CMF_ROOT.'log/think/home/spend/sendgift_'.date('y-m-d').'.txt',date('Y-m-d H:i:s').' 提交参数信息 iswin:'.'1'."\r\n",FILE_APPEND);
                                    $iswin='1';
                                    $wincoin=(string)$wincoin2;
                                    
                                    /* 用户加余额  写记录 */
                                    Db::name('user')
                                        ->where('id','=',$uid)
                                        ->inc('coin',$wincoin2)
                                        ->update();
                                    
                                    $insert=array(
                                        "type"      =>'1',
                                        "action"    =>'13',
                                        "uid"       =>$uid,
                                        "touid"     =>$uid,
                                        "giftid"    =>'0',
                                        "giftcount" =>'1',
                                        "totalcoin" =>$wincoin2,
                                        //"showid"  =>$showid,
                                        "mark"      =>$giftinfo['mark'],
                                        "addtime"   =>$addtime
                                    );
                                    Db::name('user_coinrecord')->insert($insert);
                                }
                            }
                        }
                    }
               }
            }
        }
        /* 奖池中奖 */
        
        $userinfo2=Db::name('user')->field("consumption,coin,votestotal")->where("id='{$uid}'")->find();
		$level=getLevel($userinfo2['consumption']);
        if($giftinfo['type']!=1){
            $giftinfo['isplatgift']='0';
        }
        
		$result=array(
            "uid"           =>(int)$uid,
            "giftid"        =>(int)$giftid,
            "type"          =>(string)$giftinfo['type'],
            "mark"          =>(string)$giftinfo['mark'],
            "giftcount"     =>(int)$giftcount,
            "totalcoin"     =>$total,
            "giftname"      =>$giftinfo['giftname'],
            "giftname_en"   =>$giftinfo['giftname_en'],
            "gifticon"      =>get_upload_path($giftinfo['gifticon']),
            "swftype"       =>$giftinfo['swftype'],
            "swftime"       =>$giftinfo['swftime'],
            "swf"           =>$swf,
            "level"         =>$level,
            "votestotal"    =>$userinfo3['votestotal'],
            "isluck"        =>$isluck,
            "isluckall"     =>$isluckall,
            "luckcoin"      =>$luckcoin,
            "lucktimes"     =>$lucktimes,
            "isup"          =>$isup,
            "uplevel"       =>$uplevel,
            "upcoin"        =>$upcoin,
            "iswin"         =>$iswin,
            "wincoin"       =>$wincoin,
            "ispk"          =>$ispk,
            "pkuid"         =>$pkuid,
            "pkuid1"        =>$pkuid1,
            "pkuid2"        =>$pkuid2,
            "pktotal1"      =>$pktotal1,
            "pktotal2"      =>$pktotal2,
            "isplatgift"    =>$giftinfo['isplatgift'],
			"sticker_id"    =>(string)$giftinfo['sticker_id'],
        );
        
		$sendgift_arr[]=$result;
		setcaches($gifttoken,$sendgift_arr);
        if($liveinfo){
            zIncrBy('user_'.$liveinfo['stream'],$total,$uid);
        }

		echo '{"errno":"0","uid":"'.$uid.'","level":"'.$level.'","type":"'.$giftinfo['type'].'","coin":"'.$userinfo2['coin'].'","gifttoken":"'.$gifttoken.'","livename":"'.$userinfo3['user_nickname'].'","msg":"赠送成功"}';

	}		
	/* 弹幕 */
	public function sendHorn(){
		$rs = array('code' => 0, 'msg' => '', 'info' =>array());
		
		$uid=(int)session("uid");
        
        $data = $this->request->param();
        $liveuid= $data['liveuid'] ?? '';
        $content= $data['content'] ?? '';
        $stream= $data['stream'] ?? '';
        $liveuid=(int)checkNull($liveuid);
        $content=checkNull($content);
        $stream=checkNull($stream);

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
        
        if($liveuid<1){
            $rs['code']=1000;
			$rs['msg']='参数错误';
			echo  json_encode($rs);
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
            $rs['code']=700;
            $rs['msg']='该账号已被拉黑';
            echo  json_encode($rs);
            return;
        }

        if($userinfo['end_bantime']>time()){
            session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            $rs['code']=700;
            $rs['msg']='该账号已被禁用';
            echo  json_encode($rs);
            return;
        }

		$configpri=getConfigPri();
		$giftid=0;
		$giftcount=1;
		$giftinfo=array(
			"giftname"=>'弹幕',
			"gifticon"=>'',
			"needcoin"=>$configpri['barrage_fee'],
		);		
		
		$total= $giftinfo['needcoin']*$giftcount;
		 
		$addtime=time();
		$type='0';
		$action='2';

		/* 更新用户余额 消费 */
		
        $ifok=Db::name("user")
                ->where([['id','=',$uid],['coin','>=',$total]])
                ->dec('coin',$total)
                ->inc('consumption',$total)
                ->update();
        if(!$ifok){
            $rs['code']=1001;
			$rs['msg']='余额不足';
			echo  json_encode($rs);
            return;
        }

		/* 更新直播主播 星币 累计星币 */						 
		
        Db::name("user")
                ->where([['id','=',$liveuid]])
                ->inc('coin',$total)
                ->inc('votestotal',$total)
                ->update();
        
        $stream2=explode('_',$stream);
		$showid=$stream2[1];
        
        if(!$showid){
            $showid=0;
        }
        
        $insert_votes=[
            'type'      =>'1',
            'action'    =>$action,
            'uid'       =>$liveuid,
            'fromid'    =>$uid,
            'actionid'  =>$giftid,
            'nums'      =>$giftcount,
            'total'     =>$total,
            'showid'    =>$showid,
            'votes'     =>$total,
            'addtime'   =>time(),
        ];
        Db::name('user_voterecord')->insert($insert_votes);

		$insert=array(
            "type"      =>$type,
            "action"    =>$action,
            "uid"       =>$uid,
            "touid"     =>$liveuid,
            "giftid"    =>$giftid,
            "giftcount" =>$giftcount,
            "totalcoin" =>$total,
            "showid"    =>$showid,
            "addtime"   =>$addtime
        );
		$isup=Db::name("user_coinrecord")->insert($insert);
					 
		$userinfo2 =Db::name('user')->field('consumption,coin')->where("id=".$uid)->find();	
		/*获取当前用户的等级*/
		$level=getLevel($userinfo2['consumption']);			
		
		/* 清除缓存 */
		delCache("userinfo_".$uid); 
		delCache("userinfo_".$liveuid); 
		/*获取主播影票*/
		$votestotal=Db::name('user')->field('votestotal,coin')->where("id=".$liveuid)->find();
		
		$barragetoken=md5(md5($action.$uid.$liveuid.$giftid.$giftcount.$total.$showid.$addtime.rand(100,999)));
		 
		$result=array(
            "uid"           =>$uid,
            "content"       =>$content,
            "giftid"        =>$giftid,
            "giftcount"     =>$giftcount,
            "totalcoin"     =>$total,
            "giftname"      =>$giftinfo['giftname'],
            "gifticon"      =>$giftinfo['gifticon'],
            "level"         =>$level,
            "coin"          =>$userinfo2['coin'],
            "votestotal"    =>$votestotal['votestotal'],
            "barragetoken"  =>$barragetoken
        );
		$rs['info']=$result;
		/*写入 redis*/
		unset($result['barragetoken']);
		setcaches($barragetoken,$result);
		echo json_encode($rs);
	
	}
	/*设置 取消 管理员*/
	public function cancel(){

		$rs = array('code' => 0, 'msg' => '', 'info' =>'操作成功');
		$uid=session("uid");
        
        $data = $this->request->param();
        $touid= $data['touid'] ?? '';
        $showid= $data['roomid'] ?? '';
        
        $touid=(int)checkNull($touid);
        $showid=(int)checkNull($showid);
        
        if($uid<1){
            session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            $rs['code']=1000;
			$rs['msg']='您的登陆状态失效，请重新登陆！';
			echo  json_encode($rs);
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
            $rs['code']=700;
            $rs['msg']='该账号已被拉黑';
            echo  json_encode($rs);
            return;
        }

        if($userinfo['end_bantime']>time()){
            session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            $rs['code']=700;
            $rs['msg']='该账号已被禁用';
            echo  json_encode($rs);
            return;
        }
        
        if($touid<1){
            $rs['code']=1000;
			$rs['msg']='参数错误';
			echo  json_encode($rs);
            return;
        }
        
		if($uid!=$showid)
		{
			$rs['code']=1001;
			$rs['msg']='不是该房间主播';
			echo  json_encode($rs);
            return;
		}
		if($uid==$touid)
		{
			$rs['code']=1002;
			$rs['msg']='自己无法管理自己';
			echo  json_encode($rs);
            return;
		}
		$admininfo=Db::name("live_manager")
            ->where("uid='{$touid}' and liveuid='{$showid}'")
            ->find();
        
		$userinfo=Db::name("user")
            ->field("id,avatar,avatar_thumb,user_nickname")
            ->where("id=".$touid)
            ->find();

        $rs['user_nickname']=$userinfo['user_nickname'];
        
		if($admininfo){

			Db::name("live_manager")
                ->where("uid='{$touid}' and liveuid='{$showid}'")
                ->delete();

			$rs['isadmin']=0;	
		}
		else
		{
			$count =Db::name("live_manager")
                ->where("liveuid='{$showid}'")
                ->count();

			if($count>=5){
				$rs['code']=1004;
				$rs['msg']='最多设置5个管理员';
				echo  json_encode($rs);
                return;
			}
			Db::name("live_manager")->insert( array("uid"=>$touid,"liveuid"=>$showid));
			$rs['isadmin']=1;
		}
		$rs['msg']="设置成功";
		echo  json_encode($rs);
		
	}

	//禁言
	public function gag(){

		$rs = array('code' => 0, 'msg' => '禁言成功', 'info' => array());
		$uid=(int)session("uid");
        
        $data = $this->request->param();
        $touid= $data['touid'] ?? '';
        $roomid= $data['roomid'] ?? '';

        $touid=(int)checkNull($touid);
        $liveuid=(int)checkNull($roomid);

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
        if($touid<1 || $liveuid<1){
            $rs['code']=1000;
			$rs['msg']='参数错误';
			echo  json_encode($rs);
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
            $rs['code']=700;
            $rs['msg']='该账号已被拉黑';
            echo  json_encode($rs);
            return;
        }

        if($userinfo['end_bantime']>time()){
            session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            $rs['code']=700;
            $rs['msg']='该账号已被禁用';
            echo  json_encode($rs);
            return;
        }

		$uidtype = getIsAdmin($uid,$liveuid);
		if($uidtype==30 ){
			$rs["code"]=1001;
			$rs["msg"]='你不是主播或者管理员';
			echo  json_encode($rs);
            return;
		}
		$touidtype = getIsAdmin($touid,$liveuid);
        
        if($touidtype==60 ){

			$rs["code"]=1002;
			$rs["msg"]='对方是超管，不能禁言';
			echo  json_encode($rs);
            return;
		}
        
        if($uidtype==40){
            if($touidtype==50){
                $rs["code"]=1002;
                $rs["msg"]='对方是主播，不能禁言';
                echo  json_encode($rs);
                return;
            }
            
            if($touidtype==40){
                $rs["code"]=1002;
                $rs["msg"]='对方是管理员，不能禁言';
                echo  json_encode($rs);
                return;
            }
            /* 守护 */
            $guard_info=getUserGuard($touid,$liveuid);

            if($uid != $liveuid && $guard_info && $guard_info['type']==2){
                $rs["code"]=1004;
                $rs["msg"]='对方是尊贵守护，不能禁言';
                return $rs;	
            }
        }

		$info = Db::name('live_shut')
                ->where("uid = {$touid} and liveuid={$liveuid}")
                ->find();

        if($info){

            Db::name('live_shut')
                ->where("uid = {$touid} and liveuid={$liveuid}")
                ->update(
                    [
                        'actionid'  =>$uid,
                        'showid'    =>0,
                        'addtime'   =>time()
                    ]);


        }else{

            $result=Db::name('live_shut')
                ->insert(
                    [
                        'uid'       =>$touid,
                        'liveuid'   =>$liveuid,
                        'actionid'  =>$uid,
                        'showid'    =>0,
                        'addtime'   =>time()
                    ]);


        }


        hSet($liveuid.'shutup',$touid,1);
		echo  json_encode($rs);
		
	}

    public function isShutUp() {
		$uid=(int)session("uid");
        
        $data = $this->request->param();
        $liveuid= $data['showid'] ?? '';
        $liveuid=(int)checkNull($liveuid);

		$rs = array('code' => 0, 'msg' => '', 'info' => '0');
		$nowtime=time();
		if($uid>0 && $liveuid>0)
		{
			$admin = getIsAdmin($uid,$liveuid);
			$rs['admin']=$admin;
			
            $isexist=Db::name('live_shut')
                ->where("uid = {$uid} and liveuid={$liveuid}")
                ->find();
            if($isexist){
                $rs['info']='1';
            }
		
		}
		echo  json_encode($rs);
		
  	}

	//踢人
	public function tiren(){

		$rs = array('code' => 0, 'msg' => '', 'info' =>'操作成功');
		$uid=(int)session("uid");
        
        $data = $this->request->param();
        $touid= $data['touid'] ?? '';
        $roomid= $data['roomid'] ?? '';
        $touid=(int)checkNull($touid);
        $liveuid=(int)checkNull($roomid);
        
        if($uid<1){
        	session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            $rs['code']=700;
			$rs['msg']='您的登陆状态失效，请重新登陆！';
			echo json_encode($rs);
            return;
        }
        if($touid<1 || $liveuid<1){
            $rs['code']=1000;
			$rs['msg']='参数错误';
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
            $rs['code']=700;
            $rs['msg']='该账号已被拉黑';
            echo json_encode($rs);
            return;
        }

        if($userinfo['end_bantime']>time()){
            session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            $rs['code']=700;
            $rs['msg']='该账号已被禁用';
            echo json_encode($rs);
            return;
        }

		$uidtype = getIsAdmin($uid,$liveuid);
		if($uidtype==30)
		{
			$rs['code']=1000;
			$rs['msg']='您不是管理员，无权操作';
			echo  json_encode($rs);
            return;
		}
		$touidtype =getIsAdmin($touid,$liveuid);
        
        if($touidtype==60 )
		{
			$rs["code"]=1002;
			$rs["msg"]='对方是超管，不能被踢出';
			echo  json_encode($rs);
            return;
		}
        
        if($uidtype==40){
            if($touidtype==50)
            {
                $rs["code"]=1002;
                $rs["msg"]='对方是主播，不能被踢出';
                echo  json_encode($rs);
                return;
            }
            
            if($touidtype==40 )
            {
                $rs["code"]=1002;
                $rs["msg"]='对方是管理员，不能被踢出';
                echo  json_encode($rs);
                return;
            }
            /* 守护 */
            $guard_info=getUserGuard($touid,$liveuid);

            if($uid != $liveuid && $guard_info && $guard_info['type']==2){
                $rs["code"]=1004;
                $rs["msg"]='对方是尊贵守护，不能被踢出';
                echo json_encode($rs);
                return;
            }
        }
        
        $isexist=Db::name('live_kick')
                ->where("uid={$touid} and liveuid={$liveuid} ")
                ->find();
        if($isexist){
            $rs["code"]=1005;
            $rs["msg"]='对方已被踢出';
            echo  json_encode($rs);
            return;
        }
        
        $result=Db::name('live_kick')->insert([ 'uid'=>$touid,'liveuid'=>$liveuid,'actionid'=>$uid,'addtime'=>time() ]);
		echo  json_encode($rs);
	}

	/*加入/取消 黑名单*/
	public function black(){
		$rs = array('code' => 0, 'msg' => '', 'info' =>'操作成功');
		$uid=(int)session("uid");
        
        $data = $this->request->param();
        $touid= $data['touid'] ?? '';
        $touid=(int)checkNull($touid);
        
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
        if($touid<1){
            $rs['code']=1000;
			$rs['msg']='参数错误';
			echo  json_encode($rs);
            return;
        }
		if($uid==$touid){
			$rs['code']=0;
			$rs['msg']='无法将自己拉黑';
			echo  json_encode($rs);
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
            $rs['code']=700;
            $rs['msg']='该账号已被拉黑';
            echo  json_encode($rs);
            return;
        }

        if($userinfo['end_bantime']>time()){
            session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            $rs['code']=700;
            $rs['msg']='该账号已被禁用';
            echo  json_encode($rs);
            return;
        }
		
        $where['uid']=$uid;
        $where['touid']=$touid;
		$isexist=Db::name("user_black")->where($where)->find();

		if($isexist) {
			$black=Db::name("user_black")->where($where)->delete();
			if($black) {
				$rs['code']=0;
				$rs['msg']='已将该用户移除黑名单';
				echo  json_encode($rs);
			} else {
				$rs['code']=1000;
				$rs['msg']='移除黑名单失败';
				echo  json_encode($rs);
			}
		} else {
			Db::name('user_attention')->where($where)->delete();
			$black=Db::name("user_black")->insert(array("uid"=>$uid,"touid"=>$touid));
			if($black) {
				$rs['code']=0;
				$rs['msg']='已将该用户添加到黑名单';
				echo  json_encode($rs);
			} else {
				$rs['code']=1000;
				$rs['msg']='添加黑名单失败';
				echo  json_encode($rs);
			}
			
		}			 
	}
	/*举报*/
	public function report(){

		$rs = array('code' => 0, 'msg' => '', 'info' =>'操作成功');
		$uid=(int)session("uid");
		$token=session("token");
        
        $data = $this->request->param();
        $tlleuid= $data['tlleuid'] ?? '';
        $liveuid= $data['liveuid'] ?? '';
        $content= $data['content'] ?? '';
        $tlleuid=(int)checkNull($tlleuid);
        $liveuid=(int)checkNull($liveuid);
        $content=checkNull($content);

        if($uid<1){
            $rs['code']=700;
			$rs['msg']='您的登陆状态失效，请重新登陆！';
			echo  json_encode($rs);
            return;
        }
        
        if($tlleuid<1 || $liveuid<1){
            $rs['code']=1000;
			$rs['msg']='参数错误';
			echo  json_encode($rs);
            return;
        }

		if($uid!=$tlleuid)
		{
			$rs['code']=1000;
			$rs['msg']='未知信息错误';
			echo  json_encode($rs);
            return;
		}
		$checkToken=checkToken($uid,$token);
		if($checkToken==700)
		{
			$rs['code']=$checkToken;
			$rs['msg']='登录信息过期，请重新登录';
			echo  json_encode($rs);
            return;
		}
		if($content=="")
		{
			$rs['code']=1001;
			$rs['msg']='举报内容不能为空';
			echo  json_encode($rs);
            return;
		}
		$data=array(
			"uid"=>$uid,
			"touid"=>$liveuid,
			'content'=>$content,
			'addtime'=>time() 
		);
		$user_report=Db::name("report")->insert($data);	
		if($user_report)
		{
			$rs['code']=0;
			$rs['msg']='举报成功';
			echo  json_encode($rs);
			
		}
		else
		{
			$rs['code']=1003;
			$rs['msg']='举报失败';
			echo  json_encode($rs);
			
		}
		 
	}

	/*收费房间扣费*/
	public function roomCharge(){

		$rs = array('code' => 0, 'msg' => '', 'info' => array());
        
        $data = $this->request->param();
        $liveuid= $data['liveuid'] ?? '';
        $stream= $data['stream'] ?? '';
        
        $liveuid=(int)checkNull($liveuid);
        $stream=checkNull($stream);
        
		$uid=(int)session("uid");
		$token=session("token");
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
        
        if($liveuid<1 || $stream==''){
            $rs['code']=1000;
			$rs['msg']='参数错误';
			echo  json_encode($rs);
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
            $rs['code']=700;
            $rs['msg']='该账号已被拉黑';
            echo  json_encode($rs);
            return;
        }

        if($userinfo['end_bantime']>time()){
            session('uid',null);        
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            $rs['code']=700;
            $rs['msg']='该账号已被禁用';
            echo  json_encode($rs);
            return;
        }
        
        $where['uid']=$liveuid;
        $where['stream']=$stream;
        
		$islive=Db::name("live")->field("islive,type,type_val,starttime")->where($where)->find();
		if(!$islive || $islive['islive']==0){
			$rs['code'] = 1005;
			$rs['msg'] = '直播已结束';
			echo json_encode($rs);
            return;
		}
		if($islive['type']==0 || $islive['type']==1 ){
			$rs['code'] = 1006;
			$rs['msg'] = '该房间非扣费房间';
			echo json_encode($rs);
            return;
		}
		$where2['user_id']=$uid;
		$userinfo= Db::name("user_token")->field('token,expire_time')->where($where2)->find();
		if($userinfo['token']!=$token || $userinfo['expire_time']<time()){
            session('uid',null);		
            session('token',null);
            session('user',null);
            session('user_nickname',null);
            session('avatar',null);
            cookie('uid',null);
            cookie('token',null);
			$rs['code'] = 700;
			$rs['msg'] = '您的登陆状态失效，请重新登陆！';
			echo json_encode($rs);
            return;
		}
		$total=$islive['type_val'];
		if($total<=0){
			$rs['code'] = 1007;
			$rs['msg'] = '房间费用有误';
			echo json_encode($rs);
            return;
		}
		$action='6';
		if($islive['type']==3){
			$action='7';
		}
		$giftid=0;
		$giftcount=0;
		$showid=$islive['starttime'];
		$addtime=time();
		/* 更新用户余额 消费 */
        $ifok=Db::name("user")
                ->where([['id','=',$uid],['coin','>=',$total]])
                ->dec('coin',$total)
                ->inc('consumption',$total)
                ->update();
		if(!$ifok){
            $rs['code'] = 1008;
			$rs['msg'] = '余额不足';
			echo json_encode($rs);
            return;
        }
		
	
		/* 家族分成之后的金额 */
		$anthor_total=setFamilyDivide($liveuid,$total);
		
		/* 更新直播 星币 累计星币 */
		
        Db::name("user")
                ->where([['id','=',$liveuid]])
                ->inc('coin',$anthor_total)
                ->inc('votestotal',$total)
                ->update();
                
        $insert_votes=[
            'type'      =>'1',
            'action'    =>$action,
            'uid'       =>$liveuid,
            'fromid'    =>$uid,
            'actionid'  =>$giftid,
            'nums'      =>$giftcount,
            'total'     =>$total,
            'showid'    =>$showid,
            'votes'     =>$anthor_total,
            'addtime'   =>time(),
        ];
        Db::name('user_voterecord')->insert($insert_votes);
		/* 更新直播 星币 累计星币 */
		Db::name("user_coinrecord")
            ->insert(
                array(
                    "type"      =>'0',
                    "action"    =>$action,
                    "uid"       =>$uid,
                    "touid"     =>$liveuid,
                    "giftid"    =>$giftid,
                    "giftcount" =>$giftcount,
                    "totalcoin" =>$total,
                    "showid"    =>$showid,
                    "addtime"   =>$addtime
                )
            );

		$userinfo2=Db::name("user")->field('coin')->where("id='{$uid}'")->find();	
		$rs['coin']=$userinfo2['coin'];
		echo json_encode($rs);
        
	}


}


