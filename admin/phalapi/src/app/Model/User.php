<?php
namespace App\Model;
use PhalApi\Model\NotORMModel as NotORM;

class User extends NotORM {
	/* 用户全部信息 */
	public function getBaseInfo($uid) {
		$info=\PhalApi\DI()->notorm->user
				->select("id,user_nickname,avatar,avatar_thumb,sex,signature,coin,votes,consumption,votestotal,province,city,birthday,location,live_window")
				->where('id=?  and user_type="2"',$uid)
				->fetchOne();
        if($info){
            // 统一钱包后，旧 votes 展示字段兼容为 coin。
            $info['votes']=$info['coin'];
            $info['avatar']=\App\get_upload_path($info['avatar']);
            $info['avatar_thumb']=\App\get_upload_path($info['avatar_thumb']);						
            $info['level']=\App\getLevel($info['consumption']);
            $info['level_anchor']=\App\getLevelAnchor($info['votestotal']);
            $info['lives']=\App\getLives($uid);
            $info['follows']=\App\getFollows($uid);
            $info['fans']=\App\getFans($uid);
            
            $info['vip']=\App\getUserVip($uid);
            $info['liang']=\App\getUserLiang($uid);       
            
            if($info['birthday']){
                $info['birthday']=date('Y-m-d',$info['birthday']);   
            }else{
                $info['birthday']='';
            }
        }

					
		return $info;
	}
			
	/* 判断昵称是否重复 */
	public function checkName($uid,$name){
		$isexist=\PhalApi\DI()->notorm->user
					->select('id')
					->where('id!=? and user_nickname=?',$uid,$name)
					->fetchOne();
		if($isexist){
			return 0;
		}else{
			return 1;
		}
	}
	
	/* 修改信息 */
	public function userUpdate($uid,$fields){
		/* 清除缓存 */
		\App\delcache("userinfo_".$uid);
        
        if(!$fields){
            return false;
        }

		return \PhalApi\DI()->notorm->user
					->where('id=?',$uid)
					->update($fields);
	}

	/* 修改密码 */
	public function updatePass($uid,$oldpass,$pass){
		$userinfo=\PhalApi\DI()->notorm->user
					->select("user_pass")
					->where('id=?',$uid)
					->fetchOne();
		$oldpass=\App\setPass($oldpass);							
		if($userinfo['user_pass']!=$oldpass){
			return 1003;
		}							
		$newpass=\App\setPass($pass);
		return \PhalApi\DI()->notorm->user
					->where('id=?',$uid)
					->update( array( "user_pass"=>$newpass ) );
	}
	
	/* 我的星币 */
	public function getBalance($uid){

		if($uid<1){
			return array(
				'coin'=>0,
				'score'=>0
			);
		}

		return \PhalApi\DI()->notorm->user
				->select("coin,score")
				->where('id=?',$uid)
				->fetchOne();
	}
	
	/* 充值规则 */
	public function getChargeRules(){

		$rules= \PhalApi\DI()->notorm->charge_rules
				->select('id,coin,coin_ios,money,product_id,give,coin_paypal')
				->where('type=0')
				->order('list_order asc')
				->fetchAll();

		return 	\App\normalizeChargeRules($rules);
	}
    
	/* 我的收益 */
	public function getProfit($uid){
		$info= \PhalApi\DI()->notorm->user
				->select("coin,votestotal")
				->where('id=?',$uid)
				->fetchOne();

		$config=\App\getConfigPri();
		
		// 星币提现固定 1:1，不再使用第二套比例。
		$cash_rate=1;
        $cash_start=$config['cash_start'];
		$cash_end=$config['cash_end'];
		$cash_max_times=$config['cash_max_times'];
		$cash_take=0;
		// 统一钱包后，coin 为唯一可提现/消费余额。
		$votes=$info['coin'];
		$total=(string)floor((float)$votes);

        if($cash_max_times){
            
            $tips=\PhalApi\T('每月{start}-{end}号可进行提现申请',['start'=>$cash_start,'end'=>$cash_end]).','.\PhalApi\T('每月只可提现{num}次',['num'=>$cash_max_times]);
        }else{
            
            $tips=\PhalApi\T('每月{start}-{end}号可进行提现申请',['start'=>$cash_start,'end'=>$cash_end]);
        }
        
		$rs=array(
			"votes"=>$votes,
			"votestotal"=>$info['votestotal'],
			"total"=>$total,
			"cash_rate"=>$cash_rate,
			"cash_take"=>$cash_take,
			"tips"=>$tips,
		);
		return $rs;
	}	
	/* 提现  */
	public function setCash($data){
        
        $nowtime=time();
        
        $uid=$data['uid'];
        $accountid=$data['accountid'];
        $cashvote=$data['cashvote'];
        
        $config=\App\getConfigPri();
        $cash_start=$config['cash_start'];
        $cash_end=$config['cash_end'];
        $cash_max_times=$config['cash_max_times'];
        
        $day=(int)date("d",$nowtime);
        
        if($day < $cash_start || $day > $cash_end){
            return 1005;
        }
        
        //**本月第一天
        $month=date('Y-m-d',strtotime(date("Ym",$nowtime).'01'));
        $month_start=strtotime(date("Ym",$nowtime).'01');

        //**本月最后一天
        $month_end=strtotime("{$month} +1 month");
        
        if($cash_max_times){
            $isexist=\PhalApi\DI()->notorm->cash_record
                    ->where('uid=? and addtime > ? and addtime < ?',$uid,$month_start,$month_end)
                    ->count();
            if($isexist >= $cash_max_times){
                return 1006;
            }
        }
        
		$isrz=\PhalApi\DI()->notorm->user_auth
				->select("status")
				->where('uid=?',$uid)
				->fetchOne();
		if(!$isrz || $isrz['status']!=1){
			return 1003;
		}
        
        /* 钱包信息 */
		$accountinfo=\PhalApi\DI()->notorm->cash_account
				->select("*")
				->where('id=? and uid=?',$accountid,$uid)
				->fetchOne();

        if(!$accountinfo){

            return 1007;
        }
        

		/* 最低额度 */
		$cash_min=$config['cash_min'];
		
		// 星币 1:1 提现，cashvote 即扣减和记录的星币数。
        $money=number_format((float)$cashvote,2,".","");
		
		if($money < $cash_min){
			return 1004;
		}
		
		$cashvotes=$money;
        
        
        $ifok=\PhalApi\DI()->notorm->user
            ->where('id = ? and coin>=?', $uid,$cashvotes)
            ->update(
            	array(
            		'coin' => new \NotORM_Literal("coin - {$cashvotes}")
            	)
            );

        if(!$ifok){
            return 1001;
        }
		
		$money=number_format($money,2,".","");
		
		$data=array(
			"uid"=>$uid,
			"money"=>$money,
			"votes"=>$cashvotes,
			"orderno"=>$uid.'_'.$nowtime.rand(100,999),
			"status"=>0,
			"addtime"=>$nowtime,
			"uptime"=>$nowtime,
			"type"=>$accountinfo['type'],
			"account_bank"=>$accountinfo['account_bank'],
			"account"=>$accountinfo['account'],
			"name"=>$accountinfo['name'],
		);
		
		$rs=\PhalApi\DI()->notorm->cash_record->insert($data);
		if(!$rs){
            return 1002;
		}	        
            
        
						
		
		return $rs;
	}
	
	/* 关注 */
	public function setAttent($uid,$touid){
		$isexist=\PhalApi\DI()->notorm->user_attention
					->select("*")
					->where('uid=? and touid=?',$uid,$touid)
					->fetchOne();
		if($isexist){
			\PhalApi\DI()->notorm->user_attention
				->where('uid=? and touid=?',$uid,$touid)
				->delete();
			return 0;
		}else{
			\PhalApi\DI()->notorm->user_black
				->where('uid=? and touid=?',$uid,$touid)
				->delete();
			\PhalApi\DI()->notorm->user_attention
				->insert(
					array(
						"uid"=>$uid,
						"touid"=>$touid,
						"addtime"=>time()
					)
				);

			$isexist1=\PhalApi\DI()->notorm->user_attention_messages
					->select("*")
					->where('uid=? and touid=?',$uid,$touid)
					->fetchOne();

			if($isexist1){
				\PhalApi\DI()->notorm->user_attention_messages
					->where('uid=? and touid=?',$uid,$touid)
					->update(
						array(
							"addtime"=>time()
						)
					);
			}else{

				\PhalApi\DI()->notorm->user_attention_messages
					->insert(
						array(
							"uid"=>$uid,
							"touid"=>$touid,
							"addtime"=>time()
						)
					);
				

				//发送腾讯IM
			}

			return 1;
		}			 
	}	
	
	/* 拉黑 */
	public function setBlack($uid,$touid){
		$isexist=\PhalApi\DI()->notorm->user_black
					->select("*")
					->where('uid=? and touid=?',$uid,$touid)
					->fetchOne();
		if($isexist){
			\PhalApi\DI()->notorm->user_black
				->where('uid=? and touid=?',$uid,$touid)
				->delete();
			return 0;
		}else{
			\PhalApi\DI()->notorm->user_attention
				->where('uid=? and touid=?',$uid,$touid)
				->delete();
			\PhalApi\DI()->notorm->user_black
				->insert(array("uid"=>$uid,"touid"=>$touid));

			return 1;
		}			 
	}
	
	/* 关注列表 */
	public function getFollowsList($uid,$touid,$p){
        if($p<1){
            $p=1;
        }
		$pnum=50;
		$start=($p-1)*$pnum;
		$touids=\PhalApi\DI()->notorm->user_attention
					->select("touid")
					->where('uid=?',$touid)
					->order("addtime desc")
					->limit($start,$pnum)
					->fetchAll();

		foreach($touids as $k=>$v){
			$userinfo=\App\getUserInfo($v['touid']);
			if($userinfo){
				if($uid==$touid){
					$isattent='1';
				}else{
					$isattent=\App\isAttention($uid,$v['touid']);
				}
				$userinfo['isattention']=$isattent;
				$touids[$k]=$userinfo;
			}else{
				\PhalApi\DI()->notorm->user_attention->where('uid=? or touid=?',$v['touid'],$v['touid'])->delete();
				unset($touids[$k]);
			}
		}		
		$touids=array_values($touids);
		return $touids;
	}
	
	/* 粉丝列表 */
	public function getFansList($uid,$touid,$p){
        if($p<1){
            $p=1;
        }
		$pnum=50;
		$start=($p-1)*$pnum;
		$touids=\PhalApi\DI()->notorm->user_attention
					->select("uid")
					->where('touid=?',$touid)
					->order("addtime desc")
					->limit($start,$pnum)
					->fetchAll();
		foreach($touids as $k=>$v){
			$userinfo=\App\getUserInfo($v['uid']);
			if($userinfo){
				$userinfo['isattention']=\App\isAttention($uid,$v['uid']);
				$touids[$k]=$userinfo;
			}else{
				\PhalApi\DI()->notorm->user_attention->where('uid=? or touid=?',$v['uid'],$v['uid'])->delete();
				unset($touids[$k]);
			}
			
		}		
		$touids=array_values($touids);
		return $touids;
	}	

	/* 黑名单列表 */
	public function getBlackList($uid,$touid,$p){
        if($p<1){
            $p=1;
        }
		$pnum=50;
		$start=($p-1)*$pnum;
		$touids=\PhalApi\DI()->notorm->user_black
					->select("touid")
					->where('uid=?',$touid)
					->limit($start,$pnum)
					->fetchAll();
		foreach($touids as $k=>$v){
			$userinfo=\App\getUserInfo($v['touid']);
			if($userinfo){
				$touids[$k]=$userinfo;
			}else{
				\PhalApi\DI()->notorm->user_black->where('uid=? or touid=?',$v['touid'],$v['touid'])->delete();
				unset($touids[$k]);
			}
		}
		$touids=array_values($touids);
		return $touids;
	}
	
	/* 直播记录 */
	public function getLiverecord($touid,$p){
        if($p<1){
            $p=1;
        }
		$pnum=50;
		$start=($p-1)*$pnum;
		$record=\PhalApi\DI()->notorm->live_record
					->select("id,uid,nums,starttime,endtime,title,city")
					->where('uid=?',$touid)
					->order("id desc")
					->limit($start,$pnum)
					->fetchAll();
		foreach($record as $k=>$v){
			$record[$k]['datestarttime']=date("Y.m.d",$v['starttime']);
			$record[$k]['dateendtime']=date("Y.m.d",$v['endtime']);
            $cha=$v['endtime']-$v['starttime'];
			$record[$k]['length']=\App\getSeconds($cha);
		}						
		return $record;						
	}	
	
		/* 个人主页 */
	public function getUserHome($uid,$touid){
		$info=\App\getUserInfo($touid);

		$user_status=$info['user_status'];

		$info['follows']=(string)\App\getFollows($touid);
		$info['fans']=(string)\App\getFans($touid);
		$info['isattention']=(string)\App\isAttention($uid,$touid);
		$info['isblack']=(string)\App\isBlack($uid,$touid);
		$info['isblack2']=(string)\App\isBlack($touid,$uid);
        
        /* 直播状态 */
        $islive='0';
        $isexist=\PhalApi\DI()->notorm->live
                    ->select('uid')
					->where('uid=? and islive=1',$touid)
					->fetchOne();
        if($isexist){
            $islive='1';
        }
		$info['islive']=$islive;	        
		
		/* 贡献榜前三 */
		$rs=array();
		$rs=\PhalApi\DI()->notorm->user_coinrecord
				->select("uid,sum(totalcoin) as total")
				->where('action=1 and touid=?',$touid)
				->group("uid")
				->order("total desc")
				->limit(0,3)
				->fetchAll();
		foreach($rs as $k=>$v){
			$userinfo=\App\getUserInfo($v['uid']);
			$rs[$k]['avatar']=$userinfo['avatar'];
		}		
		$info['contribute']=$rs;	
		
        /* 视频数 */

		if($uid==$touid){  //**自己的视频（需要返回视频的状态前台显示）
			$where=" uid={$uid} and isdel='0' and status=1  and is_ad=0";
		}else{  //**访问其他人的主页视频

			if($uid<1){
				$videoids_s='0';
			}else{
				$videoids_s=\App\getVideoBlack($uid);	
			}
            
			$where="id not in ({$videoids_s}) and uid={$touid} and isdel='0' and status=1  and is_ad=0";
		}

		
        
		$videonums=\PhalApi\DI()->notorm->video
				->where($where)
				->count();
        if(!$videonums){
            $videonums=0;
        }

        $info['videonums']=(string)$videonums;
		  /* 动态数 */

		if($uid==$touid){  //**自己的动态（需要返回动态的状态前台显示）
			$whered=" uid={$uid} and isdel='0' and status=1";
		}else{  //**访问其他人的主页动态
			$whered=" uid={$touid} and isdel='0' and status=1  ";
		}
        
		$dynamicnums=\PhalApi\DI()->notorm->dynamic
				->where($whered)
				->count();
        if(!$dynamicnums){
            $dynamicnums=0;
        }

        $info['dynamicnums']=(string)$dynamicnums;
        /* 直播数 */
        $livenums=\PhalApi\DI()->notorm->live_record
					->where('uid=?',$touid)
					->count();
                    
        $info['livenums']=(string)$livenums;        
		/* 直播记录 */
		$record=array();
		$record=\PhalApi\DI()->notorm->live_record
					->select("id,uid,nums,starttime,endtime,title,city")
					->where('uid=?',$touid)
					->order("id desc")
					->limit(0,50)
					->fetchAll();

		foreach($record as $k=>$v){
			$record[$k]['datestarttime']=date("Y.m.d",$v['starttime']);
			$record[$k]['dateendtime']=date("Y.m.d",$v['endtime']);
            $cha=$v['endtime']-$v['starttime'];
            $record[$k]['length']=\App\getSeconds($cha);
		}		
		$info['liverecord']=$record;

			return $info;
	}
	
	/* 贡献榜 */
	public function getContributeList($touid,$p){
		if($p<1){
            $p=1;
        }
		$pnum=50;
		$start=($p-1)*$pnum;

		$rs=array();
		$rs=\PhalApi\DI()->notorm->user_coinrecord
				->select("uid,sum(totalcoin) as total")
				->where('touid=?',$touid)
				->group("uid")
				->order("total desc")
				->limit($start,$pnum)
				->fetchAll();
				
		foreach($rs as $k=>$v){
			$rs[$k]['userinfo']=\App\getUserInfo($v['uid']);
		}		
		
		return $rs;
	}
	
	/* 设置分销 */
	public function setDistribut($uid,$code){
        
        $isexist=\PhalApi\DI()->notorm->agent
				->select("*")
				->where('uid=?',$uid)
				->fetchOne();
        if($isexist){
            return 1004;
        }
        
        //**获取邀请码用户信息
		$oneinfo=\PhalApi\DI()->notorm->agent_code
				->select("uid")
				->where('code=? and uid!=?',$code,$uid)
				->fetchOne();
		if(!$oneinfo){
			return 1002;
		}
		
		//**获取邀请码用户的邀请信息
		$agentinfo=\PhalApi\DI()->notorm->agent
				->select("*")
				->where('uid=?',$oneinfo['uid'])
				->fetchOne();
		if(!$agentinfo){
			$agentinfo=array(
				'uid'=>$oneinfo['uid'],
				'one_uid'=>0,
			);
		}
        //** 判断对方是否自己下级
        if($agentinfo['one_uid']==$uid ){
            return 1003;
        }
		
		$data=array(
			'uid'=>$uid,
			'one_uid'=>$agentinfo['uid'],
			'addtime'=>time(),
		);
		\PhalApi\DI()->notorm->agent->insert($data);
		return 0;
	}
    
    
    /* 印象标签 */
    public function getImpressionLabel(){
        
        $key="getImpressionLabel";
		$list=\App\getcaches($key);
		if(!$list){
            $list=\PhalApi\DI()->notorm->label
				->select("*")
				->order("list_order asc,id desc")
				->fetchAll();
            if($list){
                \App\setcaches($key,$list); 
            }
			
		}

        return $list;
    }       
    /* 用户标签 */
    public function getUserLabel($uid,$touid){
        $list=\PhalApi\DI()->notorm->label_user
				->select("label")
                ->where('uid=? and touid=?',$uid,$touid)
				->fetchOne();
                
        return $list;
        
    }    

    /* 设置用户标签 */
    public function setUserLabel($uid,$touid,$labels){
        $nowtime=time();
        $isexist=\PhalApi\DI()->notorm->label_user
				->select("*")
                ->where('uid=? and touid=?',$uid,$touid)
				->fetchOne();
        if($isexist){
            $rs=\PhalApi\DI()->notorm->label_user
                ->where('uid=? and touid=?',$uid,$touid)
				->update(
					array(
						'label'=>$labels,
						'uptime'=>$nowtime
					)
				);
            
        }else{
            $data=array(
                'uid'=>$uid,
                'touid'=>$touid,
                'label'=>$labels,
                'addtime'=>$nowtime,
                'uptime'=>$nowtime,
            );
            $rs=\PhalApi\DI()->notorm->label_user->insert($data);
        }
                
        return $rs;
        
    }    
    
    /* 获取我的标签 */
    public function getMyLabel($uid){
        $rs=array();
        $list=\PhalApi\DI()->notorm->label_user
				->select("label")
                ->where('touid=?',$uid)
				->fetchAll();
        $label=array();
        foreach($list as $k=>$v){
            $v_a=preg_split('/,|，/',$v['label']);
            $v_a=array_filter($v_a);
            if($v_a){
                $label=array_merge($label,$v_a);
            }
            
        }

        if(!$label){
            return $rs;
        }
        
        
        $label_nums=array_count_values($label);
        
        $label_key=array_keys($label_nums);
        
        $labels=$this->getImpressionLabel();
        
        $order_nums=array();
        foreach($labels as $k=>$v){
            if(in_array($v['id'],$label_key)){
                $v['nums']=(string)$label_nums[$v['id']];
                $order_nums[]=$v['nums'];
                $rs[]=$v;
            }
        }
        
        array_multisort($order_nums,SORT_DESC,$rs);
        
        return $rs;
        
    }   
    
    /* 获取关于我们列表 */
    public function getPerSetting(){
        $rs=array();
        
        //语言包
        $list=\PhalApi\DI()->notorm->portal_post
				->select("id,post_title,post_title_en")
                ->where("type='2'")
                ->order('list_order asc')
				->fetchAll();

		$language=\PhalApi\DI()->language;
        foreach($list as $k=>$v){

        	if($language=='en'){
        		$post_title=$v['post_title_en'];
        	}else{
        		$post_title=$v['post_title'];
        	}
            
            $rs[]=array(
            	'id'=>'0',
            	'name'=>$post_title,
            	'thumb'=>'',
            	'href'=>\App\get_upload_path("/portal/page/index?id={$v['id']}")
            );
        }

        
        return $rs;
    }
    
    /* 提现账号列表 */
    public function getUserAccountList($uid){
        
        $list=\PhalApi\DI()->notorm->cash_account
                ->select("*")
                ->where('uid=?',$uid)
                ->order("addtime desc")
                ->fetchAll();
                
        return $list;
    }

    /* 账号信息 */
    public function getUserAccount($where){
        
        $list=\PhalApi\DI()->notorm->cash_account
                ->select("*")
                ->where($where)
                ->order("addtime desc")
                ->fetchAll();
                
        return $list;
    }
    /* 设置提账号 */
    public function setUserAccount($data){
        
        $rs=\PhalApi\DI()->notorm->cash_account
                ->insert($data);
                
        return $rs;
    }

    /* 删除提账号 */
    public function delUserAccount($data){
        
        $rs=\PhalApi\DI()->notorm->cash_account
                ->where($data)
                ->delete();
                
        return $rs;
    }
    
	/* 登录奖励信息 */
	public function LoginBonus($uid){
		$rs=array(
			'bonus_switch'=>'0',
			'bonus_day'=>'0',
			'count_day'=>'0',
			'bonus_list'=>array(),
		);
        
        //file_put_contents(API_ROOT.'/../log/phalapi/user_LoginBonus_'.date('Y-m-d').'.txt',date('Y-m-d H:i:s').' 提交参数信息 uid:'.json_encode($uid)."\r\n",FILE_APPEND);
		$configpri=\App\getConfigPri();
		if(!$configpri['bonus_switch']){
			return $rs;
		}
		$rs['bonus_switch']=$configpri['bonus_switch'];

		//file_put_contents(API_ROOT.'/../log/phalapi/user_LoginBonus_'.date('Y-m-d').'.txt',date('Y-m-d H:i:s').' 提交参数信息 bonus_switch:'."\r\n",FILE_APPEND);
		/* 获取登录设置 */
        $key='loginbonus';
		$list=\App\getcaches($key);
		if(!$list){
            $list=\PhalApi\DI()->notorm->loginbonus
					->select("day,coin")
					->fetchAll();
			if($list){
				\App\setcaches($key,$list);
			}
		}
        
        //file_put_contents(API_ROOT.'/../log/phalapi/user_LoginBonus_'.date('Y-m-d').'.txt',date('Y-m-d H:i:s').' 提交参数信息 list:'."\r\n",FILE_APPEND);
		$rs['bonus_list']=$list;
		$bonus_coin=array();
		foreach($list as $k=>$v){
			$bonus_coin[$v['day']]=$v['coin'];
		}

		/* 登录奖励 */
		$signinfo=\PhalApi\DI()->notorm->user_sign
					->select("bonus_day,bonus_time,count_day")
					->where('uid=?',$uid)
					->fetchOne();
        //file_put_contents(API_ROOT.'/../log/phalapi/user_LoginBonus_'.date('Y-m-d').'.txt',date('Y-m-d H:i:s').' 提交参数信息 signinfo:'."\r\n",FILE_APPEND);
		if(!$signinfo){
			$signinfo=array(
				'bonus_day'=>'0',
				'bonus_time'=>'0',
				'count_day'=>'0',
			);
        }
        $nowtime=time();
        if($nowtime - $signinfo['bonus_time'] > 60*60*24){
            $signinfo['count_day']=0;
        }
        $rs['count_day']=(string)$signinfo['count_day'];
		
		if($nowtime>$signinfo['bonus_time']){
			//**更新
			$bonus_time=strtotime(date("Ymd",$nowtime))+60*60*24;
			$bonus_day=$signinfo['bonus_day'];
			if($bonus_day>6){
				$bonus_day=0;
			}
			$bonus_day++;
            $coin=$bonus_coin[$bonus_day];
            
			if($coin){
                $rs['bonus_day']=(string)$bonus_day;
            }
			
		}
        //file_put_contents(API_ROOT.'/../log/phalapi/user_LoginBonus_'.date('Y-m-d').'.txt',date('Y-m-d H:i:s').' 提交参数信息 rs:'."\r\n",FILE_APPEND);
		return $rs;
	}
    
	/* 获取登录奖励 */
	public function getLoginBonus($uid){
		$rs=0;
		$configpri=\App\getConfigPri();
		if(!$configpri['bonus_switch']){
			return $rs;
		}
		
		/* 获取登录设置 */
        $key='loginbonus';
		$list=\App\getcaches($key);
		if(!$list){
            $list=\PhalApi\DI()->notorm->loginbonus
					->select("day,coin")
					->fetchAll();
			if($list){
				\App\setcaches($key,$list);
			}
		}

		$bonus_coin=array();
		foreach($list as $k=>$v){
			$bonus_coin[$v['day']]=$v['coin'];
		}
		
		$isadd=0;
		/* 登录奖励 */
		$signinfo=\PhalApi\DI()->notorm->user_sign
					->select("bonus_day,bonus_time,count_day")
					->where('uid=?',$uid)
					->fetchOne();
		if(!$signinfo){
			$isadd=1;
			$signinfo=array(
				'bonus_day'=>'0',
				'bonus_time'=>'0',
				'count_day'=>'0',
			);
        }
		$nowtime=time();
		if($nowtime>$signinfo['bonus_time']){
			//**更新
			$bonus_time=strtotime(date("Ymd",$nowtime))+60*60*24;
			$bonus_day=$signinfo['bonus_day'];
			$count_day=$signinfo['count_day'];
			if($bonus_day>6){
				$bonus_day=0;
			}
            if($nowtime - $signinfo['bonus_time'] > 60*60*24){
                $count_day=0;
            }
			$bonus_day++;
			$count_day++;
            
 
            if($isadd){
                \PhalApi\DI()->notorm->user_sign
                    ->insert(
                    	array(
                    		"uid"=>$uid,
                    		"bonus_time"=>$bonus_time,
                    		"bonus_day"=>$bonus_day,
                    		"count_day"=>$count_day
                    	)
                    );

            }else{
                \PhalApi\DI()->notorm->user_sign
                    ->where('uid=?',$uid)
                    ->update(
                    	array(
                    		"bonus_time"=>$bonus_time,
                    		"bonus_day"=>$bonus_day,
                    		"count_day"=>$count_day
                    	)
                    );
            }
            
            $coin=$bonus_coin[$bonus_day];
            
			if($coin){
                \PhalApi\DI()->notorm->user
                    ->where('id=?',$uid)
                    ->update(
                    	array(
                    		"coin"=>new \NotORM_Literal("coin + {$coin}")
                    	)
                    );
				

                // 记录
                $insert=array(
                	"type"=>'1',
                	"action"=>'3',
                	"uid"=>$uid,
                	"touid"=>$uid,
                	"giftid"=>$bonus_day,
                	"giftcount"=>'0',
                	"totalcoin"=>$coin,
                	"showid"=>'0',
                	"addtime"=>$nowtime
                );

                \PhalApi\DI()->notorm->user_coinrecord->insert($insert);
            }
            $rs=1;
		}
		
		return $rs;
		
	}

	//**检测用户是否填写了邀请码
	public function checkIsAgent($uid){
		$info=\PhalApi\DI()->notorm->agent->where("uid=?",$uid)->fetchOne();
		if(!$info){
			return 0;
		}

		return 1;
	}


    //**获取认证信息
    public function getAuthInfo($uid){
    	$info=\PhalApi\DI()->notorm->user_auth
    			->where("uid=? and status=1",$uid)
    			->select("real_name,cer_no")
    			->fetchOne();
    	return $info;
    }
	
	
	
	//**获取每日任务
    public function seeDailyTasks($uid){
    	$configpri=\App\getConfigPri();
    	$configpub=\App\getConfigPub();
		$name_coin=$configpub['name_coin']; //**星币名称
		
		
		
		$list=[];
		
		//**type 任务类型 1观看直播, 2观看视频, 3直播奖励, 4打赏奖励, 5分享奖励
		$type=[
			'1'=>\PhalApi\T('观看直播'),
			'2'=>\PhalApi\T('观看视频'),
			'3'=>\PhalApi\T('直播奖励'),
			'4'=>\PhalApi\T('打赏奖励'),
			'5'=>\PhalApi\T('分享奖励')
		];
		
		//** 当天时间
		$time=strtotime(date("Y-m-d 00:00:00",time()));
		foreach($type as $k=>$v){
			$data=[
				'id'=>'0',
				'type'=>(string)$k,
				'title'=>$v,
				'tip_m'=>'',
				'state'=>'0',
			];
			
			if($k==1){
				$target=$configpri['watch_live_term'];
				$reward=$configpri['watch_live_coin'];
			}else if($k==2){
				$target=$configpri['watch_video_term'];
				$reward=$configpri['watch_video_coin'];
			}else if($k==3){
				$target=$configpri['open_live_term']*60;
				$reward=$configpri['open_live_coin'];
				
			}else if($k==4){
				$target=$configpri['award_live_term'];
				$reward=$configpri['award_live_coin'];
			}else{
				$target=$configpri['share_live_term'];
				$reward=$configpri['share_live_coin'];
			}
			
			
			$save=[
				'uid'=>$uid,
				'type'=>$k,
				'target'=>$target,
				'schedule'=>'0',
				'reward'=>$reward,
				'addtime'=>$time,
				'state'=>'0',
			];
			
			$where="uid={$uid} and type={$k}";	
			//**每日任务
			$info=\PhalApi\DI()->notorm->user_daily_tasks
    			->where($where)
    			->select("*")
    			->fetchOne();
			
			if(!$info){
				$info=\PhalApi\DI()->notorm->user_daily_tasks->insert($save);
				
				
			}else if($info['addtime']!=$time){
				$save['uptime']=time(); //**更新时间
				\PhalApi\DI()->notorm->user_daily_tasks->where("id={$info['id']}")->update($save);
			}else{
				$target=$info['target'];
				$reward=$info['reward'];
				$data['state']=$info['state'];
			}
			
			//**提示标语
			if($k==1){
				$tip_m=\PhalApi\T("观看直播时长达到{target}分钟，奖励{reward}{coinname}",['target'=>$target,'reward'=>$reward,'coinname'=>$name_coin]);
			}else if($k==2){
				$tip_m=\PhalApi\T("观看视频时长达到{target}分钟，奖励{reward}{coinname}",['target'=>$target,'reward'=>$reward,'coinname'=>$name_coin]);
			}else if($k==3){
				$tip_m=\PhalApi\T("每天开播满足{target}分钟可获得奖励{reward}{coinname}",['target'=>$target,'reward'=>$reward,'coinname'=>$name_coin]);
			}else if($k==4){
				$tip_m=\PhalApi\T("打赏主播超过{target}{coinname}，奖励{reward}{coinname}",['target'=>$target,'reward'=>$reward,'coinname'=>$name_coin]);
			}else{
				$tip_m=\PhalApi\T("直播间每日分享{target}次可获得奖励{reward}{coinname}",['target'=>$target,'reward'=>$reward,'coinname'=>$name_coin]);
			}
			$data['id']=$info['id'];
			$data['tip_m']=$tip_m;
			$list[]=$data;	
		}
    	return $list;
    }
	
	//**领取每日任务奖励
	public function receiveTaskReward($uid,$taskid){
		$rs = array('code' => 0, 'msg' => '', 'info' => array());
		$where="id={$taskid} and uid={$uid}";	
		//**每日任务
		$info=\PhalApi\DI()->notorm->user_daily_tasks
			->where($where)
			->select("*")
			->fetchOne();
			
		if(!$info){
			$rs['code']='1001';
			$rs['msg']=\PhalApi\T('系统繁忙,请稍后操作');
			return $rs;
		}
		if($info['state']==0){
			$rs['code']='1001';
			$rs['msg']=\PhalApi\T('任务未达标,请继续加油');
		}else if($info['state']==2){
			$rs['code']='1001';
			$rs['msg']=\PhalApi\T('奖励已送达,不能重复领取!');
		}else{
			$rs['msg']=\PhalApi\T('奖励已发放,明天继续加油');
			
			
			//**更新任务状态
			$issave=\PhalApi\DI()->notorm->user_daily_tasks
				->where("id={$info['id']}")
				->update(['state'=>2,'uptime'=>time()]);
				
			if($issave){
				$coin=$info['reward'];
				/* 增加用户星币 */
				$isprofit =\PhalApi\DI()->notorm->user
							->where('id = ?', $uid)
							->update(
								array(
									'coin' => new \NotORM_Literal("coin + {$coin}")
								)
							);

				if($isprofit){  //**生成记录
					$insert=array(
						"type"=>'1',
						"action"=>'21',
						"uid"=>$uid,
						"touid"=>$uid,
						"giftid"=>'0',
						"giftcount"=>'0',
						"totalcoin"=>$coin,
						"addtime"=>time() 
					);
					\PhalApi\DI()->notorm->user_coinrecord->insert($insert);
				}
				
				//**删除用户每日任务数据
				$key="seeDailyTasks_".$uid;
				\App\delCache($key);
			}
			
			

		}
	
		return $rs;
	}


	//**用户设置美颜参数
	public function setBeautyParams($uid,$params){
		$info=\PhalApi\DI()->notorm->user_beauty_params
		->where("uid=?",$uid)
		->fetchOne();

		$data=array(
			'params'=>$params
		);

		if($info){
			$res=\PhalApi\DI()->notorm->user_beauty_params
			->where("uid=?",$uid)
			->update($data);

		}else{
			
			$data['uid']=$uid;
			$res=\PhalApi\DI()->notorm->user_beauty_params
			->insert($data);
		}

		if($res===false){
			return 0;
		}

		return 1;

	}

	//**获取用户设置的美颜参数
	public function getBeautyParams($uid){
		$info=\PhalApi\DI()->notorm->user_beauty_params
		->where("uid=?",$uid)
		->fetchOne();

		$params=[];

		if(!$info){
			$configpub=\App\getConfigPub();
			$params=array(
				'skin_whiting'=>$configpub['skin_whiting'],
				'skin_smooth'=>$configpub['skin_smooth'],
				'skin_tenderness'=>$configpub['skin_tenderness'],
				'eye_brow'=>$configpub['eye_brow'],
				'big_eye'=>$configpub['big_eye'],
				'eye_length'=>$configpub['eye_length'],
				'eye_corner'=>$configpub['eye_corner'],
				'eye_alat'=>$configpub['eye_alat'],
				'face_lift'=>$configpub['face_lift'],
				'face_shave'=>$configpub['face_shave'],
				'mouse_lift'=>$configpub['mouse_lift'],
				'nose_lift'=>$configpub['nose_lift'],
				'chin_lift'=>$configpub['chin_lift'],
				'forehead_lift'=>$configpub['forehead_lift'],
				'lengthen_noseLift'=>$configpub['lengthen_noseLift'],
				'brightness'=>$configpub['brightness'],
			);
		}else{
			$params=json_decode($info['params'],true);
		}

		return $params;
	}

    //**BrainTree支付回调
	public function BraintreeCallback($uid,$orderno,$ordertype,$nonce,$money){

		$now=time();
		
		if($ordertype=='coin_charge'){ //**星币充值

			//**查询星币充值订单信息
			$charge_info=\PhalApi\DI()->notorm->charge_user
				->where("uid=? and orderno=? and type=6 and money=?",$uid,$orderno,$money)
				->fetchOne();

			if(!$charge_info){
				return 1001;
			}

			if($charge_info['status']!=0){
				return 1002;
			}

			// 星币充值不再叠加赠送字段。
			$coin=$charge_info['coin'];
			\PhalApi\DI()->notorm->user
				->where("id=?",$uid)
				->update(
					array(
						'coin' => new \NotORM_Literal("coin + {$coin}")
					)
				);

			$configpri=\App\getConfigPri();

			$data['trade_no']=$nonce;
			$data['status']=1;
			$data['ambient']=1;

			if(!$configpri['braintree_paypal_environment']){
				$data['ambient']=0;
			}

			\PhalApi\DI()->notorm->charge_user
				->where("id=?",$charge_info['id'])
				->update($data);

			//**首充 赠送积分 赠送VIP 赠送热门礼物
			if($charge_info['is_first']==1){
				//**添加积分
				if($charge_info['score']>0){
					\PhalApi\DI()->notorm->user
						->where(['id'=>$uid])
						->update(
							array(
								'score' => new \NotORM_Literal("score + {$charge_info['score']}")
							)
						);

					$arr=array(
		        		'type'=>1,
		        		'action'=>'22',
		        		'uid'=>$charge_info['uid'],
		        		'touid'=>$uid,
		        		'giftid'=>$charge_info['id'],
		        		'giftcount'=>1,
		        		'totalcoin'=>$charge_info['score'],
		        		'addtime'=>$now
		        	);

					\PhalApi\DI()->notorm->user_scorerecord->insert($arr);

				}

				//**赠送vip
				if($charge_info['vip_length']>0){
					$endtime=60*60*24*$charge_info['vip_length'];
					$vip_info=\PhalApi\DI()->notorm->vip_user
								->where(['uid'=>$uid])
								->fetchOne();
					if(!$vip_info){
						$endtime=$endtime+$now;
						\PhalApi\DI()->notorm->vip_user
							->insert([
								'uid'=>$uid,
	        					'addtime'=>$now,
	        					'endtime'=>$endtime
							]);
					}else{

						if($vip_info['endtime']>$now){
	        				$endtime=$endtime+$vip_info['endtime'];
	        			}else{
	        				$endtime=$endtime+$now;
	        			}

						\PhalApi\DI()->notorm->vip_user
							->where(['uid'=>$uid])
							->update(['endtime'=>$endtime]);
					}
				}

				//**赠送热门礼物
				if($charge_info['giftid']>0 && $charge_info['gift_num']>0){
					$backpack_info=\PhalApi\DI()->notorm->backpack
								->where(
									[
										'uid'=>$uid,
										'giftid'=>$charge_info['giftid']
									]
								)->fetchOne();

					if(!$backpack_info){

						$arr=array(
	        				'uid'=>$uid,
	        				'giftid'=>$charge_info['giftid'],
	        				'nums'=>$charge_info['gift_num']
	        			);

					}else{
						\PhalApi\DI()->notorm->backpack
							->where(
								[
									'uid'=>$uid,
									'giftid'=>$charge_info['giftid']
								]
							)->update(
								array(
									'nums' => new \NotORM_Literal("nums + {$charge_info['gift_num']}")
								)
							);
					}
				}


			}



			//**处理下级充值上级收益问题
			\App\setAgentProfit($uid,$charge_info['coin']);
			}

	}

	//**转盘中奖记录
	public function getTurntableWinLists($uid,$p){

		$pnums=50;
		$start=($p-1)*$pnums;
		$list=\PhalApi\DI()->notorm->turntable_win
			->select("id,type,type_val,thumb,nums,addtime")
			->where(['uid'=>$uid])
			->order('addtime desc')
			->limit($start,$pnums)
			->fetchAll();

		foreach ($list as $k => $v) {
			$type=$v['type'];
			if($type==1){
				$list[$k]['thumb']=\App\get_upload_path('/static/appapi/images/coin.png');

			}else if($type==2){
				$giftinfo=\PhalApi\DI()->notorm->gift->select("gifticon")->where(['id'=>$v['type_val']])->fetchOne();
				if($giftinfo){
					$list[$k]['thumb']=\App\get_upload_path($giftinfo['gifticon']);
				}
				
			}else if($type==3){
				$list[$k]['thumb']=\App\get_upload_path($v['thumb']);
			}

			$list[$k]['addtime']=date('Y-m-d H:i:s',$v['addtime']);
		}

		return $list;
	}

	public function clearTurntableWinLists($uid){
		\PhalApi\DI()->notorm->turntable_log->where(['uid'=>$uid])->delete();
		\PhalApi\DI()->notorm->turntable_win->where(['uid'=>$uid])->delete();
		return 1;
	}


	public function checkTeenager($uid){
		$rs=array('code'=>0,'msg'=>'','info'=>array());
		
		$info=\PhalApi\DI()->notorm->user_teenager
			->where(['uid'=>$uid])
			->fetchOne();

		if(!$info){

			$arr=['is_setpassword'=>'0','status'=>'0'];
			$rs['info'][0]=$arr;

			return $rs;
		}

		$arr=['is_setpassword'=>'1','status'=>(string)$info['status']];
		$rs['info'][0]=$arr;

		return $rs;
	}

	public function setTeenagerPassword($uid,$password,$type){
		$info=\PhalApi\DI()->notorm->user_teenager
			->where(['uid'=>$uid])
			->fetchOne();

		$password=md5($password);

		if($info){

			if($type==1){ //**开启青少年模式
				if($password != $info['password']){
					return 1001;
				}
			}
			
			$res=\PhalApi\DI()->notorm->user_teenager
				->where(['uid'=>$uid])
				->update(
					[
						'edittime'=>time(),
						'status'=>1,
						'password'=>$password
					]
				);

		}else{

			//**新增记录
			$res=\PhalApi\DI()->notorm->user_teenager
				->where(['uid'=>$uid])
				->insert(['uid'=>$uid,'password'=>$password,'status'=>1,'addtime'=>time()]);
		}

		if(!$res){
			return 1002;
		}

		return 1;

	}

	public function updateTeenagerPassword($uid,$oldpassword,$password){

		$info=\PhalApi\DI()->notorm->user_teenager
			->where(['uid'=>$uid])
			->fetchOne();

		if(!$info){
			return 1001;
		}

		if(md5($oldpassword) != $info['password']){
			return 1002;
		}

		$res = \PhalApi\DI()->notorm->user_teenager
			->where(['uid'=>$uid])
			->update(
				[
					'password'=>md5($password),
					'edittime'=>time()
				]
			);


		return $res;


	}

	public function closeTeenager($uid,$password){
		$info=\PhalApi\DI()->notorm->user_teenager
			->where(['uid'=>$uid])
			->fetchOne();

		if(!$info){
			return 1001;
		}

		if(md5($password) != $info['password']){
			return 1003;
		}

		if(!$info['status']){
			return 1002;
		}

		$res=\PhalApi\DI()->notorm->user_teenager
			->where(['uid'=>$uid])
			->update(
				[
					'status'=>0,
					'edittime'=>time()
				]
			);

		return $res;
	}

	//**定时增加用户青少年模式使用时间
	public function addTeenagerTime($uid){

		$rs=array('code'=>0,'msg'=>\PhalApi\T('更新成功'),'info'=>array());

		$info=\PhalApi\DI()->notorm->user_teenager
			->where(['uid'=>$uid])
			->fetchOne();

		$msg=\PhalApi\T('用户未开启青少年模式');

		if(!$info){
			$rs['code']=1001;
			$rs['msg']=$msg;
			return $rs;
		}

		if(!$info['status']){
			$rs['code']=1002;
			$rs['msg']=$msg;
			return $rs;
		}


		$res = $this->checkTeenagerIsOvertime($uid);

		if($res['code']!=0){
			return $res;
		}

		$now=time();

		$info = \PhalApi\DI()->notorm->user_teenager_time
			->where(['uid'=>$uid])
			->fetchOne();

		if(!$info){
			\PhalApi\DI()->notorm->user_teenager_time->insert(['uid'=>$uid,'length'=>10,'addtime'=>$now]);
		}else{

			\PhalApi\DI()->notorm->user_teenager_time->where(['uid'=>$uid])
				->update(
					array(
						'length' => new \NotORM_Literal("length + 10"),
						'uptime'=>$now
					)
				);
		}

		return $rs;
	}

	//**更换背景图
	public function updateBgImg($uid,$img){
		$result=\PhalApi\DI()->notorm->user
			->where(['id'=>$uid])
			->update(['bg_img'=>$img]);

		if(!$result){
			return 1001;
		}

		return 1;
	}

	//**检测用户青少年模式是否可用
	public function checkTeenagerIsOvertime($uid){
		$rs=array('code'=>0,'msg'=>'','info'=>array());

		$now=time();

		$hour=date("H",$now);

		//**测试用$hour=22;

		if($hour>=22 || $hour<6){
			$rs['code']=10010; //**code固定
			$rs['msg']=\PhalApi\T('青少年模式下每日晚22时至次日6时期间无法使用APP');
			return $rs;
		}

		$info = \PhalApi\DI()->notorm->user_teenager_time
			->where(['uid'=>$uid])
			->fetchOne();

		//**测试用$info['length']=2500;

		if($info){

			if($info['length'] >= 40*60){
				$rs['code']=10011; //**code固定
				$rs['msg']=\PhalApi\T('青少年模式下你今日的使用时长已超过40分钟，不能继续使用APP');
				return $rs;
			}
		}

		return $rs;
	}


	public function setLiveWindow($uid){
		$rs=array('code'=>0,'msg'=>'','info'=>array());
		$status = \PhalApi\DI()->notorm->user
			->where(['id'=>$uid])
			->fetchOne("live_window");

		if(!$status){
			$status='1';
			$data=['live_window'=>$status];
			$rs['msg']=\PhalApi\T('开启成功');

		}else{
			$status='0';
			$data=['live_window'=>$status];
			$rs['msg']=\PhalApi\T('关闭成功');
		}

		$res = \PhalApi\DI()->notorm->user
			->where(['id'=>$uid])
			->update($data);

		if(!$res){
			$rs['code']=1001;
			$rs['msg']=\PhalApi\T('失败啦');
			return $rs;
		}

		$rs['info'][0]['status']=$status;
		return $rs;
	}
}
