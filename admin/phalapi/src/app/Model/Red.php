<?php
namespace App\Model;
use PhalApi\Model\NotORMModel as NotORM;

class Red extends NotORM {
	/* 发布红包 */
	public function sendRed($data) {
        
        $rs = array('code' => 0, 'msg' => \PhalApi\T('发送成功'), 'info' => array());
        
        $uid=$data['uid'];
        $total=$data['coin'];
        $ifok=\PhalApi\DI()->notorm->user
				->where('id = ? and coin >= ?', $uid,$total)
				->update(
                    array(
                        'coin' => new \NotORM_Literal("coin - {$total}"),
                        'consumption' => new \NotORM_Literal("consumption + {$total}")
                    )
                );

        if(!$ifok){
            $rs['code']=1009;
            $rs['msg']=\PhalApi\T('余额不足');		
            return $rs;
        }
        
        $result= \PhalApi\DI()->notorm->red->insert($data);
        
        if(!$result){
            $rs['code']=1009;
            $rs['msg']=\PhalApi\T('发送失败，请重试');		
            return $rs;
        }
        
        $type='0';
        $action='8';
        $uid=$data['uid'];
        $giftid=$result['id'];
        $giftcount=1;
        $total=$data['coin'];
        $showid=$data['showid'];
        $addtime=$data['addtime'];
        
        
        $insert=array(
            "type"=>$type,
            "action"=>$action,
            "uid"=>$uid,
            "touid"=>$uid,
            "giftid"=>$giftid,
            "giftcount"=>$giftcount,
            "totalcoin"=>$total,
            "showid"=>$showid,
            "addtime"=>$addtime
        );

		\PhalApi\DI()->notorm->user_coinrecord->insert($insert);

		$rs['info']=$result;
		return $rs;
	}		
    
    /* 红包列表 */
    public function getRedList($liveuid,$showid){
        $list=\PhalApi\DI()->notorm->red
                ->select("*")
                ->where('liveuid = ? and showid= ?',$liveuid,$showid)
                ->order('addtime desc')
                ->fetchAll();
        return $list;
    }

	/* 抢红包 */
	public function robRed($data) {
        $type='1';
        $action='9';
        $uid=$data['uid'];
        $giftid=$data['redid'];
        $giftcount=1;
        $total=$data['coin'];
        $showid=$data['showid'];
        $addtime=$data['addtime'];
        unset($data['showid']);
        
        
        $insert=array(
            "type"=>$type,
            "action"=>$action,
            "uid"=>$uid,
            "touid"=>$uid,
            "giftid"=>$giftid,
            "giftcount"=>$giftcount,
            "totalcoin"=>$total,
            "showid"=>$showid,
            "addtime"=>$addtime
        );

		\PhalApi\DI()->notorm->user_coinrecord->insert($insert);

		$result= \PhalApi\DI()->notorm->red_record->insert($data);
        
        
        
        \PhalApi\DI()->notorm->user
				->where('id = ?', $uid)
				->update(
                    array(
                        'coin' => new \NotORM_Literal("coin + {$total}")
                    )
                );

        \PhalApi\DI()->notorm->red
				->where('id = ?', $giftid)
				->update(
                    array(
                        'coin_rob' => new \NotORM_Literal("coin_rob + {$total}"),
                        'nums_rob' => new \NotORM_Literal("nums_rob + 1")
                    )
                );
                
		return $result;
	}			

    /* 抢红包列表 */
    public function getRedRobList($redid){
        $list=\PhalApi\DI()->notorm->red_record
                ->select("*")
                ->where('redid = ?',$redid)
                ->order('addtime desc')
                ->fetchAll();
        return $list;
    }
    
    /* 红包信息 */
    public function getRedInfo($redid){
        $redinfo=\PhalApi\DI()->notorm->red
                ->select("*")
                ->where('id = ? ',$redid )
                ->fetchOne();
        if($redinfo){
            unset($redinfo['showid']);
            unset($redinfo['liveuid']);
            unset($redinfo['effecttime']);
            unset($redinfo['addtime']);
            unset($redinfo['status']);
        }
        return $redinfo;
        
    }
}
