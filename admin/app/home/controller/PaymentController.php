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

class PaymentController extends HomebaseController {
	
    
	public function index() {	
		LogIn();
		$uid=(int)session("uid");
        if(!$uid){
            $this->error('请先登录');
        }
    	$lists = Db::name("charge_rules")
			->where(['type'=>0])
			->order("list_order asc")
			->select()
            ->toArray();
        $lists=claw_normalize_charge_rules($lists);
                
    	$this->assign('lists', $lists);
        
		$user=Db::name('user')->where("id={$uid}")->find();
        
		$this->assign("user",$user);
    	return $this->fetch();
    }

	//调用微信或支付宝扫码支付
  	public function chargepay(){

        $uid=session("uid");
        if($uid<1){
        	$this->assign("jumpUrl",'/');
            $this->error("您的登陆状态失效，请重新登陆！");
        }
        
        $changeid = $this->request->param('changeid', 0, 'intval');
		
        if(!$changeid){
        	$this->assign("jumpUrl",'/');
            $this->error("参数错误");
        }
		$where['id']=$changeid;
		$charge=Db::name("charge_rules")->where($where)->find();
		if(!$charge ){
			$this->assign("jumpUrl",'/');
			$this->error("订单信息有误，请重新提交");
		}
        $charge=claw_normalize_charge_rule($charge);

		$money = $charge['money'];
		$coin = $charge['coin'];
		$give = 0;
		//读取后台配置信息
		$configpri=getConfigPri();	
		$configpub=getConfigPub();
		//当前域名
		$pay_url=$configpub['site']; 
		//商户订单号 //便于筛选，订单号为 uid_touid_时间戳_随机数
		$orderid = $uid."_".date("YmdHis").rand(100,999);
        $paytype=$_POST['c_PPPayID'];
        //订单名称
		$order_name = "充值".$coin.$configpub['name_coin'];

		if($paytype == 'usdt'){
			$bepusdt=claw_get_bepusdt_config($configpri);
			if(empty($bepusdt['enabled'])){
				$this->error("USDT支付未开启");
			}

			$data=array(
				'touid' =>$uid,
				'uid'=>$uid,
				'money' => $money,
				'coin' =>$coin,
				'coin_give' =>$give,
				'trade_no'=>'',
				'orderno'=>$orderid,
				'status'=>0,
				'addtime'=>time(),
				'type'=>7,
				'ambient'=>2,
			);
			$res=Db::name("charge_user")->insert($data);
			if(!$res){
				$this->error("订单创建失败，请重新提交");
			}

			$host=rtrim(get_host(),'/');
			$result=claw_bepusdt_create_transaction(
				$orderid,
				$money,
				$order_name,
				$host."/appapi/pay/notify_usdt",
				$host."/home/Payment/index"
			);

			if(($result['status_code'] ?? 0) != 200){
				$this->error($result['message'] ?? "USDT订单创建失败");
			}

			$payData=$result['data'] ?? [];
			if(!empty($payData['trade_id'])){
				Db::name("charge_user")->where(['orderno'=>$orderid])->update(['trade_no'=>$payData['trade_id']]);
			}

			if(empty($payData['payment_url'])){
				$this->error("USDT收银台地址为空");
			}

			return redirect($payData['payment_url']);
		}

		if($paytype == 'weixin'){

			require_once CMF_ROOT."sdk/wxpay/lib/WxPay.Api.php"; 
			require_once CMF_ROOT."sdk/wxpay/pay/WxPay.NativePay.php";

			//debug_backtrace
			$notify = new \NativePay();

			//支付记录
			$money2 = $money*100;
			$input = new \WxPayUnifiedOrder();
			$input->SetBody($order_name);
			$input->SetAttach($order_name);
			$input->SetOut_trade_no($orderid);
			$input->SetTotal_fee($money2);
			$input->SetTime_start(date("YmdHis"));
			$input->SetTime_expire(date("YmdHis", time() + 600));
			$input->SetGoods_tag("test");
			$input->SetNotify_url($pay_url."/appapi/wxpay/notify_native");
			$input->SetTrade_type("NATIVE");
			$input->SetProduct_id("123456789");
			$result = $notify->GetPayUrl($input);

			if($result['return_code']=='FAIL'){
 
				echo '<html>
				    <head><meta http-equiv="Content-Type" content="text/html; charset=UTF-8"></head>
				    <body>
				      <p>'.$result['return_msg'].',请检查微信支付信息是否配置'.'</p></body></html>';
                return;
			}

			///支付记录
            //touid 赠送人id money充值金额 coin兑换点数 orderno商户订单号 trade_no第三方订单号 status订单支付状态 addtime订单提交时间 type支付方式(1 支付宝2微信 3苹果支付)
			$data=array(
				'touid' =>$uid,
				'uid'=>$uid,
				'money' => $money,
				'coin' =>$coin,
				'coin_give' =>$give,
				'trade_no'=>'',
				'orderno'=>$orderid,
				'status'=>0,		
				'addtime'=>time(),
				'type'=>2,
				'ambient'=>2,
			);	
			$userid=Db::name("charge_user")->insert($data);

			$url2 = $result["code_url"];        			
			echo '<html>
				    <head><meta http-equiv="Content-Type" content="text/html; charset=UTF-8"></head>
				    <body>
				      <form name="form1" id="form1" method="post" action="/home/Payment/wxpay" target="_self">
								<input type="hidden" name="url" value="'. $url2.'" />
								<input type="hidden" name="money" value="'. $money.'" />
								<input type="hidden" name="coin" value="'. $coin.'" />
								<input type="hidden" name="orderid" value="'. $orderid.'" />
								<script language="javascript">document.form1.submit();</script>
							</form></body></html>
						';	
            return;
        }
        
		//支付宝扫码支付===========================
        if($paytype == 'zhifubao'){

        	$alipay_type=$configpri['alipay_pc_type'];

        	//旧版
        	if($alipay_type==0){

        		//获取后台设置的 配置信息
				/*  $siteconfig=M("siteconfig")->where("id='1'")->find(); */
				//↓↓↓↓↓↓↓↓↓↓请在这里配置您的基本信息↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓
				//合作身份者id，以2088开头的16位纯数字
				$alipay_config['partner']=$configpri['aliapp_partner'];

				//安全检验码，以数字和字母组成的32位字符
				$alipay_config['key']			= $configpri['aliapp_check'];
				//支付宝账号
				$alipay_config['seller_email'] =$configpri['aliapp_seller_id'];
				//↑↑↑↑↑↑↑↑↑↑请在这里配置您的基本信息↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑
				//签名方式 不需修改
				$alipay_config['sign_type']    = strtoupper('MD5');
				//字符编码格式 目前支持 gbk 或 utf-8
				$alipay_config['input_charset']= strtolower('utf-8');
				//ca证书路径地址，用于curl中ssl校验
				//请保证cacert.pem文件在当前文件夹目录中
				$alipay_config['cacert']    = CMF_ROOT.'sdk/alipay/cacert.pem';
				//访问模式,根据自己的服务器是否支持ssl访问，若支持请选择https；若不支持请选择http
				$alipay_config['transport']    = 'http';
				
				//↓↓↓↓↓↓↓↓↓↓请在这里配置您的基本信息↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓
				require_once CMF_ROOT."sdk/alipay/lib/alipay_submit.class.php";
				//支付记录
				//touid 赠送人id money充值金额 coin兑换点数 orderno商户订单号 trade_no第三方订单号 status订单支付状态 addtime订单提交时间 type支付方式(1 支付宝2微信 3苹果支付)
				$data=array(
						'touid'     =>$uid,
						'uid'       =>$uid,
						'money'     => $money,
						'coin'      =>$coin,
						'coin_give' =>$give,
						'trade_no'  =>'',
						'orderno'   =>$orderid,
						'status'    =>0,
						'addtime'   =>time(),
						'type'      =>1,
						'ambient'   =>1,
					);	
				$userid=Db::name("charge_user")->insert($data);
	            //支付记录					
				/**************************请求参数**************************/
				//支付类型
				$payment_type = "1";
				//必填，不能修改
				//服务器异步通知页面路径
				$notify_url = $pay_url."/home/Payment/alipay_d_notify";
				//需http://格式的完整路径，不能加?id=123这类自定义参数
				//页面跳转同步通知页面路径
				$return_url = $pay_url."/home/Payment/index";
				//需http://格式的完整路径，不能加?id=123这类自定义参数，不能写成http://localhost/
				//商户网站订单系统中唯一订单号，必填
				//订单名称
				$subject ="支付宝充值".$configpub['name_coin'];
				//付款金额
				$total_fee =$money;
				//订单描述
				$body = "充值".$configpub['name_coin'].",价格为".$coin;
				//商品展示地址
				$show_url =$pay_url;
				//需以http://开头的完整路径，例如：http://www.xxx.com/myorder.html
				//防钓鱼时间戳
				$anti_phishing_key = "";
				//若要使用请调用类文件submit中的query_timestamp函数
				//客户端的IP地址
				$exter_invoke_ip = "";
				//非局域网的外网IP地址，如：221.0.0.1
				/************************************************************/
				//构造要请求的参数数组，无需改动
					$parameter = array(
						"service"           => "create_direct_pay_by_user",
						"partner"           => trim($alipay_config['partner']),
						"payment_type"	    => $payment_type,
						"notify_url"	    => $notify_url,
						"return_url"	    => $return_url,
						"seller_email"	    => trim($alipay_config['seller_email']),
						"out_trade_no"	    => $orderid,
						"subject"	        => $subject,
						"total_fee"	        => $total_fee,
						"body"	            => $body,
						"show_url"	        => $show_url,
						"qr_pay_mode"       =>2,
						"anti_phishing_key"	=> $anti_phishing_key,
						"exter_invoke_ip"	=> $exter_invoke_ip,
						"_input_charset"	=> trim(strtolower($alipay_config['input_charset']))
					);
					
				//建立请求
				$alipaySubmit = new \AlipaySubmit($alipay_config);
				$html_text = $alipaySubmit->buildRequestForm($parameter,"get", "1"); 
				/*  $html_text = $alipaySubmit->buildRequestPara($parameter); */      
				echo $html_text;

        	}else{
        		header('Content-type:text/html; Charset=utf-8');
				require_once CMF_ROOT.'sdk/alipagepay/pagepay/service/AlipayTradeService.php';
				require_once CMF_ROOT.'sdk/alipagepay/pagepay/buildermodel/AlipayTradePagePayContentBuilder.php';

				$data=array(
					'touid' 	=>$uid,
					'uid'		=>$uid,
					'money' 	=> $money,
					'coin' 		=>$coin,
					'coin_give' =>$give,
					'trade_no'	=>'',
					'orderno'	=>$orderid,
					'status'	=>0,
					'addtime'	=>time(),
					'type'		=>1,
					'ambient'	=>1,
				);	
				$res=Db::name("charge_user")->insert($data);

				if(!$res){
					$this->assign("jumpUrl",'/');
					$this->error("订单创建失败，请重新提交");
				}

				$config=$this->getaliPagePayConfig();

				//商户订单号，商户网站订单系统中唯一订单号，必填
			    $out_trade_no = trim($orderid);

			    //订单名称，必填
			    $subject = trim($order_name);

			    //付款金额，必填
			    $total_amount = trim($money);

			    //商品描述，可空
			    $body = trim($order_name);

				//构造参数
				$payRequestBuilder = new \AlipayTradePagePayContentBuilder();
				$payRequestBuilder->setBody($body);
				$payRequestBuilder->setSubject($subject);
				$payRequestBuilder->setTotalAmount($total_amount);
				$payRequestBuilder->setOutTradeNo($out_trade_no);

				$aop = new \AlipayTradeService($config);

				$response = $aop->pagePay($payRequestBuilder,$config['return_url'],$config['notify_url']);

				//输出表单
				var_dump($response);
				return;
        	}

			
		}

	}

	/**
	 * 获取新版支付宝支付配置信息
	 * @return array 配置信息
	 */
	public function getaliPagePayConfig(){
		$configpub=getConfigPub();
		$configpri=getConfigPri();

		$alipay_public_key_path=CMF_ROOT.'sdk/alipagepay/key/publickey.php';
		require $alipay_public_key_path;
		

		$config = array (	
			//应用ID,您的APPID。
			'app_id' => $configpri['ali_application_appid'],

			//商户私钥
			'merchant_private_key' => $configpri['ali_key_pc'],
			
			//异步通知地址
			'notify_url' => $configpub['site']."/home/Payment/alipay_notify",
			
			//同步跳转
			'return_url' => $configpub['site']."/home/Payment/alipayres",

			//编码格式
			'charset' => "UTF-8",

			//签名方式
			'sign_type'=>"RSA2",

			//支付宝网关
			'gatewayUrl' => "https://openapi.alipay.com/gateway.do",

			//支付宝公钥,查看地址：https://openhome.alipay.com/platform/keyManage.htm 对应APPID下的支付宝公钥。
			'alipay_public_key' =>$publicKey,
			

	        //日志路径
	        'log_path' => CMF_ROOT.'log/think/home/payment/alipagepay_'.date("Y-m-d").'.txt',
		);

		return $config;
	}
	
	//支付宝支付同步回调地址
    function alipayres(){
    	require_once CMF_ROOT.'sdk/alipagepay/pagepay/service/AlipayTradeService.php';
    	$config=$this->getaliPagePayConfig();

			$arr=$_GET;
			$this->logali("同步回调信息");
			$this->logali(json_encode($arr));

            if(empty($arr['out_trade_no']) || empty($arr['trade_no']) || empty($arr['total_amount']) || empty($arr['app_id'])){
                $this->assign('status',0);
                $this->assign('orderinfo',[]);
                return $this->fetch();
            }

			$alipaySevice = new \AlipayTradeService($config); 
			$result = $alipaySevice->check($arr);
		$this->logali("同步回调信息验证结果：".json_encode($result));

		/* 实际验证过程建议商户添加以下校验。
		1、商户需要验证该通知数据中的out_trade_no是否为商户系统中创建的订单号，
		2、判断total_amount是否确实为该订单的实际金额（即商户订单创建时的金额），
		3、校验通知中的seller_id（或者seller_email) 是否为out_trade_no这笔单据的对应的操作方（有的时候，一个商户可能有多个seller_id/seller_email）
		4、验证app_id是否为该商户本身。
		*/
		if($result) {//验证成功
			/////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
			//请在这里加上商户的业务逻辑程序代码

			//——请根据您的业务逻辑来编写程序（以下代码仅作参考）——
		    //获取支付宝的通知返回参数，可参考技术文档中页面跳转同步通知参数列表

			//商户订单号
			$out_trade_no = htmlspecialchars($arr['out_trade_no']);

			//支付宝交易号
			$trade_no = htmlspecialchars($arr['trade_no']);

			//交易金额
			$total_amount =htmlspecialchars($arr['total_amount']);

			//应用appid
			$app_id=htmlspecialchars($arr['app_id']);

			$orderinfo=Db::name("charge_user")->where(['orderno'=>$out_trade_no,'type'=>1,'ambient'=>1,'money'=>$total_amount])->find();

			$this->logali("同步回调订单内容：".json_encode($orderinfo));

			if(!$orderinfo){
				//订单不存在
				$status=-1;
				$orderinfo=array();

			}else{

				//订单已支付
				if($orderinfo['status']==0){
					$status=0;
					$orderinfo=array();
				}else{
					$status=1;
				}

			}


		}else {
		    //验证失败
		    $status=0;
		    $orderinfo=array();
		}


		$this->assign('status',$status);
		$this->assign('orderinfo',$orderinfo);
		return $this->fetch();
    }

    /**
     * 支付宝异步回调地址
     * @return string 支付状态 success:成功 fail:失败
     */
    function alipay_notify(){
    	

		 /*************************页面功能说明*************************
		 * 如果没有收到该页面返回的 success 信息，支付宝会在24小时内按一定的时间策略重发通知
		 */

		require_once CMF_ROOT.'sdk/alipagepay/pagepay/service/AlipayTradeService.php';
		$config=$this->getaliPagePayConfig();

		$arr=$_POST;
		$this->logali("异步回调信息：".json_encode($arr));
		$alipaySevice = new \AlipayTradeService($config);

		//$alipaySevice->writeLog(var_export($_POST,true));
		$result = $alipaySevice->check($arr);


		$this->logali("异步回调信息验证结果：".json_encode($result));
		/* 实际验证过程建议商户添加以下校验。
		1、商户需要验证该通知数据中的out_trade_no是否为商户系统中创建的订单号，
		2、判断total_amount是否确实为该订单的实际金额（即商户订单创建时的金额），
		3、校验通知中的seller_id（或者seller_email) 是否为out_trade_no这笔单据的对应的操作方（有的时候，一个商户可能有多个seller_id/seller_email）
		4、验证app_id是否为该商户本身。
		*/
		if($result) {//验证成功
			/////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
			//请在这里加上商户的业务逻辑程序代

			
			//——请根据您的业务逻辑来编写程序（以下代码仅作参考）——
			
		    //获取支付宝的通知返回参数，可参考技术文档中服务器异步通知参数列表
			
			//商户订单号
			$out_trade_no = $arr['out_trade_no'];

			//支付宝交易号
			$trade_no = $arr['trade_no'];

			//交易状态
			$trade_status = $arr['trade_status'];

			//交易金额
			$total_amount=$arr['total_amount'];


		    if($arr['trade_status'] == 'TRADE_FINISHED') {

				//判断该笔订单是否在商户网站中已经做过处理
					//如果没有做过处理，根据订单号（out_trade_no）在商户网站的订单系统中查到该笔订单的详细，并执行商户的业务程序
					//请务必判断请求时的total_amount与通知时获取的total_fee为一致的
					//如果有做过处理，不执行商户的业务程序
						
				//注意：
				//退款日期超过可退款期限后（如三个月可退款），支付宝系统发送该交易状态通知
				
		    }else if ($arr['trade_status'] == 'TRADE_SUCCESS') {
				//判断该笔订单是否在商户网站中已经做过处理
					//如果没有做过处理，根据订单号（out_trade_no）在商户网站的订单系统中查到该笔订单的详细，并执行商户的业务程序
					//请务必判断请求时的total_amount与通知时获取的total_fee为一致的
					//如果有做过处理，不执行商户的业务程序			
				//注意：
				//付款完成后，支付宝系统发送该交易状态通知
				
				$where['orderno']=$out_trade_no;
                $where['money']=$total_amount;
                $where['type']=1;
                $where['ambient']=1;
                
                $data=[
                    'trade_no'=>$trade_no
                ];

                //$alipaySevice->writeLog(var_export($where,true));
                $this->logali("异步回调，订单查询条件：".json_encode($where));
                $res=handelCharge($where,$data);
				if($res==0){
                    $this->logali("异步回调，orderno:".$out_trade_no.' 订单信息不存在');	
                    echo "fail";
                    return;
				}

				$this->logali("异步回调，订单支付成功");
				echo "success";	//请不要修改或删除

		    }
			//——请根据您的业务逻辑来编写程序（以上代码仅作参考）——
			//echo "success";	//请不要修改或删除
		}else {
		    //验证失败
		    $this->logali("异步回调验证失败");
		    echo "fail";
		}	

    }

	/**
	 * 旧版支付宝即时到帐  返回处理
	 */
	public function alipay_d_notify(){
        
		//读取后台配置信息
		$getConfigPri=getConfigPri();	
		$getConfigPub=getConfigPub();
		//↓↓↓↓↓↓↓↓↓↓请在这里配置您的基本信息↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓
		//合作身份者id，以2088开头的16位纯数字
		$alipay_config['partner']=$getConfigPri['aliapp_partner'];
		//安全检验码，以数字和字母组成的32位字符
		$alipay_config['key']			= $getConfigPri['aliapp_check'];
		//支付宝账号
		$alipay_config['seller_email'] =$getConfigPri['aliapp_seller_id'];
		//↑↑↑↑↑↑↑↑↑↑请在这里配置您的基本信息↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑
		//签名方式 不需修改
		$alipay_config['sign_type']    = strtoupper('MD5');
		//字符编码格式 目前支持 gbk 或 utf-8
		$alipay_config['input_charset']= strtolower('utf-8');
		//ca证书路径地址，用于curl中ssl校验
		//请保证cacert.pem文件在当前文件夹目录中
		$alipay_config['cacert']    = CMF_ROOT.'sdk/alipay/cacert.pem';
		//访问模式,根据自己的服务器是否支持ssl访问，若支持请选择https；若不支持请选择http
		$alipay_config['transport']    = 'http';
		//↓↓↓↓↓↓↓↓↓↓请在这里配置您的基本信息↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓	
		require_once(CMF_ROOT."sdk/alipay/lib/alipay_notify.class.php");
		//计算得出通知验证结果
		$alipayNotify = new \AlipayNotify($alipay_config);
		$verify_result = $alipayNotify->verifyNotify();
		if($verify_result) {//验证成功
			/////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
			//请在这里加上商户的业务逻辑程序代
			//——请根据您的业务逻辑来编写程序（以下代码仅作参考）——
			//获取支付宝的通知返回参数，可参考技术文档中服务器异步通知参数列表
			//session("uid")."_".session("uid")."_".date("mdHis")."_".rand(999,9999); 
			//商户订单号
			$out_trade_no = $_POST['out_trade_no'];
			//支付宝交易号
			$trade_no = $_POST['trade_no'];
			//交易状态
			$trade_status = $_POST['trade_status'];
			//交易金额
			$total_fee = $_POST['total_fee'];

			if($trade_status == 'TRADE_FINISHED') {
				//判断该笔订单是否在商户网站中已经做过处理
				//如果没有做过处理，根据订单号（out_trade_no）在商户网站的订单系统中查到该笔订单的详细，并执行商户的业务程序
				//如果有做过处理，不执行商户的业务程序
					
				//注意：
				//退款日期超过可退款期限后（如三个月可退款），支付宝系统发送该交易状态通知
				//请务必判断请求时的total_fee、seller_id与通知时获取的total_fee、seller_id为一致的

				//调试用，写文本函数记录程序运行情况是否正常
				//logResult("这里写入想要调试的代码变量值，或其他运行的结果记录");
		
			}else if ($trade_status == 'TRADE_SUCCESS') {
				//判断该笔订单是否在商户网站中已经做过处理
				//如果没有做过处理，根据订单号（out_trade_no）在商户网站的订单系统中查到该笔订单的详细，并执行商户的业务程序
				//如果有做过处理，不执行商户的业务程序
					
				//注意：
				//付款完成后，支付宝系统发送该交易状态通知
				//请务必判断请求时的total_fee、seller_id与通知时获取的total_fee、seller_id为一致的

				//调试用，写文本函数记录程序运行情况是否正常
				//logResult("这里写入想要调试的代码变量值，或其他运行的结果记录");
                
                $where['orderno']=$out_trade_no;
                $where['money']=$total_fee;
                $where['type']=1;
                
                $data=[
                    'trade_no'=>$trade_no
                ];

				$this->logali("where:".json_encode($where));	
                $res=handelCharge($where,$data);
				if($res==0){
                    $this->logali("orderno:".$out_trade_no.' 订单信息不存在');	
                    echo "fail";
                    return;
				}
                
                $this->logali("成功");
                echo "success";		//请不要修改或删除
                return;
			}
			
		
			//更新会员余额
			//——请根据您的业务逻辑来编写程序（以上代码仅作参考）——
			echo "fail";		//请不要修改或删除

			/////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////	/* 打印log */
//		file_put_contents(CMF_ROOT.'log/think/home/payment/alipay_d_notify_'.date('y-m-d').'.txt',date('y-m-d h:i:s').'  msg:'.$msg."\r\n",FILE_APPEND);
		}	
		else {
			//验证失败
			echo "fail";

			//调试用，写文本函数记录程序运行情况是否正常
			//logResult("这里写入想要调试的代码变量值，或其他运行的结果记录");
		}	
	}	
	//支付宝即时到帐  返回处理	
	
	
	/* 打印log */
	public function logali($msg){
		file_put_contents(CMF_ROOT.'log/think/home/payment/alipagepay_'.date("Y-m-d").'.txt',date('Y-m-d H:i:s').' '.$msg."\r\n",FILE_APPEND);
	}	
	
	/**
	 * 获取订单支付状态
	 */
	public function getOrderStatus(){
		require_once CMF_ROOT."sdk/wxpay/lib/WxPay.Api.php";
		require_once CMF_ROOT."sdk/wxpay/lib/WxPay.Notify.php";
		require_once CMF_ROOT."sdk/wxpay/pay/notify.php";
	   
	 	$orderid = $_GET['orderid'];
	 	$notify = new \PayNotifyCallBack();
		$wxpayStatus=$notify->Queryorder($orderid);
		
		$order_info = explode("_",$orderid); 
		$uid = $order_info[0];
		$touid = $order_info[1];
		
		//获取该订单在数据库内的信息
        
        $where['orderno']=$orderid;
        
		$orderinfo=Db::name("charge_user")->where($where)->find();

		if($orderinfo['status']==1){
            echo 1;
            return;
		}
       
		//订单是否真正支付
		if($wxpayStatus['trade_state']=='SUCCESS'){
			
			if($wxpayStatus['out_trade_no']==$orderid && $orderinfo['status']==0){
				echo 1;
                return;
			}

		}elseif($wxpayStatus['trade_state']=='NOTPAY'){
			echo 0 ; //未支付

		}else{
			echo 0;//未知错误

		} 
	}

	public function wxpay(){
        
        $data = $this->request->param();
        $url= $data['url'] ?? '';
        $money= $data['money'] ?? '0';
        $coin= $data['coin'] ?? '0';
        $orderid= $data['orderid'] ?? '';

        
        $logo="微信扫码支付";
		$this->assign("logo",$logo);
		$this->assign("url",$url);
		
        $this->assign("money",$money);
        $this->assign("coin",$coin);
        $this->assign("orderid",$orderid);
    
        return $this->fetch();
	}
  
    
    public function mylist() {
		
    	return $this->fetch();
    }
    
    public function qrcode(){
        error_reporting(E_ERROR);
        require_once CMF_ROOT.'sdk/wxpay/pay/phpqrcode/phpqrcode.php';
        $url = urldecode($_GET["data"]);
        \QRcode::png($url);

    }

    public function coinchargelist(){
    	LogIn();
		$uid=(int)session("uid");
        if(!$uid){
            $this->error('请先登录');
        }

    	$lists = Db::name("charge_user")
    		->field("coin,money,addtime")
			->where(['uid'=>$uid,'status'=>1])
			->order("addtime desc")
			->paginate(12);

		$lists->each(function($v,$k){

			$v['coin']=$v['coin'];
			$v['addtime']=date('Y-m-d',$v['addtime']);

			return $v;

		});


        $page = $lists->render();
        $configpub=getConfigPub();
        $name_coin=$configpub['name_coin'];
                
    	$this->assign('lists', $lists);
    	$this->assign('page', $page);
    	$this->assign('name_coin', $name_coin);

    	return $this->fetch();
    }

}


