<?php

/**
 * 充值规则
 */
namespace app\admin\controller;

use cmf\controller\AdminBaseController;
use think\facade\Db;

class ChargerulesController extends AdminBaseController {

		
    function index(){
        
        $lists = Db::name("charge_rules")
            ->where(['type'=>'0'])
			->order("list_order asc")
			->paginate(20);
        $lists->each(function($v,$k){
            return claw_normalize_charge_rule($v);
        });
        
        $page = $lists->render();

    	$this->assign('lists', $lists);

    	$this->assign("page", $page);
    	
    	return $this->fetch();
        
    }		
		
	function del(){
        $id = $this->request->param('id', 0, 'intval');
        
        $rs = DB::name('charge_rules')->where("id={$id}")->delete();
        if(!$rs){
            $this->error("删除失败！");
        }
        
        $action="删除充值规则：{$id}";
        setAdminLog($action);
                    
        $this->resetcache();
        $this->success("删除成功！",url("Chargerules/index"));			
	}
    
    //排序
    public function listOrder() { 
		
        $model = DB::name('charge_rules');
        parent::listOrders($model);
        
        $action="更新充值规则排序";
        setAdminLog($action);
        
        $this->resetcache();
        $this->success("排序更新成功！");
        
    }	

	
    function add(){
        $configpub=getConfigPub();
        $this->assign('name_coin',$configpub['name_coin']);
		return $this->fetch();
    }	
	
    function addPost(){
		if ($this->request->isPost()) {
            
            $data = $this->request->param();
            $configpub=getConfigPub();

            $name=$data['name'];
            $money=$data['money'];
            $coin=$data['coin'];
            $coin_ios=$data['coin_ios'];
            $product_id=$data['product_id'];
            $give=$data['give'];
            $coin_paypal=$data['coin_paypal'];

            if(!$name){
                $this->error("请填写名称");
            }

            if(!$money){
                $this->error("请填写价格");
            }

            if(!is_numeric($money)){
                $this->error("价格必须为数字");
            }

            if($money<=0||$money>99999999){
                $this->error("价格在0.01-99999999之间");
            }

            if(floor($money)!=(float)$money){
                $this->error("价格必须为整数星币，1星币=1USDT");
            }

            $starCoin=(int)$money;
            $data['money']=number_format($starCoin,2,'.','');
            $data['coin']=$starCoin;
            $data['coin_ios']=$starCoin;
            $data['coin_paypal']=$starCoin;
            $data['give']='0';
            $coin=$starCoin;
            $coin_ios=$starCoin;
            $coin_paypal=$starCoin;
            $give='0';
            if($product_id==''){
                $product_id='coin_'.$starCoin;
                $data['product_id']=$product_id;
            }

            if(!$coin){
                $this->error("请填写".$configpub['name_coin']);
            }

            if(!is_numeric($coin)){
                $this->error($configpub['name_coin']."必须为数字");
            }

            if($coin<1||$coin>99999999){
                $this->error($configpub['name_coin']."在1-99999999之间");
            }

            if(floor($coin)!=$coin){
                $this->error($configpub['name_coin']."必须为整数");
            }

            if($product_id==''){
                $this->error("苹果项目ID不能为空");
            }

            $data['addtime']=time();
            
			$id = DB::name('charge_rules')->insertGetId($data);
            if(!$id){
                $this->error("添加失败！");
            }
            
            $action="添加充值规则：{$id}";
            setAdminLog($action);
            
            $this->resetcache();
            $this->success("添加成功！");
            
		}
	}
    
    function edit(){
        $id   = $this->request->param('id', 0, 'intval');
        
        $data=Db::name('charge_rules')
            ->where("id={$id}")
            ->find();
        if(!$data){
            $this->error("信息错误");
        }

        $configpub=getConfigPub();
        $this->assign('name_coin',$configpub['name_coin']);
        $this->assign('name_score',$configpub['name_score']);
        if($data['type']==1){
            $hotGiftLists=Db::name("gift")
                ->field("id,giftname,gifticon")
                ->where("mark=1")
                ->order("list_order")
                ->select()
                ->toArray(); //热门礼物
            $this->assign('hotGiftLists',$hotGiftLists);
        }
        
        $this->assign('data', $data);
        return $this->fetch();
        
    }
	
    function editPost(){
		if ($this->request->isPost()) {
            
            $data = $this->request->param();

            $configpub=getConfigPub();

            $name=$data['name'];
            $name_en=$data['name_en'];
            $money=$data['money'];
            $coin=$data['coin'];
            $product_id=$data['product_id'];
            $type=$data['type'];


            if(!$name){
                $this->error("请填写名称");
            }

            if(!$money){
                $this->error("请填写价格");
            }

            if(!is_numeric($money)){
                $this->error("价格必须为数字");
            }

            if($money<=0||$money>99999999){
                $this->error("价格在0.01-99999999之间");
            }

            if(floor($money)!=(float)$money){
                $this->error("价格必须为整数星币，1星币=1USDT");
            }

            $starCoin=(int)$money;
            $data['money']=number_format($starCoin,2,'.','');
            $data['coin']=$starCoin;
            $data['coin_ios']=$starCoin;
            $data['coin_paypal']=$starCoin;
            $data['give']='0';
            $coin=$starCoin;
            if($product_id==''){
                $product_id='coin_'.$starCoin;
                $data['product_id']=$product_id;
            }

            if(!$coin){
                $this->error("请填写".$configpub['name_coin']);
            }

            if(!is_numeric($coin)){
                $this->error($configpub['name_coin']."必须为数字");
            }

            if($coin<1||$coin>99999999){
                $this->error($configpub['name_coin']."在1-99999999之间");
            }

            if(floor($coin)!=$coin){
                $this->error($configpub['name_coin']."必须为整数");
            }

            if($product_id==''){
                $this->error("苹果项目ID不能为空");
            }

            
            if($type!=0){

                $score=$data['score'];
                $vip_length=$data['vip_length'];
                $giftid=$data['giftid'];
                $gift_num=$data['gift_num'];

                //-----------
                if($score==''){
                   $this->error($configpub['name_score']."不能为空");
                }

                if(!is_numeric($score)){
                    $this->error($configpub['name_score']."必须为数字");
                }

                if($score<0||$score>99999999){
                    $this->error($configpub['name_score']."在0-99999999之间");
                }

                if(floor($score)!=$score){
                    $this->error($configpub['name_score']."必须为整数");
                }

                //-----------
                if($vip_length==''){
                   $this->error("赠送VIP时长不能为空");
                }

                if(!is_numeric($vip_length)){
                    $this->error("赠送VIP时长必须为数字");
                }

                if($vip_length<0||$vip_length>99999999){
                    $this->error("赠送VIP时长在0-99999999之间");
                }

                if(floor($vip_length)!=$vip_length){
                    $this->error("赠送VIP时长必须为整数");
                }

                //-------------
                if($giftid>0){
                    $gift_info=Db::name("gift")->where("id={$giftid} and mark=1")->find();
                   if(!$gift_info){
                        $this->error("热门礼物不存在");
                   }

                   if($gift_num=='' || $gift_num<=0){
                       $this->error("赠送热门礼物个数应为大于0的整数");
                    }
                }
               

               //-----------
                if($gift_num==''){
                   $this->error("赠送热门礼物个数不能为空");
                }

                if(!is_numeric($gift_num)){
                    $this->error("赠送热门礼物个数必须为数字");
                }

                if($gift_num<0||$gift_num>99999){
                    $this->error("赠送热门礼物个数在0-99999之间");
                }

                if(floor($gift_num)!=$gift_num){
                    $this->error("赠送VIP时长必须为整数");
                }

                if($giftid==0){
                    $data['gift_num']=0;
                }

               $data['coin_ios']=$coin;
               $data['coin_paypal']=$coin;
            }
            
            $data['uptime']=time();
            
			$rs = DB::name('charge_rules')->update($data);
            if($rs===false){
                $this->error("修改失败！");
            }
			
            $action="修改充值规则：{$data['id']}";
            setAdminLog($action);
            
            $this->resetcache();
            $this->success("修改成功！");
		}
	}

    //星币首充规则
    function firstcharge(){
        $lists = Db::name("charge_rules")
            ->where("type=1")
            ->order("list_order asc")
            ->paginate(20);

        $lists->each(function($v,$k){
            $v=claw_normalize_charge_rule($v);
            if($v['giftid']>0){
                $v['gift_info']=Db::name("gift")->where("id={$v['giftid']}")->find();  
            }else{
                $v['gift_info']=[];
            }
            
            return $v;           
        });
        
        $page = $lists->render();

        $this->assign('lists', $lists);

        $this->assign("page", $page);
        
        return $this->fetch();
    }
    	

    function resetcache(){
        $key='getChargeRules';
        $rules= DB::name("charge_rules")
            ->field('id,coin,coin_ios,money,product_id,give,coin_paypal')
            ->where("type=0")
            ->order('list_order asc')
            ->select()
            ->toArray();
        $rules=claw_normalize_charge_rules($rules);
        if($rules){
            setcaches($key,$rules);
        }else{
			delcache($key);
		}


        $key1='getFirstChargeRules';
        $first_rules= DB::name("charge_rules")
            ->field('id,name,name_en,coin,coin_ios,money,product_id,give,coin_paypal,score,vip_length,giftid,gift_num')
            ->where("type=1")
            ->order('list_order asc')
            ->select()
            ->toArray();
        $first_rules=claw_normalize_charge_rules($first_rules);
        if($first_rules){
            setcaches($key1,$first_rules);
        }else{
            delcache($key1);
        }
        return 1;
    }
}
