<?php

/**
 * 充值记录
 */
namespace app\admin\controller;

use cmf\controller\AdminBaseController;
use think\facade\Db;

class ChargeController extends AdminBaseController {
    protected function getStatus($k=''){
        $status=array(
            '0'=>'未支付',
            '1'=>'已完成',
        );
        if($k===''){
            return $status;
        }
        
        return $status[$k] ?? '';
    }
    
    protected function getTypes($k=''){
        $type=array(
            '1'=>'支付宝',
            '2'=>'微信',
            '3'=>'苹果支付',
            '4'=>'微信小程序',
            '5'=>'Paypal',
            '6'=>'Braintree Paypal',
            '7'=>'USDT',
        );
        if($k===''){
            return $type;
        }
        
        return $type[$k] ?? '';
    }
    
    protected function getAmbient($k=''){
        $ambient=array(
            "1"=>array(
                '0'=>'App',
                '1'=>'PC',
                '2'=>'H5',
            ),
            "2"=>array(
                '0'=>'App',
                '1'=>'公众号',
                '2'=>'PC',
                '3'=>'H5',
            ),
            "3"=>array(
                '0'=>'沙盒',
                '1'=>'生产',
            ),
            "4"=>array(
                '0'=>'App',
                '1'=>'PC',
            ),
            "5"=>array(
                '0'=>'沙盒',
                '1'=>'生产',
            ),
            "6"=>array(
                '0'=>'沙盒',
                '1'=>'生产',
            ),
            "7"=>array(
                '0'=>'App',
                '2'=>'PC',
                '3'=>'H5',
            ),
        );
        
        if($k===''){
            return $ambient;
        }
        
        return $ambient[$k] ?? '';
    }
    
    function index(){
        $data = $this->request->param();
        $map=[];
        
        $start_time= $data['start_time'] ?? '';
        $end_time= $data['end_time'] ?? '';

        if($start_time!=""){
           $map[]=['addtime','>=',strtotime($start_time)];
        }

        if($end_time!=""){
           $map[]=['addtime','<=',strtotime($end_time) + 60*60*24];
        }

        $status= $data['status'] ?? '';
        if($status!=''){
            $map[]=['status','=',$status];
        }

        $uid= $data['uid'] ?? '';
        if($uid!=''){
            $lianguid=getLianguser($uid);
            if($lianguid){
                
                array_push($lianguid,$uid);
                $map[]=['uid','in',$lianguid];
            }else{
                $map[]=['uid','=',$uid];
            }
        }

        $keyword= $data['keyword'] ?? '';
        if($keyword!=''){
            $map[]=['orderno|trade_no','like','%'.$keyword.'%'];
        }


        $lists = Db::name("charge_user")
            ->where($map)
			->order("id desc")
			->paginate(20);
        
        $lists->each(function($v,$k){
			$v['userinfo']=getUserInfo($v['uid']);
            if($v['giftid']){
                $gift_info=Db::name("gift")
                    ->field("gifticon,giftname")
                    ->where(['mark'=>1,'id'=>$v['giftid']])
                    ->find();
                if(!$gift_info){
                    $gift_info=['gifticon'=>'','giftname'=>''];
                }
            }else{
                $gift_info=['gifticon'=>'','giftname'=>''];
            }
            $v['gift_info']=$gift_info;
            return $v;           
        });
        
        $lists->appends($data);
        $page = $lists->render();

    	$this->assign('lists', $lists);
    	$this->assign("page", $page);
        $this->assign('status', $this->getStatus());
        $this->assign('type', $this->getTypes());
        $this->assign('ambient', $this->getAmbient());
    	
        $moneysum = Db::name("charge_user")
            ->where($map)
			->sum('money');
        if(!$moneysum){
            $moneysum=0;
        }

    	$this->assign('moneysum', $moneysum);
        
    	return $this->fetch();
    }
    
    function setPay(){
        $id = $this->request->param('id', 0, 'intval');
        if($id){
            $result=Db::name("charge_user")->where(["id"=>$id,"status"=>0])->find();                
            if($result){

                $uid=$result['touid'];
                
                /* 更新会员星币 */
                $coin=$result['coin'];
                Db::name("user")->where("id='{$uid}'")->inc("coin",$coin)->update();


                /* 更新 订单状态 */
                Db::name("charge_user")->where("id='{$result['id']}'")->update(array("status"=>1));

                if($result['is_first']==1){

                    $now=time();


                    if($result['score'] >0){
                        Db::name("user")->where("id='{$uid}'")->inc("score",$result['score'])->update();

                        $arr=array(
                            'type'=>1,
                            'action'=>'22',
                            'uid'=>$result['uid'],
                            'touid'=>$uid,
                            'giftid'=>$result['id'],
                            'giftcount'=>1,
                            'totalcoin'=>$result['score'],
                            'addtime'=>$now
                        );

                        Db::name("user_scorerecord")->insert($arr);
                    }

                    if($result['vip_length']>0){
                        $endtime=60*60*24*$result['vip_length'];
                        $vip_info=Db::name("vip_user")->where("uid={$uid}")->find();
                        if(!$vip_info){
                            $endtime=$endtime+$now;
                            Db::name("vip_user")->insert(
                                [
                                    'uid'=>$uid,
                                    'addtime'=>$now,
                                    'endtime'=>$endtime
                                ]
                            );
                        }else{
                            if($vip_info['endtime']>$now){
                                $endtime=$endtime+$vip_info['endtime'];
                            }else{
                                $endtime=$endtime+$now;
                            }

                            Db::name('vip_user')->where(["uid"=>$uid])
                                ->update(['endtime'=>$endtime]);
                        }

                        $key='vip_'.$uid;
                        $isexist=Db::name("vip_user")->where(["uid"=>$uid])->find();
                        if($isexist){
                            setcaches($key,$isexist);
                        }
                    }


                    if($result['giftid']>0 && $result['gift_num']>0){
                        $backpack_info=Db::name("backpack")
                            ->where(['uid'=>$uid,'giftid'=>$result['giftid']])
                            ->find();

                        if(!$backpack_info){
                            $arr=array(
                                'uid'=>$uid,
                                'giftid'=>$result['giftid'],
                                'nums'=>$result['gift_num']
                            );

                            Db::name("backpack")->insert($arr);
                        }else{
                            Db::name("backpack")
                                ->where(['uid'=>$uid,'giftid'=>$result['giftid']])
                                ->inc("nums",$result['gift_num'])
                                ->update();
                        }
                    }


                    Db::name("user")->where(['id'=>$uid])->update(array('firstcharge_used'=>1));


                }


   
                $action="确认充值：{$id}";
                setAdminLog($action);
                $this->success('操作成功');
             }else{
                $this->error('数据传入失败！');
             }          
        }else{              
            $this->error('数据传入失败！');
        }                                         
    }
    
    
    function del(){
        $id = $this->request->param('id', 0, 'intval');
        
        $rs = DB::name('charge_user')->where("id={$id}")->delete();
        if(!$rs){
            $this->error("删除失败！");
        }
        
        $action="删除充值记录：{$id}";
        setAdminLog($action);
                    
        $this->success("删除成功！");
        							  			
    }

    function export(){
    
        $data = $this->request->param();
        $map=[];
        
        $start_time= $data['start_time'] ?? '';
        $end_time= $data['end_time'] ?? '';

        if($start_time!=""){
           $map[]=['addtime','>=',strtotime($start_time)];
        }

        if($end_time!=""){
           $map[]=['addtime','<=',strtotime($end_time) + 60*60*24];
        }

        $status=$data['status'] ?? '';
        if($status!=''){
            $map[]=['status','=',$status];
        }

        $uid=$data['uid'] ?? '';
        if($uid!=''){
            $lianguid=getLianguser($uid);
            if($lianguid){
                
                array_push($lianguid,$uid);
                $map[]=['uid','in',$lianguid];
            }else{
                $map[]=['uid','=',$uid];
            }
        }

        $keyword= $data['keyword'] ?? '';
        if($keyword!=''){
            $map[]=['orderno|trade_no','like','%'.$keyword.'%'];
        }


        $xlsName  = "充值记录";

        $xlsData=Db::name("charge_user")
            ->field('id,uid,money,coin,orderno,type,trade_no,status,addtime')
            ->where($map)
            ->order('id desc')
			->select()
            ->toArray();

        if(empty($xlsData)){
            $this->error("数据为空");
        }
        
        foreach ($xlsData as $k => $v) {
            $userinfo=getUserInfo($v['uid']);
            $xlsData[$k]['user_nickname']= $userinfo['user_nickname']."(".$v['uid'].")";
            $xlsData[$k]['addtime']=date("Y-m-d H:i:s",$v['addtime']); 
            $xlsData[$k]['type']=$this->getTypes($v['type']);
            $xlsData[$k]['status']=$this->getStatus($v['status']);
        }

        $action="导出充值记录：".Db::name("charge_user")->getLastSql();
        setAdminLog($action);
        
        $cellName = array('A','B','C','D','E','F','G','H','I');
        $xlsCell  = array(
            array('id','序号'),
            array('user_nickname','会员'),
            array('money','星币金额'),
            array('coin','到账星币'),
            array('orderno','商户订单号'),
            array('type','支付类型'),
            array('trade_no','第三方支付订单号'),
            array('status','订单状态'),
            array('addtime','提交时间')
        );
        exportExcel($xlsName,$xlsCell,$xlsData,$cellName);
    }

}
