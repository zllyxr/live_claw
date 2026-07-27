<?php

namespace App\Model;
use PhalApi\Model\NotORMModel as NotORM;
if (!session_id()) session_start();

class Home extends NotORM {
    protected $live_fields='uid,title,city,stream,pull,thumb,isvideo,type,type_val,goodnum,anyway,starttime,game_action,isrecommend,live_type,voice_type,deviceinfo';
     
    
	/* 轮播 */
	public function getSlide($where){

		$rs=\PhalApi\DI()->notorm->slide_item
			->select("image as slide_pic,url as slide_url")
			->where($where)
			->order("list_order asc")
			->fetchAll();
		foreach($rs as $k=>$v){
			$rs[$k]['slide_pic']=\App\get_upload_path($v['slide_pic']);
		}				

		return $rs;
	}

	/* 热门主播 */
    public function getHot($p) {
        if($p<1){
            $p=1;
        }
        $pnum=50;
        $start=($p-1)*$pnum;
		
		$where=" islive= '1' and live_type=0 and ishot=1";
        

		$result=\PhalApi\DI()->notorm->live
                    ->select($this->live_fields.',hotvotes,isrecommend,recommend_time')
                    ->where($where)
                    ->order('isrecommend desc,recommend_time desc,hotvotes desc,starttime desc')
                    ->limit($start,$pnum)
                    ->fetchAll();
                    
		foreach($result as $k=>$v){
			$v=\App\handleLive($v);     
            $result[$k]=$v;
		}	
		
		return $result;
    }
	
	
	/* 聊天室 */
    public function getRecommendVoiceLive() {

		$where=" islive= '1' and live_type=1";

		$result=\PhalApi\DI()->notorm->live
                    ->select($this->live_fields.',hotvotes')
                    ->where($where)
                    ->order('isrecommend desc,recommend_time desc,hotvotes desc,starttime desc')
                    ->limit(0,2)
                    ->fetchAll();
                    
		foreach($result as $k=>$v){
			$v=\App\handleLive($v);     
            $result[$k]=$v;
		}	

		return $result;
    }
	
	
	
	
		/* 关注列表 */
    public function getFollow($uid,$live_type,$p) {
        $rs=array(
            'title'=>\PhalApi\T('你关注的主播没有开播'),
            'des'=>\PhalApi\T('赶快去看看其他主播的直播吧'),
            'list'=>array(),
        );
        if($p<1){
            $p=1;
        }
		$result=array();
		$pnum=50;
		$start=($p-1)*$pnum;
		
		$touid=\PhalApi\DI()->notorm->user_attention
				->select("touid")
				->where("uid=?",$uid)
				->fetchAll();
		
				
		if(!$touid){
            return $rs;
        }
        
        $rs['title']=\PhalApi\T('你关注的主播没有开播');
        $rs['des']=\PhalApi\T('赶快去看看其他主播的直播吧');
        $where=" islive='1' and live_type=".$live_type;		
        			
        if($p!=1){
            $endtime=$_SESSION['follow_starttime'];
            if($endtime){
                $start=0;
                $where.=" and starttime < {$endtime}";
            }
            
        }	
    
        $touids=array_column($touid,"touid");
        $touidss=implode(",",$touids);
        $where.=" and uid in ({$touidss})";
        $result=\PhalApi\DI()->notorm->live
                ->select($this->live_fields)
                ->where($where)
                ->order("starttime desc")
                ->limit(0,$pnum)
                ->fetchAll();
	
		foreach($result as $k=>$v){
            
			$v=\App\handleLive($v);
            
            $result[$k]=$v;
		}	

		if($result){
			$last=end($result);
			$_SESSION['follow_starttime']=$last['starttime'];
		}
        
        $rs['list']=$result;

		return $rs;					
    }
		
		
		/* 搜索 */
    public function search($uid,$key,$p) {
        if($p<1){
            $p=1;
        }
		$pnum=50;
		$start=($p-1)*$pnum;
		$where=' user_type="2" and ( id=? or user_nickname like ?  or goodnum like ? ) and id!=?';
		if($p!=1){
			$id=$_SESSION['search'];
            if($id){
                $where.=" and id < {$id}";
            }
		}
		
		$result=\PhalApi\DI()->notorm->user
				->select("id,user_nickname,avatar,sex,signature,consumption,votestotal")
				->where($where,$key,'%'.$key.'%','%'.$key.'%',$uid)
				->order("id desc")
				->limit($start,$pnum)
				->fetchAll();
		foreach($result as $k=>$v){
			$v['id']=(string)$v['id'];
			$v['sex']=(string)$v['sex'];
			$v['votestotal']=(string)$v['votestotal'];
			$v['level']=\App\getLevel($v['consumption']);
			$v['level_anchor']=\App\getLevelAnchor($v['votestotal']);
			$v['isattention']=\App\isAttention($uid,$v['id']);
			$v['avatar']=\App\get_upload_path($v['avatar']);
			unset($v['consumption']);
            
            $result[$k]=$v;
		}				
		
		if($result){
			$last=end($result);
			$_SESSION['search']=$last['id'];
		}
		
		return $result;
    }
	
	/* 附近 */
    public function getNearby($lng,$lat,$live_type,$p) {
        if($p<1){
            $p=1;
        }
		$pnum=50;
		$start=($p-1)*$pnum;
		$where=" islive='1' and lng!='' and lat!='' and live_type='{$live_type}'";
		
		$result=\PhalApi\DI()->notorm->live
				->select($this->live_fields.",getDistance('{$lat}','{$lng}',lat,lng) as distance,province")
				->where($where)
                ->order("distance asc")
                ->limit($start,$pnum)
				->fetchAll();	
		foreach($result as $k=>$v){
            
			$v=\App\handleLive($v);
            
            if($v['distance']>1000){
                $v['distance']=1000;
            }
            $v['distance']=$v['distance'].'km';

            $result[$k]=$v;
		}
		
		return $result;
    }


	/* 推荐 */
	public function getRecommend(){

		$result=\PhalApi\DI()->notorm->user
				->select("id,user_nickname,avatar,avatar_thumb")
				->where("isrecommend='1'")
				->order("recommend_time desc,votestotal desc")
				->limit(0,12)
				->fetchAll();
		foreach($result as $k=>$v){
			$v['avatar']=\App\get_upload_path($v['avatar']);
			$v['avatar_thumb']=\App\get_upload_path($v['avatar_thumb']);
			$fans=\App\getFans($v['id']);
			$v['fans']=$fans;
            
            $result[$k]=$v;
		}
		return  $result;
	}
	/* 关注推荐 */
	public function attentRecommend($uid,$touids){

        $users=preg_split('/,|，/',$touids);
		foreach($users as $k=>$v){
			$touid=$v;
			$isAttention=\App\isAttention($uid,$touid);
			if($touid && !$isAttention){
				\PhalApi\DI()->notorm->user_black
					->where('uid=? and touid=?',$uid,$touid)
					->delete();
				\PhalApi\DI()->notorm->user_attention
					->insert(array("uid"=>$uid,"touid"=>$touid));
			}
			
		}
		return 1;
	}

	/*获取收益排行榜*/
	public function profitList($uid,$type,$p){
        if($p<1){
            $p=1;
        }
		$pnum=50;
		$start=($p-1)*$pnum;

		switch ($type) {
			case 'day':
				//获取今天开始结束时间
				$dayStart=strtotime(date("Y-m-d"));
				$dayEnd=strtotime(date("Y-m-d 23:59:59"));
				$where=" addtime >={$dayStart} and addtime<={$dayEnd} and ";

			break;

			case 'week':
                $w=date('w'); 
                //获取本周开始日期，如果$w是0，则表示周日，减去 6 天 
                $first=1;
                //周一
                $week=date('Y-m-d H:i:s',strtotime( date("Ymd")."-".($w ? $w - $first : 6).' days')); 
                $week_start=strtotime( date("Ymd")."-".($w ? $w - $first : 6).' days'); 

                //本周结束日期 
                //周天
                $week_end=strtotime("{$week} +1 week")-1;
                
				$where=" addtime >={$week_start} and addtime<={$week_end} and ";

			break;

			case 'month':
                //本月第一天
                $month=date('Y-m-d',strtotime(date("Ym").'01'));
                $month_start=strtotime(date("Ym").'01');

                //本月最后一天
                $month_end=strtotime("{$month} +1 month")-1;

				$where=" addtime >={$month_start} and addtime<={$month_end} and ";

			break;

			case 'total':
				$where=" ";
			break;
			
			default:
				//获取今天开始结束时间
				$dayStart=strtotime(date("Y-m-d"));
				$dayEnd=strtotime(date("Y-m-d 23:59:59"));
				$where=" addtime >={$dayStart} and addtime<={$dayEnd} and ";
			break;
		}

		//判断榜单金额开关是否开启
		$configpri = \App\getConfigPri();
		$list_coin_switch=$configpri['list_coin_switch'];

		$where.=" action in (1,2)";
		
		$result=\PhalApi\DI()->notorm->user_voterecord
            ->select('sum(total) as totalcoin, uid')
            ->where($where)
            ->group('uid')
            ->order('totalcoin desc')
            ->limit($start,$pnum)
            ->fetchAll();

		foreach ($result as $k => $v) {
            $userinfo=\App\getUserInfo($v['uid']);
            $v['avatar']=$userinfo['avatar'];
			$v['avatar_thumb']=$userinfo['avatar_thumb'];
			$v['user_nickname']=$userinfo['user_nickname'];
			$v['sex']=$userinfo['sex'];
			$v['level']=$userinfo['level'];
			$v['level_anchor']=$userinfo['level_anchor'];
			
			
			$totalcoin='***';
			if($list_coin_switch){
				$totalcoin=(string)intval($v['totalcoin']);
			}
			$v['totalcoin']=$totalcoin;
            
            $v['isAttention']=\App\isAttention($uid,$v['uid']);//判断当前用户是否关注了该主播
            $v['uid']=(string)$v['uid'];
            $v['sex']=(string)$v['sex'];
            
            $result[$k]=$v;
		}

		return $result;
	}



	/*获取消费排行榜*/
	public function consumeList($uid,$type,$p){
        if($p<1){
            $p=1;
        }
		$pnum=50;
		$start=($p-1)*$pnum;

		switch ($type) {
			case 'day':
				//获取今天开始结束时间
				$dayStart=strtotime(date("Y-m-d"));
				$dayEnd=strtotime(date("Y-m-d 23:59:59"));
				$where=" addtime >={$dayStart} and addtime<={$dayEnd} and ";

			break;
            
            case 'week':
                $w=date('w'); 
                //获取本周开始日期，如果$w是0，则表示周日，减去 6 天 
                $first=1;
                //周一
                $week=date('Y-m-d H:i:s',strtotime( date("Ymd")."-".($w ? $w - $first : 6).' days')); 
                $week_start=strtotime( date("Ymd")."-".($w ? $w - $first : 6).' days'); 

                //本周结束日期 
                //周天
                $week_end=strtotime("{$week} +1 week")-1;
                
				$where=" addtime >={$week_start} and addtime<={$week_end} and ";

			break;

			case 'month':
                //本月第一天
                $month=date('Y-m-d',strtotime(date("Ym").'01'));
                $month_start=strtotime(date("Ym").'01');

                //本月最后一天
                $month_end=strtotime("{$month} +1 month")-1;

				$where=" addtime >={$month_start} and addtime<={$month_end} and ";

			break;

			case 'total':
				$where=" ";
			break;
			
			default:
				//获取今天开始结束时间
				$dayStart=strtotime(date("Y-m-d"));
				$dayEnd=strtotime(date("Y-m-d 23:59:59"));
				$where=" addtime >={$dayStart} and addtime<={$dayEnd} and ";
			break;
		}
		
		//判断榜单金额开关是否开启
		$configpri = \App\getConfigPri();
		$list_coin_switch=$configpri['list_coin_switch'];

		$where.=" type=0 and action in ('1','2')";
		
        $result=\PhalApi\DI()->notorm->user_coinrecord
            ->select('sum(totalcoin) as totalcoin, uid')
            ->where($where)
            ->group('uid')
            ->order('totalcoin desc')
            ->limit($start,$pnum)
            ->fetchAll();

		foreach ($result as $k => $v) {
            $userinfo=\App\getUserInfo($v['uid']);
            $v['avatar']=$userinfo['avatar'];
			$v['avatar_thumb']=$userinfo['avatar_thumb'];
			$v['user_nickname']=$userinfo['user_nickname'];
			$v['sex']=$userinfo['sex'];
			$v['level']=$userinfo['level'];
			$v['level_anchor']=$userinfo['level_anchor'];
			
			$totalcoin='***';
			if($list_coin_switch){
				$totalcoin=(string)intval($v['totalcoin']);
			}
            
            $v['totalcoin']=$totalcoin;
            $v['isAttention']=\App\isAttention($uid,$v['uid']);//判断当前用户是否关注了该主播
            $v['uid']=(string)$v['uid'];
            $v['sex']=(string)$v['sex'];
            
            $result[$k]=$v;
		}

		return $result;
	}
    
    /* 分类下直播 */
    public function getClassLive($liveclassid,$p) {
        if($p<1){
            $p=1;
        }
		$pnum=50;
		//$start=($p-1)*$pnum;
		$start=0;
		$where=" islive='1' and liveclassid={$liveclassid} and live_type=0";
        
		if($p!=1){
			$endtime=$_SESSION['getClassLive_starttime'];
            if($endtime){
                $where.=" and starttime < {$endtime}";
            }
			
		}
		$last_starttime=0;
		$result=\PhalApi\DI()->notorm->live
				->select($this->live_fields)
				->where($where)
				->order("starttime desc")
				->limit(0,$pnum)
				->fetchAll();	
		foreach($result as $k=>$v){
			$v=\App\handleLive($v);
            $result[$k]=$v;
		}		
		if($result){
            $last=end($result);
			$_SESSION['getClassLive_starttime']=$last['starttime'];
		}

		return $result;
    }


    //获取语音聊天室列表
    public function getVoiceLiveList($p){
    	if($p<1){
            $p=1;
        }
		$pnum=50;
		$where=" islive= '1' and ishot='1' and live_type=1";
        
        if($p==1){
			$_SESSION['voicelive_starttime']=time();
		}
        
		if($p>1){
			$recommend_time=$_SESSION['recommend_time'];
            if($recommend_time){
                $where.=" and recommend_time < {$recommend_time}";
            }else{
				$hotvotes=$_SESSION['hotvotes'];
				if($hotvotes){
					$where.=" and hotvotes < {$hotvotes}";
				}else{
					$endtime=$_SESSION['voicelive_starttime'];
					if($endtime){
						$where.=" and starttime < {$endtime}";
					}
				}
			}

			$where.=" and isvideo=0";
			
		}

		
		$result=\PhalApi\DI()->notorm->live
            ->select($this->live_fields.',hotvotes,recommend_time')
            ->where($where)
            ->order('isrecommend desc,recommend_time desc,hotvotes desc,starttime desc')
            ->limit(0,$pnum)
            ->fetchAll();
                    
		foreach($result as $k=>$v){
			$v=\App\handleLive($v);     
            $result[$k]=$v;
		}


		if($result){
			$last=end($result);

			$_SESSION['recommend_time']=$last['recommend_time'];
			$_SESSION['hotvotes']=$last['hotvotes'];
			$_SESSION['voicelive_starttime']=$last['starttime'];
		}

		return $result;
    }

    //获取首页推荐的我的关注主播列表
    public function getRecommendAttentLive($uid){

    	$rs=array(
    		'list'=>[],
    		'nums'=>'0'
    	);

    	$touid=\PhalApi\DI()->notorm->user_attention
				->select("touid")
				->where("uid=?",$uid)
				->fetchAll();
		
				
		if(!$touid){
            return $rs;
        }
        
        $where=" islive='1'";
    
        $touids=array_column($touid,"touid");
        $touidss=implode(",",$touids);
        $where.=" and uid in ({$touidss})";
        $result=\PhalApi\DI()->notorm->live
            ->select($this->live_fields)
            ->where($where)
            ->order("starttime desc")
            ->fetchAll();

        $list=[];

        if(!empty($result)){
        	foreach($result as $k=>$v){
            
				$v=\App\handleLive($v);
	            
	            $result[$k]=$v;
			}
        	$list = array_slice($result,0,3);
        }


        $rs['list']=$list;
        $rs['nums'] =(string)count($result);

		return $rs;
    }


    public function updateCity($uid,$city){
    	$res=\PhalApi\DI()->notorm->user
    		->where(['id'=>$uid])
    		->update(['city'=>$city]);

    	return $res;
    }
}
