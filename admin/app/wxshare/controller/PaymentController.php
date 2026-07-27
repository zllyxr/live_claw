<?php
/**
 * 我的星币
 */
namespace app\wxshare\controller;

use app\common\controller\HomeBaseController;
use think\facade\Db;

class PaymentController extends HomebaseController {
	
    
	public function index() {	
		$uid=session('uid');
		if(!$uid){
			return $this->redirect('/wxshare/user/login');
		}
		
		//充值规则
    	$lists = Db::name("charge_rules")
			->where(['type'=>0])
			->order("list_order asc")
			->select()
			->toArray();
    	$lists=claw_normalize_charge_rules($lists);
    	$this->assign('lists', $lists);
		
		//我的星币
		$user=Db::name('user')
			->where("id={$uid}")
			->find();
		$this->assign("user",$user);
    	
		$configpri=getConfigPri();
		$this->assign('configpri',$configpri);
		
		return $this->fetch();
    }
	
	
	/* 获取订单号 */
	public function getOrderId(){
		$rs=array('code'=>0,'data'=>array(),'msg'=>'');
		$uid=(int)session("uid");
		$data = $this->request->param();
		if(!$uid){
			$rs['code']=700;
			$rs['msg']='登录信息失效，请重新登录';
            echo json_encode($rs);
            return;
		}
		
		
		
		
		$charge=Db::name('charge_rules')->where("id={$data['chargeid']}")->find();

		if(!$charge){
            $rs['code']=1002;
			$rs['msg']='订单信息错误';
            echo json_encode($rs);
            return;
        }
		$charge=claw_normalize_charge_rule($charge);
		$configpri=getConfigPri();
		
		
		$paytype=$data['paytype'] ?? '';
		$type=$paytype;
		if($type=='ali'){
			if(!$configpri['alih5_switch']){
				$rs['code']=1002;
				$rs['msg']='当前支付方式未开启';
				echo json_encode($rs);
				return;
			}
			
			
			
			$type=1;
			$ambient=2;
		}elseif($type=='wx'){
			
			if(!$configpri['wxgzh_switch']){
				$rs['code']=1002;
				$rs['msg']='当前支付方式未开启';
				echo json_encode($rs);
				return;
			}
			$type=2;
			$ambient=1;
		}elseif($type=='wxh5'){
			if(!$configpri['wxh5_switch']){
				$rs['code']=1002;
				$rs['msg']='当前支付方式未开启';
				echo json_encode($rs);
				return;
			}

			$type=2;
			$ambient=3;
		}elseif($type=='usdt'){
			$bepusdt=claw_get_bepusdt_config($configpri);
			if(empty($bepusdt['enabled'])){
				$rs['code']=1002;
				$rs['msg']='当前支付方式未开启';
				echo json_encode($rs);
				return;
			}

			$type=7;
			$ambient=3;
		}else{
            $rs['code']=1002;
			$rs['msg']='订单信息错误';
            echo json_encode($rs);
            return;
        }
		
        
        $orderid=$uid.'_'.date('YmdHis').rand(100,999);
        $orderinfo=array(
            "uid"		=>$uid,
            "touid"		=>$uid,
            "money"		=>$charge['money'],
            "coin"		=>$charge['coin'],
            "coin_give"	=>0,
            "orderno"	=>$orderid,
            "type"		=>$type,
            "ambient"	=>$ambient,
            "status"	=>'0',
            "addtime"	=>time()
        );
        $result=Db::name('charge_user')->insert($orderinfo);
        if(!$result){
            $rs['code']=1001;
            $rs['msg']='订单生成失败';
            echo json_encode($rs);
            return;
        }
            
        $rs['data']['uid']=$uid;
        $rs['data']['money']=$charge['money'];
        $rs['data']['coin']=$charge['coin'];
        $rs['data']['give']=0;
        $rs['data']['orderid']=$orderid;

		if($paytype=='usdt'){
			$configpub=getConfigPub();
			$host=rtrim(get_host(),'/');
			$order_name="充值".$charge['coin'].$configpub['name_coin'];
			$payResult=claw_bepusdt_create_transaction(
				$orderid,
				$charge['money'],
				$order_name,
				$host.'/appapi/pay/notify_usdt',
				$host.'/wxshare/payment/index'
			);

			if(($payResult['status_code'] ?? 0) != 200){
				$rs['code']=1005;
				$rs['msg']=$payResult['message'] ?? 'USDT订单创建失败';
				echo json_encode($rs);
				return;
			}

			$payData=$payResult['data'] ?? [];
			if(!empty($payData['trade_id'])){
				Db::name('charge_user')->where(['orderno'=>$orderid])->update(['trade_no'=>$payData['trade_id']]);
			}

			if(empty($payData['payment_url'])){
				$rs['code']=1005;
				$rs['msg']='USDT收银台地址为空';
				echo json_encode($rs);
				return;
			}

			$rs['data']['pay_url']=$payData['payment_url'];
			$rs['data']['trade_id']=$payData['trade_id'] ?? '';
			$rs['data']['actual_amount']=$payData['actual_amount'] ?? '';
			$rs['data']['token']=$payData['token'] ?? '';
		}

		echo json_encode($rs);
	}
	
	//支付宝手机支付
	public function alipay(){
		$data = $this->request->param();
		$uid=session('uid');
		if(!$uid){
			$reason='您的登陆状态失效，请重新登陆！';
			$this->assign('reason', $reason);
			return $this->fetch(':error');
		}
		
		require_once(CMF_ROOT."sdk/zfbpay/wappay/service/AlipayTradeService.php");
		require_once(CMF_ROOT.'sdk/zfbpay/wappay/buildermodel/AlipayTradeWapPayContentBuilder.php');
		require(CMF_ROOT.'sdk/zfbpay/config.php');
		
		
		$orderinfo=Db::name("charge_user")
			->where("orderno='{$data['orderno']}' and status='0'")
			->find();
		
		if(!$orderinfo){
			$reason='订单信息错误！';
			$this->assign('reason', $reason);
			return $this->fetch(':error');
		}
		
		//商户订单号，商户网站订单系统中唯一订单号，必填
		$out_trade_no = $orderinfo['orderno'];
		//订单名称，必填
		$subject = "充值星币";
		//付款金额，必填
		$total_amount = $orderinfo['money'];
		//商品描述，可空
		$body = "充值星币";
		$coin = $orderinfo['coin'];
		//超时时间
		$timeout_express="1m";
		
		$payRequestBuilder = new\AlipayTradeWapPayContentBuilder();
		$payRequestBuilder->setBody($body);
		$payRequestBuilder->setSubject($subject);
		$payRequestBuilder->setOutTradeNo($out_trade_no);
		$payRequestBuilder->setTotalAmount($total_amount);
		$payRequestBuilder->setTimeExpress($timeout_express);
		$payResponse = new\AlipayTradeService($config);
		$result=$payResponse->wapPay($payRequestBuilder,$config['return_url'],$config['notify_url']);
		return ;
	}
	public function alipay_notice(){
		require_once(CMF_ROOT."sdk/zfbpay/wappay/service/AlipayTradeService.php");
		require(CMF_ROOT.'sdk/zfbpay/config.php');
		if(!$_POST){
			$arr=$_GET;
		}else{
			$arr=$_POST;
		}
		

		
		$alipaySevice = new\AlipayTradeService($config); 
		$alipaySevice->writeLog(var_export($arr,true));
		$result = $alipaySevice->check($arr);
		/* 实际验证过程建议商户添加以下校验。
		1、商户需要验证该通知数据中的out_trade_no是否为商户系统中创建的订单号，
		2、判断total_amount是否确实为该订单的实际金额（即商户订单创建时的金额），
		3、校验通知中的seller_id（或者seller_email) 是否为out_trade_no这笔单据的对应的操作方（有的时候，一个商户可能有多个seller_id/seller_email）
		4、验证app_id是否为该商户本身。
		*/
		if($result && $arr['trade_status']=='TRADE_SUCCESS') {//验证成功
			/////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
			//请在这里加上商户的业务逻辑程序
			//——请根据您的业务逻辑来编写程序（以下代码仅作参考）——
			//获取支付宝的通知返回参数，可参考技术文档中服务器异步通知参数列表
			//商户订单号
			$out_trade_no = $arr['out_trade_no'];
			//支付宝交易号
			$trade_no = $arr['trade_no'];
			//消费金额
			$WIDtotal_amount=$arr['total_amount'];	

			
			$where['orderno']=$out_trade_no;
			$where['money']=$WIDtotal_amount;
			$where['type']=1;
			$where['ambient']=2;
			
			$data=[
				'trade_no'=>$trade_no
			];
			
			$res=handelCharge($where,$data);
			if($res==0){

				echo "fail";
				return;
			}
			echo "success";
		}else {
			//验证失败
			echo "fail";
		}		
	} 
	
	
	
	
	//微信h5支付
	public function wxh5pay(){
		
		$rs=array('code'=>0,'msg'=>'','info'=>array());
		
		$data = $this->request->param();
		$uid=session('uid');
		if(!$uid){
			$rs['code']=1001;
			$rs['msg']='您的登陆状态失效，请重新登陆！';
			echo json_encode($rs);
			exit;	
		}
		$orderinfo=Db::name("charge_user")
			->where("orderno='{$data['orderno']}' and status='0'")
			->find();
		
		if(!$orderinfo){
			$rs['code']=1001;
			$rs['msg']='订单信息有误！';
			echo json_encode($rs);
			exit;	
		}
		$configpri = getConfigPri(); 
		$configpub = getConfigPub();
		
		if($configpri['wx_h5_appid']== "" || $configpri['wx_h5_appsecret']== "" || $configpri['wx_h5_mchid']=="" || $configpri['wx_h5_key']== ""){
			$rs['code']=1001;
			$rs['msg']='微信支付未配置';
			echo json_encode($rs);
			exit;		 
		}
		
		
		$money=$orderinfo['money']*100; //总金额
		$noceStr = md5(rand(100,1000).time());//获取随机字符串
		
		$spbill_create_ip=$this->ip();
		$payinfo=array(
			'appid'=>$configpri['wx_h5_appid'],
			'mch_id'=>$configpri['wx_h5_mchid'],
			'nonce_str'=>$noceStr,
			'body'=>'充值星币',
			'out_trade_no'=>$orderinfo['orderno'],
			'total_fee'=>$money,
			'spbill_create_ip'=>$spbill_create_ip,
			'notify_url'=>$configpub['site'].'/wxshare/payment/notify_wx_h5',
			'trade_type'=>'MWEB',
			'scene_info'=>'{"h5_info": {"type":"Wap","wap_url": "'.$configpub['site'].'","wap_name": "'.$configpub['sitename'].'"}} ',
		);
		
		
		$sign=$this->sign($payinfo,$configpri['wx_h5_key']); //商户号的secret
		
		$payinfo['sign'] = $sign;
		$paramXml = "<xml>";
		foreach($payinfo as $k => $v){
			$paramXml .= "<" . $k . ">" . $v . "</" . $k . ">";
		}
		$paramXml .= "</xml>";
		
		$ch = curl_init ();
		@curl_setopt($ch, CURLOPT_SSL_VERIFYPEER, false); // 跳过证书检查  
		@curl_setopt($ch, CURLOPT_SSL_VERIFYHOST, true);  // 从证书中检查SSL加密算法是否存在  
		@curl_setopt($ch, CURLOPT_URL, "https://api.mch.weixin.qq.com/pay/unifiedorder");
		@curl_setopt($ch, CURLOPT_RETURNTRANSFER, true);
		@curl_setopt($ch, CURLOPT_POST, 1);
		@curl_setopt($ch, CURLOPT_POSTFIELDS, $paramXml);
		//@$resultXmlStr = curl_exec($ch);

		$return = curl_exec($ch);
		
		curl_close($ch);

		$dataxml = json_decode(json_encode(simplexml_load_string($return, 'SimpleXMLElement', LIBXML_NOCDATA)), true);//转成数组，
		if($dataxml['return_code']=='SUCCESS' && $dataxml['return_msg']=='OK'){
			$rs['info'][0]['url']=$dataxml['mweb_url'];
			echo json_encode($rs);
			exit;	
		}else{
			$rs['cod']=1001;
			$rs['msg']=$dataxml['return_msg'];
			echo json_encode($rs);
			exit;	
		}
	}
	
	
	//获取ip
	public function ip(){
		$ip = $_SERVER['REMOTE_ADDR'];
		if (isset($_SERVER['HTTP_X_FORWARDED_FOR']) && preg_match_all('#\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}#s', $_SERVER['HTTP_X_FORWARDED_FOR'], $matches)) {
			foreach ($matches[0] AS $xip) {
				if (!preg_match('#^(10|172\.16|192\.168)\.#', $xip)) {
					$ip = $xip;
					break;
				}
			}
		} elseif (isset($_SERVER['HTTP_CLIENT_IP']) && preg_match('/^([0-9]{1,3}\.){3}[0-9]{1,3}$/', $_SERVER['HTTP_CLIENT_IP'])) {
			$ip = $_SERVER['HTTP_CLIENT_IP'];
		} elseif (isset($_SERVER['HTTP_CF_CONNECTING_IP']) && preg_match('/^([0-9]{1,3}\.){3}[0-9]{1,3}$/', $_SERVER['HTTP_CF_CONNECTING_IP'])) {
			$ip = $_SERVER['HTTP_CF_CONNECTING_IP'];
		} elseif (isset($_SERVER['HTTP_X_REAL_IP']) && preg_match('/^([0-9]{1,3}\.){3}[0-9]{1,3}$/', $_SERVER['HTTP_X_REAL_IP'])) {
			$ip = $_SERVER['HTTP_X_REAL_IP'];
		}
		
		
		return $ip;
	}
	
	/**
	* sign拼装获取
	*/
	public function sign($param,$key){
		ksort($param);
		$sign = "";
		foreach($param as $k => $v){
			$sign .= $k."=".$v."&";
		}
		$sign .= "key=".$key;

		$sign = strtoupper(md5($sign));
		return $sign;
	
	}
	
	/**
	* xml转为数组
	*/
	public function xmlToArray($xmlStr){
		$msg = array(); 
		$postStr = $xmlStr; 
		$msg = (array)simplexml_load_string($postStr, 'SimpleXMLElement', LIBXML_NOCDATA); 
		return $msg;
	}
	
	
	/* 微信H5支付---回调 */		
	public function notify_wx_h5(){
		$config=getConfigPri();
		$configpub=getConfigPub();
		$xmlInfo=file_get_contents("php://input"); 

		//解析xml
		$arrayInfo = $this -> xmlToArray($xmlInfo);
		$this -> wxDate = $arrayInfo;
		$this -> logwxh5("wx_data:".json_encode($arrayInfo));//log打印保存
		if($arrayInfo['return_code'] == "SUCCESS"){
			if($return_msg != null){
				echo $this -> returnInfo("FAIL","签名失败");
				$this -> logwxh5("签名失败:".$sign);//log打印保存
				exit;
			}else{
				$wxSign = $arrayInfo['sign'];
				unset($arrayInfo['sign']);
				$arrayInfo['appid']  =  $config['wx_h5_appid'];
				$arrayInfo['mch_id'] =  $config['wx_h5_mchid'];
				$key =  $config['wx_h5_appsecret'];
				ksort($arrayInfo);//按照字典排序参数数组
				$sign = $this -> sign($arrayInfo,$key);//生成签名
				$this -> logwxh5("数据打印测试签名signmy:".$sign.":::微信sign:".$wxSign);//log打印保存
				if($this -> checkSign($wxSign,$sign)){
					echo $this -> returnInfo("SUCCESS","OK");
					$this -> logwxh5("签名验证结果成功:".$sign);//log打印保存
					$this -> orderServerh5();//订单处理业务逻辑
					exit;
				}else{
					echo $this -> returnInfo("FAIL","签名失败");
					$this -> logwxh5("签名验证结果失败:本地加密：".$sign.'：：：：：三方加密'.$wxSign);//log打印保存
					exit;
				}
			}
		}else{
			echo $this -> returnInfo("FAIL","签名失败");
			$this -> logwx($arrayInfo['return_code']);//log打印保存
			exit;
		}			
	}
	
	
	public function returnInfo($type,$msg){
		if($type == "SUCCESS"){
			return $returnXml = "<xml><return_code><![CDATA[{$type}]]></return_code></xml>";
		}else{
			return $returnXml = "<xml><return_code><![CDATA[{$type}]]></return_code><return_msg><![CDATA[{$msg}]]></return_msg></xml>";
		}
	}	
	
	//签名验证
	public function checkSign($sign1,$sign2){
		return trim($sign1) == trim($sign2);
	}
	
	/* 订单查询加值业务处理
	 * @param orderNum 订单号	   
	 */
	public function orderServerh5(){
		$info = $this -> wxDate;
		$this->logwxh5("info:".json_encode($info));
		$orderinfo=M("users_charge")->where("orderno='{$info['out_trade_no']}' and status='0' and type='2'")->find();
		//$this->logwx("sql:".M()->getLastSql());
		$this->logwxh5("orderinfo:".json_encode($orderinfo));
		if($orderinfo){
			/* 更新会员虚拟币 */
			$coin=$orderinfo['coin'];
			M("users")->where("id='{$orderinfo['touid']}'")->setInc("coin",$coin);
			/* 更新 订单状态 */
			M("users_charge")->where("id='{$orderinfo['id']}'")->save(array("status"=>1,"trade_no"=>$info['transaction_id']));
            $this->logwxh5("orderno:".$out_trade_no.' 支付成功');
		}else{
			$this->logwxh5("orderno:".$out_trade_no.' 订单信息不存在');		
			return false;
		}		

	}


}


