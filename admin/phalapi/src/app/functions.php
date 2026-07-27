<?php
	namespace App;
	

    
    /* 密码检查 */
	function passcheck($user_pass) {
        /* 必须包含字母、数字 */
        $preg='/^(?=.*[A-Za-z])(?=.*[0-9])[a-zA-Z0-9~!@&%#_.]{6,20}$/';
        $isok=preg_match($preg,$user_pass);
        if($isok){
            return 1;
        }
        return 0;
	}	
	/* 检验手机号 */
	function checkMobile($mobile){
		$ismobile = preg_match("/^1[3|4|5|6|7|8|9]\d{9}$/",$mobile);
		if($ismobile){
			return 1;
		}else{
			return 0;
		}
	}
	/* 随机数 */
	function random($length = 6 , $numeric = 0) {
		PHP_VERSION < '4.2.0' && mt_srand((double)microtime() * 1000000);
		if($numeric) {
			$hash = sprintf('%0'.$length.'d', mt_rand(0, pow(10, $length) - 1));
		} else {
			$hash = '';
			$chars = 'ABCDEFGHJKLMNPQRSTUVWXYZ23456789abcdefghjkmnpqrstuvwxyz';
			$max = strlen($chars) - 1;
			for($i = 0; $i < $length; $i++) {
				$hash .= $chars[mt_rand(0, $max)];
			}
		}
		return $hash;
	}
	/* 发送验证码--互译无线 */
	function sendCode_huiyi($mobile,$code){
		$rs=array();
		$config = getConfigPri();
        
        if(!$config['sendcode_switch']){
            $rs['code']=667;
			$rs['msg']='123456';
            return $rs;
        }
        
		/* 互亿无线 */
		$target = "http://106.ihuyi.cn/webservice/sms.php?method=Submit";
		$content="您的验证码是：".$code."。请不要把验证码泄露给其他人。";
		$post_data = "account=".$config['ihuyi_account']."&password=".$config['ihuyi_ps']."&mobile=".$mobile."&content=".rawurlencode($content);
		//密码可以使用明文密码或使用32位MD5加密
		$gets = xml_to_array(Post($post_data, $target));
//        file_put_contents(API_ROOT.'/../log/phalapi/function_sendcode_huiyi_'.date('Y-m-d').'.txt',date('Y-m-d H:i:s').' 提交参数信息 post_data:'.$post_data."\r\n",FILE_APPEND);
//        file_put_contents(API_ROOT.'/../log/phalapi/function_sendcode_huiyi_'.date('Y-m-d').'.txt',date('Y-m-d H:i:s').'返回结果 gets:'.json_encode($gets)."\r\n\r\n",FILE_APPEND);
		if($gets['SubmitResult']['code']==2){
            setSendcode(array('type'=>'1','account'=>$mobile,'content'=>$content));
			$rs['code']=0;
		}else{
			$rs['code']=1002;
			//$rs['msg']=$gets['SubmitResult']['msg'];
			$rs['msg']="获取失败";
		} 
		return $rs;
	}

	function Post($curlPost,$url){
		$curl = curl_init();
		curl_setopt($curl, CURLOPT_URL, $url);
		curl_setopt($curl, CURLOPT_HEADER, false);
		curl_setopt($curl, CURLOPT_RETURNTRANSFER, true);
		curl_setopt($curl, CURLOPT_NOBODY, true);
		curl_setopt($curl, CURLOPT_POST, true);
		curl_setopt($curl, CURLOPT_POSTFIELDS, $curlPost);
		$return_str = curl_exec($curl);
		curl_close($curl);
		return $return_str;
	}
	
	function xml_to_array($xml){
		$reg = "/<(\w+)[^>]*>([\\x00-\\xFF]*)<\\/\\1>/";
		if(preg_match_all($reg, $xml, $matches)){
			$count = count($matches[0]);
			for($i = 0; $i < $count; $i++){
			$subxml= $matches[2][$i];
			$key = $matches[1][$i];
				if(preg_match( $reg, $subxml )){
					$arr[$key] = xml_to_array( $subxml );
				}else{
					$arr[$key] = $subxml;
				}
			}
		}
		return $arr;
	}
	/* 发送验证码 */
    
    	/* 发送验证码 -- 容联云 */
	function sendCode_ronglianyun($mobile,$code){
        
		$rs = array('code' => 0, 'msg' => '', 'info' => array());
        
		$config = getConfigPri();
        
        if(!$config['sendcode_switch']){
            $rs['code']=667;
			$rs['msg']='123456';
            return $rs;
        }
        
        require_once API_ROOT.'/../sdk/ronglianyun/CCPRestSDK.php';
        
        //主帐号
        $accountSid= $config['ccp_sid'];
        //主帐号Token
        $accountToken= $config['ccp_token'];
        //应用Id
        $appId=$config['ccp_appid'];
        //请求地址，格式如下，不需要写https://
        $serverIP='app.cloopen.com';
        //请求端口 
        $serverPort='8883';
        //REST版本号
        $softVersion='2013-12-26';
        
        $tempId=$config['ccp_tempid'];
        
//        file_put_contents(API_ROOT.'/../log/phalapi/function_sendcode_ronglianyun_'.date('Y-m-d').'.txt',date('Y-m-d H:i:s').' 提交参数信息 post_data: accountSid:'.$accountSid.";accountToken:{$accountToken};appId:{$appId};tempId:{$tempId}\r\n",FILE_APPEND);

        $rest = new REST($serverIP,$serverPort,$softVersion);
        $rest->setAccount($accountSid,$accountToken);
        $rest->setAppId($appId);
        
        $datas=[];
        $datas[]=$code;
        
        $result = $rest->sendTemplateSMS($mobile,$datas,$tempId);
//        file_put_contents(API_ROOT.'/../log/phalapi/function_sendcode_rly_'.date('Y-m-d').'.txt',date('Y-m-d H:i:s').' 提交参数信息 result:'.json_encode($result)."\r\n\r\n",FILE_APPEND);
        
         if($result == NULL ) {
            $rs['code']=1002;
			$rs['msg']=\PhalApi\T("获取失败");
            return $rs;
         }
         if($result->statusCode!='000000') {
            //echo "error code :" . $result->statusCode . "<br>";
            //echo "error msg :" . $result->statusMsg . "<br>";
            //TODO 添加错误处理逻辑
            $rs['code']=1002;
			//$rs['msg']=$gets['SubmitResult']['msg'];
			$rs['msg']=\PhalApi\T("获取失败");
            return $rs;
         }
        $content=$code;
        setSendcode(array('type'=>'1','account'=>$mobile,'content'=>$content));

		return $rs;
	}

	/* 发送验证码*/
		function sendCode($country_code,$mobile,$code){
        
		$rs = array('code' => 0, 'msg' => '', 'info' => array());
        
		$config = getConfigPri();
        
        if(!$config['sendcode_switch']){
            $rs['code']=667;
			$rs['msg']='123456';
            return $rs;
        }

        $typecode_switch=$config['typecode_switch'];

		if($typecode_switch=='1'){//阿里云
			$res=sendCodeByAli($country_code,$mobile,$code);
		}else if($typecode_switch=='2'){ //容联云
			$res=sendCodeByRonglian($mobile,$code);
		}else if($typecode_switch=='3'){ //腾讯云
			$res=sendCodeByTencentSms($country_code,$mobile,$code);//腾讯云
		}

        $content=$code;
        setSendcode(array('type'=>'1','account'=>'+'.$country_code.'-'.$mobile,'content'=>$content,'send_type'=>$config['typecode_switch']));

			return $res;
		}

		function getCmfOption($name){
			$config= \PhalApi\DI()->notorm->option
					->select('option_value')
					->where('option_name=?',$name)
					->fetchOne();
			if(!$config || !$config['option_value']){
				return array();
			}
			$data=json_decode($config['option_value'],true);
			return is_array($data) ? $data : array();
		}

		function renderEmailCodeTemplate($template,$data){
			if(!$template){
				$template='您的验证码是：{$code}，5分钟内有效。';
			}
			$template=htmlspecialchars_decode($template,ENT_QUOTES);
			foreach($data as $key=>$value){
				$template=str_replace('{$'.$key.'}',$value,$template);
				$template=str_replace('{'.$key.'}',$value,$template);
			}
			return $template;
		}

		function sendEmailCode($email,$code,$scene='register'){
			$rs = array('code' => 0, 'msg' => '', 'info' => array());
			$smtp=getCmfOption('smtp_setting');
			if(!$smtp || empty($smtp['host']) || empty($smtp['from']) || empty($smtp['username']) || empty($smtp['password'])){
				$rs['code']=1002;
				$rs['msg']=\PhalApi\T('邮箱服务未配置');
				return $rs;
			}

			if(!class_exists('\\PHPMailer\\PHPMailer\\PHPMailer')){
				$autoload=API_ROOT.'/../vendor/autoload.php';
				if(file_exists($autoload)){
					require_once $autoload;
				}
			}
			if(!class_exists('\\PHPMailer\\PHPMailer\\PHPMailer')){
				$rs['code']=1002;
				$rs['msg']=\PhalApi\T('邮件组件不可用');
				return $rs;
			}

			$emailTemplate=getCmfOption('email_template_verification_code');
			$defaultSubject=$scene=='forget' ? \PhalApi\T('找回密码验证码') : \PhalApi\T('注册验证码');
			$subject=empty($emailTemplate['subject']) ? $defaultSubject : $emailTemplate['subject'];
			$message=renderEmailCodeTemplate($emailTemplate['template'] ?? '',array(
				'code'=>$code,
				'username'=>$email,
				'scene'=>$scene,
			));

			try{
				$mail = new \PHPMailer\PHPMailer\PHPMailer(true);
				$mail->isSMTP();
				$mail->isHTML(true);
				$mail->CharSet='UTF-8';
				$mail->addAddress($email);
				$mail->Body=$message;
				$mail->From=$smtp['from'];
				$mail->FromName=$smtp['from_name'] ?? '';
				$mail->Subject=$subject;
				$mail->Host=$smtp['host'];
				$mail->SMTPSecure=empty($smtp['smtp_secure']) ? '' : $smtp['smtp_secure'];
				$mail->Port=empty($smtp['port']) ? 25 : (int)$smtp['port'];
				$mail->SMTPAuth=true;
				$mail->SMTPAutoTLS=false;
				$mail->Timeout=10;
				$mail->Username=$smtp['username'];
				$mail->Password=$smtp['password'];
				$mail->send();
				setSendcode(array('type'=>'2','account'=>$email,'content'=>$code,'send_type'=>'email'));
			}catch(\Throwable $e){
				$rs['code']=1002;
				$rs['msg']=\PhalApi\T('邮箱验证码发送失败：').$e->getMessage();
			}

			return $rs;
		}

		function sendCodeByRonglian($mobile,$code){
		$rs = array('code' => 0, 'msg' => '', 'info' => array());
        
		$config = getConfigPri();
       
        require_once API_ROOT.'/../sdk/ronglianyun/CCPRestSDK.php';
        
        //主帐号
        $accountSid= $config['ccp_sid'];
        //主帐号Token
        $accountToken= $config['ccp_token'];
        //应用Id
        $appId=$config['ccp_appid'];
        //请求地址，格式如下，不需要写https://
        $serverIP='app.cloopen.com';
        //请求端口 
        $serverPort='8883';
        //REST版本号
        $softVersion='2013-12-26';
        
        $tempId=$config['ccp_tempid'];
        
        //file_put_contents(API_ROOT.'/../data/sendCode_rly_'.date('Y-m-d').'.txt',date('Y-m-d H:i:s').' 提交参数信息 post_data: accountSid:'.$accountSid.";accountToken:{$accountToken};appId:{$appId};tempId:{$tempId}\r\n",FILE_APPEND);

        $rest = new REST($serverIP,$serverPort,$softVersion);
        $rest->setAccount($accountSid,$accountToken);
        $rest->setAppId($appId);
        
        $datas=[];
        $datas[]=$code;
        
        $result = $rest->sendTemplateSMS($mobile,$datas,$tempId);
        //file_put_contents(API_ROOT.'/../data/sendCode_rly_'.date('Y-m-d').'.txt',date('Y-m-d H:i:s').' 提交参数信息 result:'.json_encode($result)."\r\n",FILE_APPEND);
        
         if($result == NULL ) {
            $rs['code']=1002;
			$rs['msg']=\PhalApi\T("获取失败");
            return $rs;
         }
         if($result->statusCode!='000000') {
            //echo "error code :" . $result->statusCode . "<br>";
            //echo "error msg :" . $result->statusMsg . "<br>";
            //TODO 添加错误处理逻辑
            $rs['code']=1002;
			//$rs['msg']=$gets['SubmitResult']['msg'];
			$rs['msg']=\PhalApi\T("获取失败");
            return $rs;
         }
        

		return $rs;
	}
	//阿里云短信
	function sendCodeByAli($country_code,$mobile,$code){
		$rs = array('code' => 0, 'msg' => '', 'info' => array());
        
        $config = getConfigPri();

        //判断是否是国外
        $aly_sendcode_type=$config['aly_sendcode_type'];
        if($aly_sendcode_type==1 && $country_code!=86){ //国内
        	$rs['code']=1002;
			$rs['msg']=\PhalApi\T("平台短信仅支持中国大陆地区");
            return $rs;
        }

        if($aly_sendcode_type==2 && $country_code==86){
        	$rs['code']=1002;
			$rs['msg']=\PhalApi\T('平台短信仅支持国际/港澳台地区');
			return $rs;
        }
		
		require_once API_ROOT.'/../sdk/aliyunsms/AliSmsApi.php';

		$config_dl  = array(
            'accessKeyId' => $config['aly_keyid'], 
            'accessKeySecret' => $config['aly_secret'], 
            'PhoneNumbers' => $mobile, 
            'SignName' => $config['aly_signName'], //国内短信签名 
            'TemplateCode' => $config['aly_templateCode'], //国内短信模板ID
            'TemplateParam' => array("code"=>$code) 
        );

        $config_hw  = array(
            'accessKeyId' => $config['aly_keyid'], 
            'accessKeySecret' => $config['aly_secret'], 
            'PhoneNumbers' => $country_code.$mobile, 
            'SignName' => $config['aly_hw_signName'], //港澳台/国外短信签名 
            'TemplateCode' => $config['aly_hw_templateCode'], //港澳台/国外短信模板ID
            'TemplateParam' => array("code"=>$code) 
        );
        
        if($aly_sendcode_type==1){ //国内
            $config=$config_dl;
        }else if($aly_sendcode_type==2){ //国际/港澳台地区
            $config=$config_hw;
        }else{

            if($country_code==86){
                $config=$config_dl;
            }else{
                $config=$config_hw;
            }
        }
		 
		$go = new \AliSmsApi($config);
		$result = $go->send_sms();
//        file_put_contents(API_ROOT.'/../log/phalapi/function_sendCodeByAli_'.date('Y-m-d').'.txt',date('Y-m-d H:i:s').' 提交参数信息 result:'.json_encode($result)."\r\n",FILE_APPEND);
		
        if($result == NULL ) {
            $rs['code']=1002;
			$rs['msg']=\PhalApi\T("发送失败");
            return $rs;
        }
		if($result['Code']!='OK') {
            //TODO 添加错误处理逻辑
            $rs['code']=1002;
			//$rs['msg']=$result['Code'];
			$rs['msg']=\PhalApi\T("获取失败");
            return $rs;
        }
		return $rs;
	}

	//腾讯云短信
	function sendCodeByTencentSms($nationCode,$mobile,$code){
		require_once API_ROOT."/../sdk/tencentSms/index.php";
		$rs=array();
		$configpri = getConfigPri();
        
        $appid=$configpri['tencent_sms_appid'];
        $appkey=$configpri['tencent_sms_appkey'];


		$smsSign_dl = $configpri['tencent_sms_signName'];
        $smsSign_hw = $configpri['tencent_sms_hw_signName'];
        $templateId_dl=$configpri['tencent_sms_templateCode'];
        $templateId_hw=$configpri['tencent_sms_hw_templateCode'];

		$tencent_sendcode_type=$configpri['tencent_sendcode_type'];

		if($tencent_sendcode_type==1){ //中国大陆
            $smsSign = $smsSign_dl;
            $templateId = $templateId_dl;

        }else if($tencent_sendcode_type==2){//港澳台/国际

            $smsSign=$smsSign_hw;
            $templateId = $templateId_hw;

        }else{ //全球

            if($nationCode==86){
                $smsSign = $smsSign_dl;
                $templateId = $templateId_dl;
            }else{
                $smsSign=$smsSign_hw;
                $templateId = $templateId_hw;
            }
        }

	
		$sender = new \Qcloud\Sms\SmsSingleSender($appid,$appkey);

		$params = [$code]; //参数列表与腾讯云后台创建模板时加的参数列表保持一致
		$result = $sender->sendWithParam($nationCode, $mobile, $templateId, $params, $smsSign, "", "");  // 签名参数未提供或者为空时，会使用默认签名发送短信
				
//		file_put_contents(API_ROOT.'/../log/phalapi/function_sendCodeByTencentSms_'.date('Y-m-d').'.txt',date('Y-m-d H:i:s').' 提交参数信息 result:'.json_encode($result)."\r\n",FILE_APPEND);
		$arr=json_decode($result,TRUE);

		if($arr['result']==0 && $arr['errmsg']=='OK'){
            //setSendcode(array('type'=>'1','account'=>$mobile,'content'=>"验证码:".$code."---国家区号:".$nationCode));
			$rs['code']=0;
		}else{
			$rs['code']=1002;
			$rs['msg']=$arr['errmsg'];
			// $rs['msg']='验证码发送失败';
		} 
		return $rs;		
				
	}
    
    /* curl get请求 */
    function curl_get($url){
		$curl = curl_init();
		curl_setopt($curl, CURLOPT_URL, $url);
		curl_setopt($curl, CURLOPT_HEADER, false);
		curl_setopt($curl, CURLOPT_RETURNTRANSFER, true);
		curl_setopt($curl, CURLOPT_NOBODY, true);
        curl_setopt($curl, CURLOPT_SSL_VERIFYPEER, false); // 跳过证书检查  
        curl_setopt($curl, CURLOPT_SSL_VERIFYHOST, 0);  // 从证书中检查SSL加密算法是否存在
		$return_str = curl_exec($curl);
		curl_close($curl);
		return $return_str;
	}
    
	/* 检测文件后缀 */
	function checkExt($filename){
		$config=array("jpg","png","jpeg");
		$ext   =   pathinfo(strip_tags($filename), PATHINFO_EXTENSION);
		 
		return empty($config) ? true : in_array(strtolower($ext), $config);
	}	    
	/* 密码加密 */
	function setPass($pass){
		$authcode=getenv('DATABASE_AUTHCODE') ?: 'change-me-in-environment';
		$pass="###".md5(md5($authcode.$pass));
		return $pass;
	}	
    /* 去除NULL 判断空处理 主要针对字符串类型*/
	function checkNull($checkstr){

		$checkstr=urldecode($checkstr);
        $checkstr=htmlspecialchars($checkstr);
        $checkstr=trim($checkstr);

		//$checkstr=filterEmoji($checkstr);
		if( strstr($checkstr,'null') || (!$checkstr && $checkstr!=0 ) ){
			$str='';
		}else{
			$str=$checkstr;
		}

		$str=htmlspecialchars($str);
		return $str;	
	}

	function starCoinName(){
		return '星币';
	}

	function starCoinNameEn(){
		return 'Star Coin';
	}

	function normalizePublicCurrencyConfig($config){
		$config=is_array($config) ? $config : array();
		$config['name_coin']=starCoinName();
		$config['name_votes']=starCoinName();
		$config['name_score']=starCoinName();
		$config['name_coin_en']=starCoinNameEn();
		$config['name_votes_en']=starCoinNameEn();
		$config['name_score_en']=starCoinNameEn();

		return $config;
	}

	function normalizePrivateCurrencyConfig($config){
		$config=is_array($config) ? $config : array();
		$config['cash_rate']='1';
		$config['cash_take']='0';
		$config['bepusdt_fiat']='USD';

		return $config;
	}

	function starCoinAmountFromCharge($charge){
		$money=(float)($charge['money'] ?? 0);
		if($money<=0){
			$money=(float)($charge['coin'] ?? 0);
		}

		$coin=(int)round($money);
		return $coin > 0 ? $coin : 0;
	}

	function normalizeChargeRule($charge){
		if(!$charge || !is_array($charge)){
			return $charge;
		}

		$coin=starCoinAmountFromCharge($charge);
		if($coin<=0){
			return $charge;
		}

		$charge['coin']=$coin;
		$charge['coin_ios']=$coin;
		$charge['coin_paypal']=$coin;
		$charge['give']=0;
		$charge['money']=number_format($coin,2,'.','');

		return $charge;
	}

	function normalizeChargeRules($rules){
		if(!is_array($rules)){
			return $rules;
		}

		foreach($rules as $k=>$rule){
			$rules[$k]=normalizeChargeRule($rule);
		}

		return $rules;
	}
	/* 去除emoji表情 */
	function filterEmoji($str){
		$str = preg_replace_callback(
			'/./u',
			function (array $match) {
				return strlen($match[0]) >= 4 ? '' : $match[0];
			},
			$str);
		return $str;
	}	
	/* 公共配置 */
	function getConfigPub() {
		static $runtimeConfig=array();
		$language=isset(\PhalApi\DI()->language) ? \PhalApi\DI()->language : 'zh-cn';
		if(PHP_SAPI!=='cli' && isset($runtimeConfig[$language])){
			return $runtimeConfig[$language];
		}

		$key='getConfigPub';
		$config=getcaches($key);
		if(!$config){
			$configRow= \PhalApi\DI()->notorm->option
					->select('option_value')
					->where("option_name='site_info'")
					->fetchOne();

            $config=$configRow ? json_decode($configRow['option_value'],true) : array();
            
            if($config){
                setcaches($key,$config);
            }
			
		}
        if(!is_array($config)){
            $config=array();
        }
        if(isset($config['live_time_coin'])){
            if(is_array($config['live_time_coin'])){
                
            }else if($config['live_time_coin']){
                $config['live_time_coin']=preg_split('/,|，/',$config['live_time_coin']);
            }else{
                $config['live_time_coin']=array();
            }
        }else{
            $config['live_time_coin']=array();
        }
        
        if(isset($config['login_type'])){
            if(is_array($config['login_type'])){
                
            }else if($config['login_type']){
                $config['login_type']=preg_split('/,|，/',$config['login_type']);
            }else{
                $config['login_type']=array();
            }
        }else{
            $config['login_type']=array();
        }
        
        if(isset($config['share_type'])){
            if(is_array($config['share_type'])){
                
            }else if($config['share_type']){
                $config['share_type']=preg_split('/,|，/',$config['share_type']);
            }else{
                $config['share_type']=array();
            }
        }else{
            $config['share_type']=array();
        }
        
        if(isset($config['live_type'])){
            if(is_array($config['live_type'])){
                
            }else if($config['live_type']){
                $live_type=preg_split('/,|，/',$config['live_type']);

                foreach($live_type as $k=>$v){

                	//var_dump($v);

                    $live_type[$k]=preg_split('/;|；/',$v);
                }

                /*var_dump($live_type);
                die;*/
                $config['live_type']=$live_type;
            }else{
                $config['live_type']=array();
            }
        }else{
            $config['live_type']=array();
        }

        //语言包
        if($language=='en'){
        	$config['maintain_tips']=$config['maintain_tips_en'] ?? '';
        	$config['name_coin']=$config['name_coin_en'] ?? '';
        	$config['name_score']=$config['name_score_en'] ?? '';
        	$config['name_votes']=$config['name_votes_en'] ?? '';
        	$config['apk_des']=$config['apk_des_en'] ?? '';
        	$config['ipa_des']=$config['ipa_des_en'] ?? '';
        	$config['share_title']=$config['share_title_en'] ?? '';
        	$config['share_des']=$config['share_des_en'] ?? '';
        	$config['video_share_title']=$config['video_share_title_en'] ?? '';
        	$config['video_share_des']=$config['video_share_des_en'] ?? '';
        	$config['teenager_des']=$config['teenager_des_en'] ?? '';

        	foreach ($config['live_type'] as $k => $v) {

        		$v['1']=\PhalApi\T($v['1']);

        		$config['live_type'][$k]=$v;
        	}
        }

        //die;

        unset($config['maintain_tips_en']);
        unset($config['name_coin_en']);
        unset($config['name_score_en']);
        unset($config['name_votes_en']);
        unset($config['apk_des_en']);
        unset($config['ipa_des_en']);
        unset($config['share_title_en']);
        unset($config['share_des_en']);
        unset($config['video_share_title_en']);
        unset($config['video_share_des_en']);
        unset($config['teenager_des_en']);
        
		$config=normalizePublicCurrencyConfig($config);
		if(PHP_SAPI!=='cli'){
			$runtimeConfig[$language]=$config;
		}

		return 	$config;
	}		
	
	/* 私密配置 */
	function getConfigPri() {
		static $runtimeConfig=null;
		if(PHP_SAPI!=='cli' && $runtimeConfig!==null){
			return $runtimeConfig;
		}

		$key='getConfigPri';
		$config=getcaches($key);
		if(!$config){
			$configRow= \PhalApi\DI()->notorm->option
					->select('option_value')
					->where("option_name='configpri'")
					->fetchOne();
            $config=$configRow ? json_decode($configRow['option_value'],true) : array();
            if($config){
                setcaches($key,$config);
            }
			
		}

        if(!is_array($config)){
            $config=array();
        }

        if(isset($config['game_switch'])){
            if(is_array($config['game_switch'])){
                
            }else if($config['game_switch']){
                $config['game_switch']=preg_split('/,|，/',$config['game_switch']);
            }else{
                $config['game_switch']=array();
            }
        }else{
            $config['game_switch']=array();
        }

        //语言包
        $language=\PhalApi\DI()->language;

        $config['usdt_switch']=$config['usdt_switch'] ?? (envValue('BEPUSDT_API_TOKEN') ? '1' : '0');
        $config['bepusdt_api_url']=$config['bepusdt_api_url'] ?? envValue('BEPUSDT_API_URL','');
        $config['bepusdt_api_token']=$config['bepusdt_api_token'] ?? envValue('BEPUSDT_API_TOKEN','');
        $config['bepusdt_trade_type']=$config['bepusdt_trade_type'] ?? envValue('BEPUSDT_TRADE_TYPE','usdt.trc20');
        $config['bepusdt_fiat']=$config['bepusdt_fiat'] ?? envValue('BEPUSDT_FIAT','USD');
        $config['bepusdt_timeout']=$config['bepusdt_timeout'] ?? envValue('BEPUSDT_TIMEOUT','1200');
		$config=normalizePrivateCurrencyConfig($config);
		if(PHP_SAPI!=='cli'){
			$runtimeConfig=$config;
		}

		return 	$config;
	}		

	function envValue($key,$default=''){
	    $value=getenv($key);
	    if($value===false && isset($_ENV[$key])){
	        $value=$_ENV[$key];
	    }

	    return ($value===false || $value===null) ? $default : $value;
	}

	function getBepusdtConfig($configpri=null){
	    if($configpri===null){
	        $configpri=getConfigPri();
	    }

	    $apiUrl=trim($configpri['bepusdt_api_url'] ?? '') ?: trim(envValue('BEPUSDT_API_URL',''));
	    if($apiUrl!=='' && file_exists('/.dockerenv')){
	        $apiUrl=preg_replace('#^(https?://)(127\.0\.0\.1|localhost)(:\d+)?#i','$1host.docker.internal$3',$apiUrl);
	    }

	    $token=trim($configpri['bepusdt_api_token'] ?? '') ?: trim(envValue('BEPUSDT_API_TOKEN',''));
	    $enabled=(string)($configpri['usdt_switch'] ?? ($token!=='' ? '1' : '0')) === '1';
	    $timeout=(int)(trim($configpri['bepusdt_timeout'] ?? '') ?: envValue('BEPUSDT_TIMEOUT','1200'));
	    if($timeout<120){
	        $timeout=1200;
	    }

	    return array(
	        'enabled'=>$enabled,
	        'api_url'=>rtrim($apiUrl,'/'),
	        'api_token'=>$token,
	        'trade_type'=>trim($configpri['bepusdt_trade_type'] ?? '') ?: envValue('BEPUSDT_TRADE_TYPE','usdt.trc20'),
	        'fiat'=>strtoupper(trim($configpri['bepusdt_fiat'] ?? '') ?: envValue('BEPUSDT_FIAT','USD')),
	        'timeout'=>$timeout,
	    );
	}

	function bepusdtSign($params,$token){
	    unset($params['signature']);
	    ksort($params,SORT_STRING);

	    $pairs=array();
	    foreach($params as $key=>$value){
	        if($value===null || $value===''){
	            continue;
	        }
	        if(is_bool($value)){
	            $value=$value ? 'true' : 'false';
	        }
	        $pairs[]=$key.'='.$value;
	    }

	    return md5(implode('&',$pairs).$token);
	}

	function bepusdtRequest($path,$params,$config=null){
	    if($config===null){
	        $config=getBepusdtConfig();
	    }

	    if(empty($config['enabled'])){
	        return array('status_code'=>0,'message'=>'USDT支付未开启');
	    }

	    if(empty($config['api_url']) || empty($config['api_token'])){
	        return array('status_code'=>0,'message'=>'BEpusdt网关未配置');
	    }

	    $params['signature']=bepusdtSign($params,$config['api_token']);
	    $ch=curl_init($config['api_url'].$path);
	    curl_setopt($ch,CURLOPT_RETURNTRANSFER,true);
	    curl_setopt($ch,CURLOPT_POST,true);
	    curl_setopt($ch,CURLOPT_POSTFIELDS,json_encode($params,JSON_UNESCAPED_UNICODE | JSON_UNESCAPED_SLASHES));
	    curl_setopt($ch,CURLOPT_HTTPHEADER,array('Content-Type: application/json'));
	    curl_setopt($ch,CURLOPT_CONNECTTIMEOUT,10);
	    curl_setopt($ch,CURLOPT_TIMEOUT,15);
	    $body=curl_exec($ch);
	    $errno=curl_errno($ch);
	    $error=curl_error($ch);
	    curl_close($ch);

	    if($errno){
	        return array('status_code'=>0,'message'=>'BEpusdt请求失败：'.$error);
	    }

	    $result=json_decode($body,true);
	    if(!is_array($result)){
	        return array('status_code'=>0,'message'=>'BEpusdt响应解析失败','raw'=>$body);
	    }

	    return $result;
	}

	function bepusdtCreateTransaction($orderid,$money,$name,$notifyUrl,$redirectUrl=''){
	    $config=getBepusdtConfig();
	    $amount=(float)$money;
	    if($amount<=0){
	        return array('status_code'=>0,'message'=>'充值金额错误');
	    }

	    $params=array(
	        'order_id'=>(string)$orderid,
	        'amount'=>$amount,
	        'fiat'=>$config['fiat'],
	        'trade_type'=>$config['trade_type'],
	        'name'=>(string)$name,
	        'notify_url'=>(string)$notifyUrl,
	        'redirect_url'=>(string)$redirectUrl,
	        'timeout'=>$config['timeout'],
	    );

	    return bepusdtRequest('/api/v1/order/create-transaction',$params,$config);
	}

	/**
	 * 返回带协议的域名
	 */
	function get_host(){
		$config=getConfigPub();
		$site=trim((string)($config['site'] ?? ''));
		if($site !== ''){
			return rtrim($site,'/');
		}

		$forwardedProto=trim(explode(',',$_SERVER['HTTP_X_FORWARDED_PROTO'] ?? '')[0]);
		$scheme=$forwardedProto ?: ((!empty($_SERVER['HTTPS']) && $_SERVER['HTTPS'] !== 'off') ? 'https' : 'http');
		$forwardedHost=trim(explode(',',$_SERVER['HTTP_X_FORWARDED_HOST'] ?? '')[0]);
		$host=$forwardedHost ?: ($_SERVER['HTTP_HOST'] ?? '127.0.0.1:18080');
		return $scheme.'://'.$host;
	}	

	function getStorageTypeByCloudtype($cloudtype = null){
		if($cloudtype === null || $cloudtype === ''){
			$configpri=getConfigPri();
			$cloudtype=$configpri['cloudtype'] ?? '3';
		}

		$cloudtype=strtolower((string)$cloudtype);
		switch ($cloudtype) {
			case '1':
			case 'qiniu':
				return 'qiniu';
			case '2':
			case 'aws':
				return 'aws';
			case '4':
			case 'minio':
				return 'minio';
			case '3':
			case 'local':
			default:
				return 'local';
		}
	}

	function getStorageType(){
		$configpri=getConfigPri();
		return getStorageTypeByCloudtype($configpri['cloudtype'] ?? '3');
	}

	function buildStorageUrl($baseUrl,$file){
		$file=str_replace('\\','/',ltrim((string)$file,'/'));
		$parts=array_map('rawurlencode',explode('/',$file));
		return rtrim($baseUrl,'/').'/'.implode('/',$parts);
	}

	function getLocalUploadUrl($file){
		return buildStorageUrl(rtrim(get_host(),'/').'/upload',$file);
	}

	function getMinioBaseUrl($configpri = null){
		if($configpri === null){
			$configpri=getConfigPri();
		}

		$publicUrl=trim($configpri['minio_public_url'] ?? '');
		if($publicUrl !== ''){
			return rtrim($publicUrl,'/');
		}

		$endpoint=trim($configpri['minio_endpoint'] ?? '');
		$bucket=trim($configpri['minio_bucket'] ?? '');
		if($endpoint === ''){
			return '';
		}

		return rtrim($endpoint,'/').($bucket !== '' ? '/'.$bucket : '');
	}

	function getMinioUploadUrl($file,$configpri = null){
		$baseUrl=getMinioBaseUrl($configpri);
		if($baseUrl === ''){
			return '';
		}

		return buildStorageUrl($baseUrl,$file);
	}

	function getUploadApiUrl(){
		return rtrim(get_host(),'/').'/appapi/?service=Upload.uploadFile';
	}

	function buildUploadFileKey($filename,$dir = 'appapi'){
		$pathinfo=pathinfo((string)$filename);
		$suffix=strtolower($pathinfo['extension'] ?? 'dat');
		$suffix=preg_replace('/[^a-z0-9]/','',$suffix);
		if($suffix === ''){
			$suffix='dat';
		}

		return trim($dir,'/').'/'.date('Ymd').'/'.time().mt_rand(10000,99999).'.'.$suffix;
	}

	function moveUploadFileToLocal($tmpName,$fileKey){
		$target=API_ROOT.'/../public/upload/'.ltrim($fileKey,'/');
		$dir=dirname($target);
		if(!is_dir($dir)){
			mkdir($dir,0777,true);
		}

		if(is_uploaded_file($tmpName)){
			return move_uploaded_file($tmpName,$target);
		}

		return rename($tmpName,$target);
	}

	function uploadFileToMinio($tmpName,$fileKey,$contentType = ''){
		$configpri=getConfigPri();
		$endpoint=trim($configpri['minio_endpoint'] ?? '');
		$bucket=trim($configpri['minio_bucket'] ?? '');
		$accessKey=trim($configpri['minio_access_key'] ?? '');
		$secretKey=trim($configpri['minio_secret_key'] ?? '');
		$region=trim($configpri['minio_region'] ?? 'us-east-1');

		if($endpoint === '' || $bucket === '' || $accessKey === '' || $secretKey === ''){
			return false;
		}

		$path=API_ROOT.'/../sdk/aws/aws-autoloader.php';
		if(!file_exists($path)){
			return false;
		}
		require_once($path);

		try{
			$sdk = new \Aws\Sdk([
				'region' => $region ?: 'us-east-1',
				'version' => 'latest',
				'endpoint' => rtrim($endpoint,'/'),
				'use_path_style_endpoint' => true,
				'credentials' => [
					'key' => $accessKey,
					'secret' => $secretKey,
				],
			]);
			$s3Client = $sdk->createS3();

			$params=[
				'Bucket' => $bucket,
				'Key' => ltrim($fileKey,'/'),
				'ACL' => 'public-read',
				'Body' => fopen($tmpName,'r'),
			];
			if($contentType !== ''){
				$params['ContentType']=$contentType;
			}

			$s3Client->putObject($params);
			return true;
		}catch(\Throwable $e){
			return false;
		}
	}
	
	/**
	 * 转化数据库保存的文件路径，为可以访问的url
	 */
	function get_upload_path($file){
        if($file==''){
            return $file;
        }
		if(strpos($file,"http")===0){
			return html_entity_decode(htmlspecialchars_decode($file));
		}else if(strpos($file,"/")===0){
			$filepath= get_host().$file;
			return html_entity_decode(htmlspecialchars_decode($filepath));
		}else{

			$fileinfo=explode("_",$file);

			$storage_type=$fileinfo[0];
			$start=strlen($storage_type)+1;

			if($storage_type=='local'){
				$file=substr($file,$start);
				$filepath=getLocalUploadUrl($file);
	            return html_entity_decode(htmlspecialchars_decode($filepath));

			}else if($storage_type=='minio'){
				$configpri=getConfigPri();
				$file=substr($file,$start);
				$filepath=getMinioUploadUrl($file,$configpri);
	            return html_entity_decode(htmlspecialchars_decode($filepath));

			}else if($storage_type=='qiniu'){ //历史七牛数据兼容

				$space_host= \PhalApi\DI()->config->get('app.Qiniu.space_host');
				$file=substr($file,$start);
				$filepath=$space_host."/".$file;
	            return html_entity_decode(htmlspecialchars_decode($filepath));

			}else if($storage_type=='aws'){ //历史AWS数据兼容
				$configpri=getConfigPri();
				$space_host= $configpri['aws_hosturl'];
				$file=substr($file,$start);
				return html_entity_decode(htmlspecialchars_decode($space_host."/".$file));
			}else{

	            $filepath=getLocalUploadUrl($file);

	            return html_entity_decode(htmlspecialchars_decode($filepath));

			}

            
			
			
		}
	}
	
	/* 判断是否关注 */
	function isAttention($uid,$touid) {

		if($uid<1 || $touid<1){
			return '0';
		}
		$isexist=\PhalApi\DI()->notorm->user_attention
					->select("*")
					->where('uid=? and touid=?',$uid,$touid)
					->fetchOne();
		if($isexist){
			return  '1';
		}
        return  '0';
	}
	/* 是否黑名单 */
	function isBlack($uid,$touid) {

		if($uid<1 || $touid<1){
			return '0';
		}	
		$isexist=\PhalApi\DI()->notorm->user_black
				->select("*")
				->where('uid=? and touid=?',$uid,$touid)
				->fetchOne();
		if($isexist){
			return '1';
		}
        return '0';
	}	
	
	/* 判断权限 */
	function isAdmin($uid,$liveuid) {

		if($uid<1){
			return 30;
		}
		if($uid==$liveuid){
			return 50;
		}
		$isuper=isSuper($uid);
		if($isuper){
			return 60;
		}
		$isexist=\PhalApi\DI()->notorm->live_manager
					->select("*")
					->where('uid=? and liveuid=?',$uid,$liveuid)
					->fetchOne();
		if($isexist){
			return  40;
		}
		
		return  30;
			
	}	
	/* 判断账号是否超管 */
	function isSuper($uid){
		$isexist=\PhalApi\DI()->notorm->user_super
					->select("*")
					->where('uid=?',$uid)
					->fetchOne();
		if($isexist){
			return 1;
		}			
		return 0;
	}
	/* 判断token */
	function checkToken($uid,$token) {

		//return 0;
		$userinfo=getcaches("token_".$uid);

		if(!$userinfo){
			$userinfo=\PhalApi\DI()->notorm->user_token
						->select('token,expire_time')
						->where(['user_id'=>$uid])
						->fetchOne();
            if($userinfo){
                setcaches("token_".$uid,$userinfo);
            }
		}

		if((!$userinfo) || ($userinfo['token']!=$token) || ($userinfo['expire_time']<time())){
            return 700;
		}
        
        /* 是否禁用、拉黑 */
        $info=\PhalApi\DI()->notorm->user
					->select('user_status,end_bantime')
					->where('id=? and user_type="2"',$uid)
					->fetchOne();

        if(!$info || $info['user_status']==0  || $info['end_bantime']>time()){
            return 700;	
        }
        
        return 	0;				
				
	}	
	
	/* 用户基本信息 */
	function getUserInfo($uid,$type=0) {
		static $runtimeUserInfo=array();

		if(!is_numeric($uid)){
			$info['id']=(string)$uid;
			$info['user_nickname']=\PhalApi\T('用户不存在');
			$info['avatar']='/default.jpg';
			$info['avatar_thumb']='/default_thumb.jpg';
			$info['coin']="0";
			$info['sex']="1";
			$info['signature']='';
			$info['province']='';
			$info['city']=\PhalApi\T('城市未填写');
			$info['birthday']='';
			$info['issuper']="0";
			$info['votestotal']="0";
			$info['consumption']="0";
			$info['location']='';
			$info['user_status']='1';
			$info['praise_num']='0';
			$info['bg_img']=$info['avatar'];
			$info['age']='0';

		}else{
			$runtimeCacheKey=(string)$uid.':'.(string)$type;
			if(PHP_SAPI!=='cli' && array_key_exists($runtimeCacheKey,$runtimeUserInfo)){
				return $runtimeUserInfo[$runtimeCacheKey];
			}

			$info=$type==0 ? getcaches("userinfo_".$uid) : false;
			
			
			if(!$info){
				$info=\PhalApi\DI()->notorm->user
						->select('id,user_nickname,avatar,avatar_thumb,sex,signature,consumption,votestotal,province,city,birthday,user_status,issuper,location,praise_num,bg_img')
						->where('id=? and user_type="2"',$uid)
						->fetchOne();
				$isDbUserInfo=$info ? true : false;


				if($info){
					
				}else if($type==1){
					if(PHP_SAPI!=='cli'){
						$runtimeUserInfo[$runtimeCacheKey]=$info;
					}
	                return 	$info;
	                
	            }else{
	                $info['id']=(string)$uid;
	                $info['user_nickname']=\PhalApi\T('用户不存在');
	                $info['avatar']='/default.jpg';
	                $info['avatar_thumb']='/default_thumb.jpg';
	                $info['sex']='0';
	                $info['signature']='';
	                $info['consumption']='0';
	                $info['votestotal']='0';
	                $info['province']='';
	                $info['city']='';
	                $info['birthday']='';
	                $info['issuper']='0';
	                $info['user_status']='1';
	                $info['location']='';
	                $info['praise_num']='0';
	                $info['bg_img']=$info['avatar'];
	                $info['age']='0';

	                if($uid==1){
	                	$info['user_nickname']=\PhalApi\T('系统账号');
	                }
	            }

		            if($isDbUserInfo){
	                setcaches("userinfo_".$uid,$info);
	            }
				
			}
	        if($info){
	        	if(!isset($info['bg_img'])){
	        		$info['bg_img']=isset($info['avatar']) ? $info['avatar'] : '/default.jpg';
	        	}
	        	if(!isset($info['praise_num'])){
	        		$info['praise_num']='0';
	        	}
	        	$info['id']=(string)$info['id'];
	        	$info['sex']=(string)$info['sex'];
	            $info['level']=getLevel($info['consumption']);
	            $info['level_anchor']=getLevelAnchor($info['votestotal']);
	            $info['avatar']=get_upload_path($info['avatar']);
	            $info['avatar_thumb']=get_upload_path($info['avatar_thumb']);
	            $info['bg_img']=get_upload_path($info['bg_img']);   
	            $info['vip']=getUserVip($uid);
	            $info['liang']=getUserLiang($uid);
	            $info['consumption']=(string)$info['consumption'];
	            $info['votestotal']=(string)$info['votestotal'];
	            $info['user_status']=(string)$info['user_status'];
	            $info['issuper']=(string)$info['issuper'];
	            $info['praise_num']=(string)$info['praise_num'];

	            if($info['birthday']){
	                
	                $now=time();
	                $nowYear=date("Y",$now);
	                $month=date("m",$info['birthday']);
	                $nowMonth=date("m",$now);

	                if($nowMonth>=$month){
						$cha=0;
					}else{
						$cha=1;
					}

					$birthdayYear=date("Y",$info['birthday']);

					$age=$nowYear-$birthdayYear-$cha;
					$info['age']=(string)$age;

					$info['birthday']=date('Y-m-d',$info['birthday']);

	            }else{
	                $info['birthday']='';
	                $info['age']='0';
	            }
	            
	        }


		}

		
		if(isset($runtimeCacheKey) && PHP_SAPI!=='cli'){
			$runtimeUserInfo[$runtimeCacheKey]=$info;
		}
		return 	$info;		
	}
	
	/* 会员等级 */
	function getLevelList(){
		static $runtimeLevel=null;
		if(PHP_SAPI!=='cli' && $runtimeLevel!==null){
			return $runtimeLevel;
		}

        $key='level';
		$level=getcaches($key);
		if(!$level){
			$level=\PhalApi\DI()->notorm->level
					->select("*")
					->order("level_up asc")
					->fetchAll();
            if($level){
                setcaches($key,$level);	
            }
					 
		}
        $level=is_array($level) ? $level : array();
        
        foreach($level as $k=>$v){
            $v['thumb']=get_upload_path($v['thumb']);
            $v['thumb_mark']=get_upload_path($v['thumb_mark']);
            $v['bg']=get_upload_path($v['bg']);
            if($v['colour']){
                $v['colour']='#'.$v['colour'];
            }else{
                $v['colour']='#ffdd00';
            }
            $level[$k]=$v;
        }

		if(PHP_SAPI!=='cli'){
			$runtimeLevel=$level;
		}
        
        return $level;
    }
	function getLevel($experience){
		$levelid=1;
        $level_a=1;
		$level=getLevelList();

		foreach($level as $k=>$v){
			if( $v['level_up']>=$experience){
				$levelid=$v['levelid'];
				break;
			}else{
				$level_a = $v['levelid'];
			}
		}
		$levelid = $levelid < $level_a ? $level_a:$levelid;
		return (string)$levelid;
	}
	/* 主播等级 */
	function getLevelAnchorList(){
		static $runtimeLevel=null;
		if(PHP_SAPI!=='cli' && $runtimeLevel!==null){
			return $runtimeLevel;
		}

		$key='levelanchor';
		$level=getcaches($key);
		if(!$level){
			$level=\PhalApi\DI()->notorm->level_anchor
					->select("*")
					->order("level_up asc")
					->fetchAll();
            if($level){
                setcaches($key,$level);
            }
            
		}
        $level=is_array($level) ? $level : array();
        
        foreach($level as $k=>$v){
            $v['thumb']=get_upload_path($v['thumb']);
            $v['thumb_mark']=get_upload_path($v['thumb_mark']);
            $v['bg']=get_upload_path($v['bg']);
            $level[$k]=$v;
        }

		if(PHP_SAPI!=='cli'){
			$runtimeLevel=$level;
		}
        
        return $level;
    }
	function getLevelAnchor($experience){
		$levelid=1;
		$level_a=1;
        $level=getLevelAnchorList();

		foreach($level as $k=>$v){
			if( $v['level_up']>=$experience){
				$levelid=$v['levelid'];
				break;
			}else{
				$level_a = $v['levelid'];
			}
		}
		$levelid = $levelid < $level_a ? $level_a:$levelid;
		return (string)$levelid;
	}

	/* 统计 直播 */
	function getLives($uid) {
		/* 直播中 */
		$count1=\PhalApi\DI()->notorm->live
				->where('uid=? and islive="1"',$uid)
				->count();
		/* 回放 */
		$count2=\PhalApi\DI()->notorm->live_record
					->where('uid=? ',$uid)
					->count();
		return 	$count1+$count2;
	}		
	
	/* 统计 关注 */
	function getFollows($uid) {
		$count=\PhalApi\DI()->notorm->user_attention
				->where('uid=? ',$uid)
				->count();
		return 	$count;
	}			
	
	/* 统计 粉丝 */
	function getFans($uid) {
		$count=\PhalApi\DI()->notorm->user_attention
				->where('touid=? ',$uid)
				->count();
		return 	$count;
	}		
	/**
	*  @desc 根据两点间的经纬度计算距离
	*  @param float $lat 纬度值
	*  @param float $lng 经度值
	*/
	function getDistance($lat1, $lng1, $lat2, $lng2){
		$earthRadius = 6371000; //近似地球半径 单位 米
		 /*
		   Convert these degrees to radians
		   to work with the formula
		 */

		$lat1 = ($lat1 * pi() ) / 180;
		$lng1 = ($lng1 * pi() ) / 180;

		$lat2 = ($lat2 * pi() ) / 180;
		$lng2 = ($lng2 * pi() ) / 180;


		$calcLongitude = $lng2 - $lng1;
		$calcLatitude = $lat2 - $lat1;
		$stepOne = pow(sin($calcLatitude / 2), 2) + cos($lat1) * cos($lat2) * pow(sin($calcLongitude / 2), 2);  $stepTwo = 2 * asin(min(1, sqrt($stepOne)));
		$calculatedDistance = $earthRadius * $stepTwo;
		
		$distance=$calculatedDistance/1000;
		if($distance<10){
			$rs=round($distance,2);
		}else if($distance > 1000){
			$rs='1000';
		}else{
			$rs=round($distance);
		}
		return $rs.'km';
	}
	/* 判断账号是否禁用 */
	function isBanBF($uid){
		$status=\PhalApi\DI()->notorm->user
					->select("user_status")
					->where('id=?',$uid)
					->fetchOne();
		if(!$status || $status['user_status']==0){
			return '0';
		}
		return '1';
	}
	/* 是否认证 */
	function isAuth($uid){
		$status=\PhalApi\DI()->notorm->user_auth
					->select("status")
					->where('uid=?',$uid)
					->fetchOne();
		if($status && $status['status']==1){
			return '1';
		}

		return '0';
	}
	/* 过滤字符 */
	function filterField($field){
		$configpri=getConfigPri();
		
		$sensitive_field=$configpri['sensitive_field'];
		
		$sensitive=explode(",",$sensitive_field);
		$replace=array();
		$preg=array();
		foreach($sensitive as $k=>$v){
			if($v!=''){
				$re='';
				$num=mb_strlen($v);
				for($i=0;$i<$num;$i++){
					$re.='*';
				}
				$replace[$k]=$re;
				$preg[$k]='/'.$v.'/';
			}else{
				unset($sensitive[$k]);
			}
		}
		
		return preg_replace($preg,$replace,$field);
	}
	/* 时间差计算 */
	function datetime($time){
		$cha=time()-$time;
		$iz=floor($cha/60);
		$hz=floor($iz/60);
		$dz=floor($hz/24);
		/* 秒 */
		$s=$cha%60;
		/* 分 */
		$i=floor($iz%60);
		/* 时 */
		$h=floor($hz/24);
		/* 天 */
		
		if($cha<60){
			return \PhalApi\T('{num}秒前',['num'=>$cha]);
		}else if($iz<60){
			return \PhalApi\T('{num}分钟前',['num'=>$iz]);
		}else if($hz<24){
			return \PhalApi\T('{num}小时',['num'=>$hz]).\PhalApi\T('{num}分钟前',['num'=>$i]);
		}else if($dz<30){
			return \PhalApi\T('{num}天前',['num'=>$dz]);
		}else{
			return date("Y-m-d",$time);
		}
	}		


	/* 时长格式化 */
	function getSeconds($time,$type=0){

			if(!$time){
				return (string)$time;
			}

		    $value = array(
		      "years"   => 0,
		      "days"    => 0,
		      "hours"   => 0,
		      "minutes" => 0,
		      "seconds" => 0
		    );
		    
		    if($time >= 31556926){
		      $value["years"] = floor($time/31556926);
		      $time = ($time%31556926);
		    }
		    if($time >= 86400){
		      $value["days"] = floor($time/86400);
		      $time = ($time%86400);
		    }
		    if($time >= 3600){
		      $value["hours"] = floor($time/3600);
		      $time = ($time%3600);
		    }
		    if($time >= 60){
		      $value["minutes"] = floor($time/60);
		      $time = ($time%60);
		    }
		    $value["seconds"] = floor($time);

		    if($value['years']){
		    	if($type==1&&$value['years']<10){
		    		$value['years']='0'.$value['years'];
		    	}
		    }

		    if($value['days']){
		    	if($type==1&&$value['days']<10){
		    		$value['days']='0'.$value['days'];
		    	}
		    }

		    if($value['hours']){
		    	if($type==1&&$value['hours']<10){
		    		$value['hours']='0'.$value['hours'];
		    	}
		    }

		    if($value['minutes']){
		    	if($type==1&&$value['minutes']<10){
		    		$value['minutes']='0'.$value['minutes'];
		    	}
		    }

		    if($value['seconds']){
		    	if($type==1&&$value['seconds']<10){
		    		$value['seconds']='0'.$value['seconds'];
		    	}
		    }

		    if($value['years']){
		    	$t=$value["years"] .\PhalApi\T("年").$value["days"] .\PhalApi\T("天"). $value["hours"] .\PhalApi\T("小时"). $value["minutes"] .\PhalApi\T("分").$value["seconds"].\PhalApi\T("秒");
		    }else if($value['days']){
		    	$t=$value["days"] .\PhalApi\T("天"). $value["hours"] .\PhalApi\T("小时"). $value["minutes"] .\PhalApi\T("分").$value["seconds"].\PhalApi\T("秒");
		    }else if($value['hours']){
		    	$t=$value["hours"] .\PhalApi\T("小时"). $value["minutes"] .\PhalApi\T("分").$value["seconds"].\PhalApi\T("秒");
		    }else if($value['minutes']){
		    	$t=$value["minutes"] .\PhalApi\T("分").$value["seconds"].\PhalApi\T("秒");
		    }else if($value['seconds']){
		    	$t=$value["seconds"].\PhalApi\T("秒");
		    }
		    
		    return $t;

	}

	/* 数字格式化 */
	function NumberFormat($num){
		if($num<10000){

		}else if($num<1000000){
			$num=round($num/10000,2).\PhalApi\T('万');
		}else if($num<100000000){
			$num=round($num/10000,1).\PhalApi\T('万');
		}else if($num<10000000000){
			$num=round($num/100000000,2).\PhalApi\T('亿');
		}else{
			$num=round($num/100000000,1).\PhalApi\T('亿');
		}
		return $num;
	}

	/**
	*  @desc 获取推拉流地址
	*  @param string $host 协议，如:http、rtmp、trtc
	*  @param string $stream 流名,如有则包含 .flv、.m3u8
	*  @param int $type 类型，0表示播流，1表示推流
	*/
	function PrivateKeyA($host,$stream,$type){
		$configpri=getConfigPri();
		$cdn_switch=$configpri['cdn_switch'];

		switch($cdn_switch){
			case '1':
				$url=PrivateKey_tx($host,$stream,$type);
				break;
			case '2':
				$url=PrivateKey_sw($host,$stream,$type);
				break;
			
		}

		
		return $url;
	}
	
	/**
	 * 声网推拉流地址
	 * @param string $host 协议，如:http、rtmp、trtc
	*  @param string $stream 流名,可包含 .flv、.m3u8 如1_1706839838.flv
	*  @param int $type 类型，0表示播流，1表示推流
	*  https://doc.shengwang.cn/doc/fusion-cdn/restful/basic-features/streaming-url#%E6%8B%BC%E6%8E%A5%E6%8E%A8%E6%B5%81-url
	 */
	function PrivateKey_sw($host,$stream,$type){
		
		$configpri=\App\getConfigPri();

		$now=time();
		$stream_arr=explode('.',$stream);
		$streamKey = isset($stream_arr[0])? $stream_arr[0] : '';
    	$ext = isset($stream_arr[1])? $stream_arr[1] : 'flv';

    	//推流【固定为rtmp协议】
		if($type==1){
			$push_url=$configpri['sw_push_url'];
			$push_url_key=$configpri['sw_push_key'];
			$push_url_key_time=$configpri['sw_push_length'];

			
			$safe_url='';
			$url="rtmp://{$push_url}/live/{$streamKey}";

			if($push_url_key!=''){
				$txTime = $now + $push_url_key_time*60;
				$safe_url="?ts={$txTime}&sign=".strtolower(md5("{$push_url_key}/live/{$streamKey}{$txTime}"));
			}

			return $url.$safe_url;

		}else{

			$pull_url=$configpri['sw_pull_url'];
			$pull_url_key=$configpri['sw_pull_key'];
			$pull_url_key_time=$configpri['sw_pull_length'];

			if($host=='rtmp'){
				$url="rtmp://{$pull_url}/live/{$streamKey}";
			}else{
				$url="https://{$pull_url}/live/{$streamKey}.".$ext;
			}

			if($pull_url_key!=''){
				$txTime = $now + $pull_url_key_time*60;

				$safe_url="?ts={$txTime}&sign=".strtolower(md5("{$pull_url_key}/live/{$streamKey}.{$ext}{$txTime}"));

				$url.=$safe_url;
			}

            return $url;

		}

		
	}

	/**
	*  @desc 腾讯云推拉流地址
	*  @param string $host 协议，如:http、rtmp、trtc
	*  @param string $stream 流名,可包含 .flv、.m3u8 如1_1706839838.flv
	*  @param int $type 类型，0表示播流，1表示推流
	*/
	function PrivateKey_tx($host,$stream,$type){

		$configpri=getConfigPri();

		$stream_arr=explode('.',$stream);
		$streamKey = isset($stream_arr[0])? $stream_arr[0] : '';
        $ext = isset($stream_arr[1])? $stream_arr[1] : '';

        $streamkey_arr=explode('_',$streamKey);

		$uid = $streamkey_arr[0];
		$showid = $streamkey_arr[1];

		$now=time();

		if($type==1){

			//TRTC推流
			$url = getTxTrtcUrl($uid,$streamKey,1);
			
		}else{

			//rtmp播流
			
			$pull=$configpri['tx_pull'];
			$play_url_key=$configpri['tx_play_key'];
			$play_safe_url='';
			$live_code=$streamKey;

			//后台开启了播流鉴权
			if($configpri['tx_play_key_switch']){
				//播流鉴权时间
				
				$play_auth_time=$now+(int)$configpri['tx_play_time'];
				$txPlayTime = dechex($play_auth_time);
				$txPlaySecret = md5($play_url_key . $live_code . $txPlayTime);
				$play_safe_url = "?txSecret=" .$txPlaySecret."&txTime=" .$txPlayTime;

			}

			$url = "http://{$pull}/live/" . $live_code . ".flv".$play_safe_url;
			
			if($ext){
                $url = "http://{$pull}/live/" . $live_code . ".".$ext.$play_safe_url;
            }
			
			$configpub=getConfigPub();
			if(strstr($configpub['site'],'https')){
                $url=str_replace('http:','https:',$url);
            }

            //TRTC播流
			//$url=getTxTrtcUrl($uid,$streamKey,0);

		}
		
		return $url;
	}

	/**
	*  @desc 获取腾讯云trtc推流/播流地址
	*  @param int $uid 观看用户id
	*  @param string $stream 流名 如:31258_1675238014
	*  @param int $type 流类型 0 播流 1 推流
	*/
	function getTxTrtcUrl($uid,$stream,$type=0){
		$configpri=getConfigPri();
		$appId=$configpri['tencentRTC_appid'] ?? '';
		$appKey=$configpri['tencentRTC_appkey'] ?? '';
		if($appId==='' || $appKey===''){
			return '';
		}
		$now=time();
		$expire=86400;
		$content="TLS.identifier:{$uid}\nTLS.sdkappid:{$appId}\nTLS.time:{$now}\nTLS.expire:{$expire}\n";
		$payload=array(
			'TLS.ver'=>'2.0','TLS.identifier'=>(string)$uid,'TLS.sdkappid'=>(int)$appId,
			'TLS.expire'=>$expire,'TLS.time'=>$now,
			'TLS.sig'=>base64_encode(hash_hmac('sha256',$content,$appKey,true)),
		);
		$userSign=str_replace(array('+','/','='),array('*','-','_'),base64_encode(gzcompress(json_encode($payload),6)));
		$streamType=((int)$type===0) ? 'play' : 'push';
		return 'trtc://cloud.tencent.com/'.$streamType.'/'.$stream.'?sdkappid='.$appId.'&userId='.$uid.'&usersig='.$userSign.'&appscene=live';
	}


    /* 游戏类型 */
    function getGame($action){
        $game_action=array(
            '0'=>'',
            '1'=>\PhalApi\T('智勇三张'),
            '2'=>\PhalApi\T('海盗船长'),
            '3'=>\PhalApi\T('转盘'),
            '4'=>\PhalApi\T('开心牛仔'),
            '5'=>\PhalApi\T('二八贝'),
        );
        
        return isset($game_action[$action])?$game_action[$action]:'';
    }
    
	/* 获取用户VIP: 功能已下线，保留字段形状以兼容客户端 */
	function getUserVip($uid){
		return array(
			'type'=>'0',
			'endtime'=>''
		);
	}

	/* 获取用户坐骑 */
	function getUserCar($uid){

		//语言包
		$rs=array(
			'id'=>'0',
			'swf'=>'',
			'swftime'=>'0',
			'words'=>'',
			'words_en'=>'',
		);

		if($uid<1){
			return $rs;
		}

		$nowtime=time();
		
		$key='car_'.$uid;
		$isexist=getcaches($key);
		if(!$isexist){
			$isexist=\PhalApi\DI()->notorm->car_user
						->select("*")
						->where('uid=? and status=1',$uid)
						->fetchOne();
			if($isexist){
				setcaches($key,$isexist);
			}
        }
		if($isexist){
			if($isexist['endtime']<= $nowtime){
				return $rs;
			}
			$key2='carinfo';
			$car_list=getcaches($key2);
			if(!$car_list){
				$car_list=\PhalApi\DI()->notorm->car
					->select("*")
                    ->order("list_order asc")
					->fetchAll();
				if($car_list){
					setcaches($key2,$car_list);
				}
			}
			$info=array();
			if($car_list){
				foreach($car_list as $k=>$v){
					if($v['id']==$isexist['carid']){
						$info=$v;
					}	
				}
				
				if($info){
					$rs['id']=$info['id'];
					$rs['swf']=get_upload_path($info['swf']) ;
					$rs['swftime']=$info['swftime'];
					$rs['words']=$info['words'];
					$rs['words_en']=$info['words_en'];
                }
			}
			
		}
		
		return $rs;
	}

	/* 获取用户靓号: 功能已下线，保留字段形状以兼容客户端 */
	function getUserLiang($uid){
		return array(
			'name'=>'0',
		);
	}
	
	/* 邀请奖励 */
	function setAgentProfit($uid,$total){

		$distribut1=0;
		$configpri=getConfigPri();
		if($configpri['agent_switch']==1){
			$agent=\PhalApi\DI()->notorm->agent
				->select("*")
				->where('uid=?',$uid)
				->fetchOne();
			$isinsert=0;
			/* 一级 */
			if($agent['one_uid'] && $configpri['distribut1']){
				$distribut1=$total*$configpri['distribut1']*0.01;
                if($distribut1>0){
                    $profit=\PhalApi\DI()->notorm->agent_profit
                        ->select("*")
                        ->where('uid=?',$agent['one_uid'])
                        ->fetchOne();
                    if($profit){
                        \PhalApi\DI()->notorm->agent_profit
                            ->where('uid=?',$agent['one_uid'])
                            ->update(array('one_profit' => new \NotORM_Literal("one_profit + {$distribut1}")));
                    }else{
                        \PhalApi\DI()->notorm->agent_profit
                            ->insert(array('uid'=>$agent['one_uid'],'one_profit' =>$distribut1 ));
                    }
                    \PhalApi\DI()->notorm->user
                            ->where('id=?',$agent['one_uid'])
                            ->update(array('coin' => new \NotORM_Literal("coin + {$distribut1}")));
                    $isinsert=1;
                    $insert_votes=[
                        'type'=>'1',
                        'action'=>'3',
                        'uid'=>$agent['one_uid'],
                        'fromid'=>$uid,
                        'total'=>$distribut1,
                        'votes'=>$distribut1,
                        'addtime'=>time(),
                    ];
                    \PhalApi\DI()->notorm->user_voterecord->insert($insert_votes);
                }
			}
			
			if($isinsert==1){
				$data=array(
					'uid'=>$uid,
					'total'=>$total,
					'one_uid'=>$agent['one_uid'],
					'one_profit'=>$distribut1,
					'addtime'=>time(),
				);
				
				\PhalApi\DI()->notorm->agent_profit_recode->insert( $data );
				
			}
		}
		return 1;
		
	}
    
    /* 家族分成 */
    function setFamilyDivide($liveuid,$total){
        $configpri=getConfigPri();
	
		$anthor_total=$total;
		/* 家族 */
		if($configpri['family_switch']==1){
			$users_family=\PhalApi\DI()->notorm->family_user
							->select("familyid,divide_family")
							->where('uid=? and state=2',$liveuid)
							->fetchOne();

			if($users_family){
				$familyinfo=\PhalApi\DI()->notorm->family
							->select("uid,divide_family,disable")
							->where('id=? and state=2',$users_family['familyid'])
							->fetchOne();

                if($familyinfo){

                	if($familyinfo['disable']==1){
						return $anthor_total;
					}

                    $divide_family=$familyinfo['divide_family'];

                    /* 主播 */
                    if( $users_family['divide_family']>=0){
                        $divide_family=$users_family['divide_family'];
                        
                    }
                    $family_total=$total * $divide_family * 0.01;
                    
                        $anthor_total=floor(($total - $family_total)*100)/100;
                        $addtime=time();
                        $time=date('Y-m-d',$addtime);
                        \PhalApi\DI()->notorm->family_profit
                               ->insert(array("uid"=>$liveuid,"time"=>$time,"addtime"=>$addtime,"profit"=>$family_total,"profit_anthor"=>$anthor_total,"total"=>$total,"familyid"=>$users_family['familyid']));

                    if($family_total){
                        
                        \PhalApi\DI()->notorm->user
                                ->where('id = ?', $familyinfo['uid'])
                                ->update( array( 'coin' => new \NotORM_Literal("coin + {$family_total}")  ));
                                
                        $insert_votes=[
                            'type'=>'1',
                            'action'=>'4',
                            'uid'=>$familyinfo['uid'],
                            'fromid'=>$liveuid,
                            'total'=>$family_total,
                            'votes'=>$family_total,
                            'addtime'=>time(),
                        ];
                        \PhalApi\DI()->notorm->user_voterecord->insert($insert_votes);
                    }
                }
			}	
		}
        return $anthor_total;
    }
	
	/* ip限定 */
	function ip_limit(){
		$configpri=getConfigPri();
		if($configpri['iplimit_switch']==0){
			return 0;
		}
		$date = date("Ymd");
		$ip= ip2long($_SERVER["REMOTE_ADDR"]) ; 
		
		$isexist=\PhalApi\DI()->notorm->getcode_limit_ip
				->select('ip,date,times')
				->where(' ip=? ',$ip) 
				->fetchOne();
		if(!$isexist){
			$data=array(
				"ip" => $ip,
				"date" => $date,
				"times" => 1,
			);
			$isexist=\PhalApi\DI()->notorm->getcode_limit_ip->insert($data);
			return 0;
		}elseif($date == $isexist['date'] && $isexist['times'] >= $configpri['iplimit_times'] ){
			return 1;
		}else{
			if($date == $isexist['date']){
				$isexist=\PhalApi\DI()->notorm->getcode_limit_ip
						->where(' ip=? ',$ip) 
						->update(array('times'=> new \NotORM_Literal("times + 1 ")));
				return 0;
			}else{
				$isexist=\PhalApi\DI()->notorm->getcode_limit_ip
						->where(' ip=? ',$ip) 
						->update(array('date'=> $date ,'times'=>1));
				return 0;
			}
		}	
	}	
    
    /* 验证码记录 */
    function setSendcode($data){
        if($data){
            $data['addtime']=time();
            \PhalApi\DI()->notorm->sendcode->insert($data);
        }
    }

    /* 检测用户是否存在 */
    function checkUser($where){
        if($where==''){
            return 0;
        }

        $isexist=\PhalApi\DI()->notorm->user->where($where)->fetchOne();
        
        if($isexist){
            return 1;
        }
        
        return 0;
    }
    
    /* 直播分类 */
    function getLiveClass(){
        $key="getLiveClass";
		$list=getcaches($key);
		if(!$list){
            $list=\PhalApi\DI()->notorm->live_class
					->select("*")
                    ->order("list_order asc,id desc")
					->fetchAll();
            if($list){
                setcaches($key,$list);
            }
			
		}
		
		//语言包
		$language=\PhalApi\DI()->language;
        
        foreach($list as $k=>$v){
            $v['thumb']=get_upload_path($v['thumb']);
			if($language=='en'){
				$v['name']=$v['name_en'];
				$v['des']=$v['des_en'];
			}

			$v['id']=(string)$v['id'];

			unset($v['name_en']);
			unset($v['des_en']);
					
            $list[$k]=$v;
        }
        return $list;        
        
    }

    
    /* 校验签名 */
    function checkSign($data,$sign){
        $key=\PhalApi\DI()->config->get('app.sign_key');
        $str='';
        ksort($data);
        foreach($data as $k=>$v){
            $str.=$k.'='.$v.'&';
        }
        
        $str.=$key;
        $newsign=md5($str);

        if($sign==$newsign){
            return 1;
        }
        return 0;
    }

    
	/*获取音乐信息*/
	function getMusicInfo($user_nickname,$musicid){

		$res=\PhalApi\DI()->notorm->music->select("id,title,author,img_url,length,file_url,use_nums")->where("id=?",$musicid)->fetchOne();

		if(!$res){
			$res=array();
			$res['id']='0';
			$res['title']='';
			$res['author']='';
			$res['img_url']='';
			$res['length']='00:00';
			$res['file_url']='';
			$res['use_nums']='0';
			$res['music_format']='@'.\PhalApi\T('{name}创作的原声',['name'=>$user_nickname]);

		}else{
			$res['music_format']=$res['title'].'--'.$res['author'];
			$res['img_url']=get_upload_path($res['img_url']);
			$res['file_url']=get_upload_path($res['file_url']);
		}

		

		return $res;

	}

	/*距离格式化*/
	function distanceFormat($distance){
		if($distance<1000){
			return $distance.\PhalApi\T('米');
		}else{

			if(floor($distance/10)<10){
				return number_format($distance/10,1);  //保留一位小数，会四舍五入
			}else{
				return \PhalApi\T(">10千米");
			}
		}
	}

	/* 视频是否点赞 */
	function ifLike($uid,$videoid){
		$like=\PhalApi\DI()->notorm->video_like
				->select("id")
				->where("uid='{$uid}' and videoid='{$videoid}'")
				->fetchOne();
		if($like){
			return 1;
		}else{
			return 0;
		}	
	}

    
    /* 拉黑视频名单 */
	function getVideoBlack($uid){
		$videoids=array('0');
		$list=\PhalApi\DI()->notorm->video_black
						->select("videoid")
						->where("uid='{$uid}'")
						->fetchAll();
		if($list){
			$videoids=array_column($list,'videoid');
		}
		
		$videoids_s=implode(",",$videoids);
		
		return $videoids_s;
	}

    /* 生成二维码 */
    
    function scerweima($url='',$type=0,$uid=0){

    	if($type==1){
    		$key=$uid;
    	}else{
    		$key=md5($url);
    	}
        
        //生成二维码图片
        $filename2 = '/upload/qr/'.$key.'.png';
        $filename = API_ROOT.'/../public/upload/qr/'.$key.'.png';
        
        //if(!file_exists($filename)){
            require_once API_ROOT.'/../sdk/phpqrcode/phpqrcode.php';
            
            $value = $url;					//二维码内容
            
            $errorCorrectionLevel = 'H';	//容错级别 
            $matrixPointSize = 6.2068965517241379310344827586207;			//生成图片大小  
            
            //生成二维码图片
            \QRcode::png($value,$filename , $errorCorrectionLevel, $matrixPointSize, 2); 
       // }
      
        return $filename2;
    }
    
    /* 奖池信息 */
    function getJackpotInfo(){
        $jackpotinfo = \PhalApi\DI()->notorm->jackpot->where("id = 1 ") -> fetchOne();
        return $jackpotinfo;
    }
    
    /* 奖池配置 */
    function getJackpotSet(){
        $key='jackpotset';
		$config=getcaches($key);
		if(!$config){
			$config= \PhalApi\DI()->notorm->option
					->select('option_value')
					->where("option_name='jackpot'")
					->fetchOne();
            $config=json_decode($config['option_value'],true);
            if($config){
                setcaches($key,$config);
            }
			
		}
		return 	$config;
    }
    
    /* 奖池等级设置 */
    function getJackpotLevelList(){
        $key='jackpot_level';
        $list=getcaches($key);
        if(!$list){
            $list= \PhalApi\DI()->notorm->jackpot_level->order("level_up asc")->fetchAll();
            if($list){
                setcaches($key,$list);
            }
        }
        return $list;
    }

    /* 奖池等级 */
    function getJackpotLevel($experience){
        $levelid='0';

		$level=getJackpotLevelList();

		foreach($level as $k=>$v){
			if( $v['level_up']<=$experience){
				$levelid=$v['levelid'];
			}
		}

		return (string)$levelid;
    }
    /* 奖池中奖配置 */
    function getJackpotRate(){
        $key='jackpot_rate';
        $list=getcaches($key);
        if(!$list){
            $list= \PhalApi\DI()->notorm->jackpot_rate->order("id desc")->fetchAll();
            if($list){
                setcaches($key,$list);
            }
        }
        return $list;
    }

    /* 幸运礼物中奖配置 */
    function getLuckRate(){
        $key='gift_luck_rate';
        $list=getcaches($key);
        if(!$list){
            $list= \PhalApi\DI()->notorm->gift_luck_rate->order("id desc")->fetchAll();
            if($list){
                setcaches($key,$list);
            }
        }
        return $list;
    }
    
    /* 视频数据处理 */
    function handleVideo($uid,$v){
        
			$userinfo=getUserInfo($v['uid']);
			if(!$userinfo){
				$userinfo['user_nickname']=\PhalApi\T("已删除");
			}

			//防止uid为0时因为找不到用户信息而出现头像昵称为null的问题
			$v['user_nickname']=$userinfo['user_nickname'];
			$v['avatar']=$userinfo['avatar'];
			
			$v['userinfo']=$userinfo;
			$v['datetime']=datetime($v['addtime']);	
			$v['addtime']=date('Y-m-d H:i:s',$v['addtime']);	
			$v['comments']=NumberFormat($v['comments']);	
			$v['likes']=NumberFormat($v['likes']);	
			$v['steps']=NumberFormat($v['steps']);	
            
            $v['islike']='0';	
            $v['isstep']='0';	
            $v['isattent']='0';
            
			if($uid>0){
				$v['islike']=(string)ifLike($uid,$v['id']);
			}
            
            if($uid>0 && $uid!=$v['uid']){
                $v['isattent']=(string)isAttention($uid,$v['uid']);	
            }
            
			$v['thumb']=get_upload_path($v['thumb']);
			$v['thumb_s']=get_upload_path($v['thumb_s']);
			$v['href']=get_upload_path($v['href']);
			$v['href_w']=get_upload_path($v['href_w']);
            
            $v['ad_url']=get_upload_path($v['ad_url']);

            if($v['ad_endtime']>0 &&($v['ad_endtime']<time())){
                $v['ad_url']='';
            }

            if($v['music_id']){
            	$music_info = \PhalApi\DI()->notorm->music->select("title,img_url")->where(['id'=>$v['music_id']])->fetchOne();

            	if(!$music_info){
            		$v['music_img']='';
            		$v['music_title']='';
            	}else{
            		$v['music_img']=get_upload_path($music_info['img_url']);
            		$v['music_title']=$music_info['title'];
            	}



            }else{
            	$v['music_img']='';
            	$v['music_title']='';
            }
            
			unset($v['ad_endtime']);
			unset($v['orderno']);
			unset($v['isdel']);
			unset($v['show_val']);
			unset($v['xiajia_reason']);
			unset($v['nopass_time']);
			unset($v['watch_ok']);

        return $v;
    }
    //账号是否禁用
	function  isBan($uid){

		$result= \PhalApi\DI()->notorm->user->where("end_bantime>? and id=?",time(),$uid)->fetchOne();
		if($result){
			return 0;
		}
		
		return 1;
	}
	/* 时长格式化 */
	function getBanSeconds($cha,$type=0){		 
		$iz=floor($cha/60);
		$hz=floor($iz/60);
		$dz=floor($hz/24);
		/* 秒 */
		$s=$cha%60;
		/* 分 */
		$i=floor($iz%60);
		/* 时 */
		$h=floor($hz/24);
		/* 天 */
        
        if($type==1){
            if($s<10){
                $s='0'.$s;
            }
            if($i<10){
                $i='0'.$i;
            }

            if($h<10){
                $h='0'.$h;
            }
            
            if($hz<10){
                $hz='0'.$hz;
            }
            return $hz.':'.$i.':'.$s; 
        }
        
		
		if($cha<60){
			return $cha.\PhalApi\T('秒');
		}else if($iz<60){
			return $iz.\PhalApi\T('分钟').$s.\PhalApi\T('秒');
		}else if($hz<24){
			return $hz.\PhalApi\T('小时').$i.\PhalApi\T('分钟');
		}else if($dz<30){
			return $dz.\PhalApi\T('天').$h.\PhalApi\T('小时');
		}
	}	
	
	/* 过滤：敏感词 */
	function sensitiveField($field){
		if($field){
			$configpri=getConfigPri();
			
			$sensitive_words=$configpri['sensitive_words'];
			
			$sensitive=explode(",",$sensitive_words);
			$replace=array();
			$preg=array();
			
			foreach($sensitive as $k=>$v){
				if($v!=''){
					if(strstr($field, $v)!==false){
						return 1001;
					}
				}else{
					unset($sensitive[$k]);
				}
			}
		}
		return 1;
	}
	 /* 视频分类 */
    function getVideoClass(){
        $key="getVideoClass";
		$list=getcaches($key);
		if(!$list){
            $list=\PhalApi\DI()->notorm->video_class
					->select("*")
                    ->order("list_order asc,id desc")
					->fetchAll();
			setcaches($key,$list); 
		}

		//语言包
		$language=\PhalApi\DI()->language;
		foreach ($list as $k => $v) {
			if($language=='en'){
				$list[$k]['name']=$v['name_en'];
			}
			$list[$k]['id']=(string)$v['id'];
			$list[$k]['list_order']=(string)$v['list_order'];

			unset($list[$k]['name_en']);
		}
        return $list;        
        
    }
	 /* 动态数据处理 */
    function handleDynamic($uid,$v){
        $v['addtime']=isset($v['addtime']) ? $v['addtime'] : time();
        $v['city']=isset($v['city']) ? $v['city'] : '';
        $v['address']=isset($v['address']) ? $v['address'] : '';
        $v['labelid']=isset($v['labelid']) ? $v['labelid'] : '0';
     
				$v['datetime']=datetime($v['addtime']);
				if(!$v['city']){
					$v['city']=\PhalApi\T("好像在火星");
				}
				if($v['thumb']){
					$thumbs=explode(";",$v['thumb']);
					foreach($thumbs as $kk=>$vv){
					
						$thumbs[$kk]=get_upload_path($vv);
					}
					$v['thumbs']=$thumbs;
				}else{
					$v['thumbs']=array();
				}
				
				if($v['video_thumb']){
					$v['video_thumb']=get_upload_path($v['video_thumb']);
				}
			   
				if($v['voice']){
					$v['voice']=get_upload_path($v['voice']);
				}
				if($v['href']){
					$v['href']=get_upload_path($v['href']);
				}
				
				$v['likes']=NumberFormat($v['likes']);
				$v['comments']=NumberFormat($v['comments']);
				
				if($uid<1){
					$v['islike']='0';
				}else{
					if($v['uid']==$uid){
						$v['islike']='0';
					}else{
						$v['islike']=isdynamiclike($uid,$v['id']);
					}
				}
				
				$userinfo=getUserInfo($v['uid']);
				$user['id']=$userinfo['id'];
				$user['user_nickname']=$userinfo['user_nickname'];
				$user['avatar']=$userinfo['avatar'];
				$user['avatar_thumb']=$userinfo['avatar_thumb'];
				$user['sex']=$userinfo['sex'];
				$user['isAttention']=isAttention($uid,$v['uid']);
				
				
				$v['userinfo']=$user;
				
				/* 标签 */
				$label_name='';
				if($v['labelid']>0){
					$labelinfo=getLabelInfo($v['labelid']);
					if($labelinfo){
						$label_name='#'.$labelinfo['name'];
					}else{
						$v['labelid']='0';
					}
				}
				$v['label_name']=$label_name;

			return $v;
    }
	
	
	
	/* 标签信息 */
    function getLabelInfo($labelid){
        $key='LabelInfo_'.$labelid;
        $info=getcaches($key);

        //语言包
        
        $language=\PhalApi\DI()->language;

        if(!$info){
            $info=\PhalApi\DI()->notorm->dynamic_label
                ->select("id,name,name_en,thumb")
                ->where('id=?',$labelid)
                ->fetchOne();
            if($info){
                setcaches($key,$info);
            }
        }
        if($info){
            $info['thumb']=get_upload_path($info['thumb']);

            if($language=='en'){
            	$info['name']=$info['name_en'];
            }
        }
        
        return $info;
    }
	
	 /* 动态：是否点赞 */
	function isdynamiclike($uid,$dynamicid) {
        
		$isexist=\PhalApi\DI()->notorm->dynamic_like
						->select("id")
						->where("uid='{$uid}' and dynamicid='{$dynamicid}'")
						->fetchOne();
        if($isexist){
            return '1';
        }
        
		return '0';
	}
    
    function getPageDirectLiveTask($stream){
        $stream=(string)$stream;
        if($stream===''){
            return null;
        }

        return \PhalApi\DI()->notorm->virtual_live_task
            ->select("id,uid,stream,source_page,pull_url,updatetime")
            ->where("stream=? and source_type=3 and status=1",$stream)
            ->fetchOne();
    }

    function isDirectLivePullUrl($url){
        return is_string($url) && preg_match('~^https?://.+\.(m3u8|flv|mp4)(\?|#|$)~i',trim($url));
    }

    function getPagePullUserAgent(){
        return 'Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1';
    }

    function fetchPagePullText($url){
        if(!function_exists('curl_init')){
            return '';
        }

        $ch=curl_init($url);
        curl_setopt($ch,CURLOPT_RETURNTRANSFER,true);
        curl_setopt($ch,CURLOPT_FOLLOWLOCATION,true);
        curl_setopt($ch,CURLOPT_SSL_VERIFYPEER,false);
        curl_setopt($ch,CURLOPT_SSL_VERIFYHOST,false);
        curl_setopt($ch,CURLOPT_CONNECTTIMEOUT,6);
        curl_setopt($ch,CURLOPT_TIMEOUT,14);
        curl_setopt($ch,CURLOPT_HTTPHEADER,array(
            'User-Agent: '.getPagePullUserAgent(),
            'Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8',
        ));
        $body=curl_exec($ch);
        $status=(int)curl_getinfo($ch,CURLINFO_HTTP_CODE);
        curl_close($ch);

        if($body===false || $status>=400){
            return '';
        }
        return (string)$body;
    }

    function decodePagePullJsString($value){
        $decoded=json_decode('"'.$value.'"',true);
        if(is_string($decoded)){
            return str_replace('\\/','/',$decoded);
        }
        return str_replace('\\/','/',$value);
    }

    function absolutePagePullUrl($base,$value){
        $value=trim((string)$value);
        if(preg_match('#^https?://#i',$value)){
            return $value;
        }
        if(strpos($value,'//')===0){
            $scheme=parse_url($base,PHP_URL_SCHEME) ?: 'https';
            return $scheme.':'.$value;
        }

        $parts=parse_url($base);
        if(!$parts || empty($parts['host'])){
            return $value;
        }
        $scheme=$parts['scheme'] ?? 'https';
        $host=$parts['host'];
        $port=isset($parts['port']) ? ':'.$parts['port'] : '';
        if(strpos($value,'/')===0){
            return $scheme.'://'.$host.$port.$value;
        }

        $path=$parts['path'] ?? '/';
        $dir=preg_replace('#/[^/]*$#','/',$path);
        return $scheme.'://'.$host.$port.$dir.$value;
    }

    function findPagePullHlsSource($page,$html){
        if(preg_match('/window\.initialRoomDossier\s*=\s*"(.*?)";/s',$html,$matches)){
            $json=decodePagePullJsString($matches[1]);
            $dossier=json_decode($json,true);
            if(is_array($dossier)){
                $hls=trim((string)($dossier['hls_source'] ?? ''));
                if($hls!==''){
                    return $hls;
                }
            }
        }

        if(preg_match('/https?:\\\\?\/\\\\?\/[^"\'<>\s]+?\.m3u8[^"\'<>\s]*/',$html,$matches)){
            return decodePagePullJsString($matches[0]);
        }

        if(preg_match('/["\']([^"\']+?\.m3u8[^"\']*)["\']/',$html,$matches)){
            return absolutePagePullUrl($page,decodePagePullJsString($matches[1]));
        }

        return '';
    }

    function resolvePagePullHlsUrl($page){
        $page=trim((string)$page);
        if($page===''){
            return '';
        }
        $html=fetchPagePullText($page);
        if($html===''){
            return '';
        }
        $hls=findPagePullHlsSource($page,$html);
        return isDirectLivePullUrl($hls) ? $hls : '';
    }

    function refreshPageDirectPullIfNeeded($task,$live_info){
        if(!$task || empty($task['source_page'])){
            return trim((string)($live_info['pull'] ?? ''));
        }

        $pull=trim((string)($task['pull_url'] ?? ''));
        $updated=(int)($task['updatetime'] ?? 0);
        if(isDirectLivePullUrl($pull) && time()-$updated<60){
            return $pull;
        }

        $resolved=resolvePagePullHlsUrl($task['source_page']);
        if($resolved===''){
            return $pull!=='' ? $pull : trim((string)($live_info['pull'] ?? ''));
        }

        \PhalApi\DI()->notorm->virtual_live_task
            ->where("id=?",$task['id'])
            ->update(array('pull_url'=>$resolved,'updatetime'=>time()));
        \PhalApi\DI()->notorm->live
            ->where("stream=?",$task['stream'])
            ->update(array('pull'=>$resolved));

        return $resolved;
    }

    function applyPageDirectPullInfo(&$v){
        $task=getPageDirectLiveTask($v['stream'] ?? '');
        if(!$task || empty($task['source_page'])){
            return false;
        }

        $v['isvideo']='1';
        $pull=trim((string)($task['pull_url'] ?? ''));
        if($pull===''){
            $pull=trim((string)($v['pull'] ?? ''));
        }
        $isDirect=isDirectLivePullUrl($pull);
        $v['pull_type']=$isDirect ? 'hls' : 'page_hls';
        $v['source_page']=$task['source_page'];
        $v['pull']=$pull!=='' ? $pull : $task['source_page'];
        return true;
    }

    /* 处理直播信息 */
    function handleLive($v){
        
        $configpri=getConfigPri();
        
        $nums=zCard('user_'.$v['stream']);
        $v['nums']=(string)$nums;
        
        $userinfo=getUserInfo($v['uid']);
        $v['avatar']=$userinfo['avatar'];
        $v['avatar_thumb']=$userinfo['avatar_thumb'];
        $v['user_nickname']=$userinfo['user_nickname'];
        $v['sex']=$userinfo['sex'];
        $v['level']=$userinfo['level'];
        $v['level_anchor']=$userinfo['level_anchor'];
        
        if(!$v['thumb']){
            $v['thumb']=$v['avatar'];
        }
        $isPageDirect=applyPageDirectPullInfo($v);
        if($isPageDirect && isset($v['hotvotes']) && (int)$v['hotvotes']>0){
            $v['nums']=(string)(int)$v['hotvotes'];
        }
        if($isPageDirect){
            // PAGE 直拉由客户端解析 HLS，不再走声网频道或 CDN 转推。
        }else if($v['isvideo']==0 && $configpri['cdn_switch']!=5){
            if(isset($v['deviceinfo']) && $v['deviceinfo']=='virtual_live'){
                $v['pull']=!empty($v['pull']) ? $v['pull'] : PrivateKeyA('http',$v['stream'].'.flv',0);
            }else{
                $v['pull']=PrivateKeyA('rtmp',$v['stream'],0);
            }
        }
        
        if($v['type']==1){
            $v['type_val']='';
        }
		$v['thumb']=get_upload_path($v['thumb']);
        $v['game']=getGame($v['game_action']);
        
        return $v;
    }


    /**
	 * 判断是否为合法的身份证号码
	 * @param $mobile
	 * @return int
	 */
	function isCreditNo($vStr){
		
		$vCity = array(
		  	'11','12','13','14','15','21','22',
		  	'23','31','32','33','34','35','36',
		  	'37','41','42','43','44','45','46',
		  	'50','51','52','53','54','61','62',
		  	'63','64','65','71','81','82','91'
		);
		
		if (!preg_match('/^([\d]{17}[xX\d]|[\d]{15})$/', $vStr)){
		 	return false;
		}

	 	if (!in_array(substr($vStr, 0, 2), $vCity)){
	 		return false;
	 	}
	 
	 	$vStr = preg_replace('/[xX]$/i', 'a', $vStr);
	 	$vLength = strlen($vStr);

	 	if($vLength == 18){
	  		$vBirthday = substr($vStr, 6, 4) . '-' . substr($vStr, 10, 2) . '-' . substr($vStr, 12, 2);
	 	}else{
	  		$vBirthday = '19' . substr($vStr, 6, 2) . '-' . substr($vStr, 8, 2) . '-' . substr($vStr, 10, 2);
	 	}

		if(date('Y-m-d', strtotime($vBirthday)) != $vBirthday){
		 	return false;
		}

	 	if ($vLength == 18) {
	  		$vSum = 0;
	  		for ($i = 17 ; $i >= 0 ; $i--) {
	   			$vSubStr = substr($vStr, 17 - $i, 1);
	   			$vSum += (pow(2, $i) % 11) * (($vSubStr == 'a') ? 10 : intval($vSubStr , 11));
	  		}
	  		if($vSum % 11 != 1){
	  			return false;
	  		}
	 	}

	 	return true;
	}


	// 获取用户的余额
	function getUserBalance($uid){
		$res=array(
			'balance'=>'0.00',
			'balance_total'=>'0.00'
		);

		$info=\PhalApi\DI()->notorm->user->where("id=?",$uid)->select("balance,balance_total")->fetchOne();

		if($info){
			$res['balance']=$info['balance'];
			$res['balance_total']=$info['balance_total'];
		}

		return $res;
	}


	//修改用户的余额 type:0 扣除余额 1 增加余额
	function setUserBalance($uid,$type,$balance){

		$res=0;

		if($type==0){ //扣除用户余额，增加用户余额消费总额
			$res=\PhalApi\DI()->notorm->user
				->where("id=? and balance>=?",$uid,$balance)
				->update(array('balance' => new \NotORM_Literal("balance - {$balance}"),'balance_consumption'=>new \NotORM_Literal("balance_consumption + {$balance}")) );

		}else if($type==1){ //增加用户余额

			$res=\PhalApi\DI()->notorm->user
				->where("id=?",$uid)
				->update(array('balance' => new \NotORM_Literal("balance + {$balance}"),'balance_total'=>new \NotORM_Literal("balance_total + {$balance}")) );
		}

		return $res;
		
	}


	//添加余额操作记录
	function addBalanceRecord($data){
		$res=\PhalApi\DI()->notorm->user_balance_record->insert($data);
		return $res;
	}


	/* 时长格式化 */
	function secondsFormat($time){

		$now=time();
		$cha=$now-$time;

		if($cha<60){
			return \PhalApi\T('刚刚');
		}

		if($cha>=4*24*60*60){ //超过4天
			$now_year=date('Y',$now);
			$time_year=date('Y',$time);

			$language=\PhalApi\DI()->language;

			if($now_year==$time_year){
				if($language=='en'){
					return date("d,m",$time);
				}else{
					return date("m月d日",$time);
				}
				
			}else{
				if($language=='en'){
					return date("d,m,Y",$time);
				}else{
					return date("Y年m月d日",$time);
				}
				
			}

		}else{

			$iz=floor($cha/60);
			$hz=floor($iz/60);
			$dz=floor($hz/24);

			if($dz>3){
				return \PhalApi\T('{num}天前',['num'=>3]);
			}else if($dz>2){
				return \PhalApi\T('{num}天前',['num'=>2]);
			}else if($dz>1){
				return \PhalApi\T('{num}天前',['num'=>1]);
			}

			if($hz>1){
				return \PhalApi\T('{num}小时前',['num'=>$hz]);
			}

			return \PhalApi\T('{num}分钟前',['num'=>$iz]);
			

		}

	}


	//判断用户是否注销
	function checkIsDestroyByLogin($country_code,$user_login){
		$user_status=\PhalApi\DI()->notorm->user->where("country_code=? and user_login=?",$country_code,$user_login)->fetchOne('user_status');
		if($user_status==3){
			return 1;
		}

		return 0;
	}

	//判断用户是否注销
	function checkIsDestroyByUid($uid){
		$user_status=\PhalApi\DI()->notorm->user->where("id=?",$uid)->fetchOne('user_status');
		if($user_status==3){
			return 1;
		}

		return 0;
	}

	//获取播流地址
    function getPull($stream){
    	$pull_arr=[
    		'isvideo'=>'0',
    		'pull'=>''
    	];
    	$pull='';
    	$live_info=\PhalApi\DI()->notorm->live->where("stream=?",$stream)->fetchOne();
        if(!$live_info){
            return $pull_arr;
        }

        $task=getPageDirectLiveTask($stream);
        if($task && !empty($task['source_page'])){
            $pull=refreshPageDirectPullIfNeeded($task,$live_info);
            $isDirect=isDirectLivePullUrl($pull);
            $pull_arr['isvideo']='1';
            $pull_arr['pull']=$pull!=='' ? $pull : $task['source_page'];
            $pull_arr['pull_type']=$isDirect ? 'hls' : 'page_hls';
            $pull_arr['source_page']=$task['source_page'];
        }else if($live_info['isvideo']==1){ //视频

    		$pull=$live_info['pull'];
    		$pull_arr['isvideo']='1';
    		$pull_arr['pull']=$pull;
        }else if(isset($live_info['deviceinfo']) && $live_info['deviceinfo']=='virtual_live'){
            // 虚拟直播是服务器推到 CDN 的 HTTP-FLV，不是声网 RTC 频道。
            // Android 在 live_sdk=2 时需要走 URL 播放分支，否则会尝试加入 Agora RTC 房间导致黑屏。
            $pull_arr['isvideo']='1';
            $pull_arr['pull']=!empty($live_info['pull']) ? $live_info['pull'] : PrivateKeyA('http',$stream.'.flv',0);
        }else{
    		
    		$pull=PrivateKeyA('rtmp',$stream,0);

    		$pull_arr['isvideo']='0';
    		$pull_arr['pull']=$pull;
    	}

    	return $pull_arr;
	}

	
	//每日任务处理
	function dailyTasks($uid,$data){
		$configpri=getConfigPri();
		$type=$data['type'];  //type 任务类型 

		$dailytask_switch=$configpri['dailytask_switch'];
		if(!$dailytask_switch){
			return 0;
		}
		
		// 当天时间
		$time=strtotime(date("Y-m-d 00:00:00",time()));
		$where="uid={$uid} and type={$type}";	
		//每日任务
		$info=\PhalApi\DI()->notorm->user_daily_tasks
    			->where($where)
    			->select("*")
    			->fetchOne();
    			
		if($info){

    		if($info['addtime']!=$time){
    			\PhalApi\DI()->notorm->user_daily_tasks
    				->where($where)
    				->delete();
    			$info=[];
    		}else{
    			if($info['state']==1||$info['state']==2){
    				return 1;
    			}
    		}
    	}
				
		$save=[
			'uid'=>$uid,
			'type'=>$type,
			'addtime'=>$time,
			'uptime'=>time(),
		];
		$state='0';
		if($type==1){  //1观看直播
			$target=$configpri['watch_live_term'];
			$reward=$configpri['watch_live_coin'];

			
		}else if($type==2){ //2观看视频
			$target=$configpri['watch_video_term'];
			$reward=$configpri['watch_video_coin'];	

		}else if($type==3){ //3直播奖励
			$target=$configpri['open_live_term']*60;
			$reward=$configpri['open_live_coin'];
			

		}else if($type==4){ //4打赏奖励
			$target=$configpri['award_live_term'];
			$reward=$configpri['award_live_coin'];
			
			$schedule=ceil($data['total']);
			
		}else if($type==5){ //5分享奖励
			$target=$configpri['share_live_term'];
			$reward=$configpri['share_live_coin'];
			
			$schedule=ceil($data['nums']);
		}
		
		//关于时间奖励的处理
		if(in_array($type,['1','2','3'])){
			
			$day=date("d",$data['starttime']); 
			$day2=date("d",$data['endtime']);
			if($day!=$day2){ //判断结束时间是否超过当天, 超过则按照今天凌晨来算
				$data['starttime']=$time;
			}

			$schedulet=0;
			$time_diff=$data['endtime']-$data['starttime'];
			$schedule=$time_diff; //以秒为单位

		}
		
		
		
		if(!$info || $info['addtime']!=$time){  //当数据中查不到当天的数据时
			$save['target']=$target;
			$save['reward']=$reward;

			if(in_array($type,['1','2','3'])){
				$target_format=$target*60;
			}else{
				$target_format=$target;
			}

			if($schedule>=$target_format){
				$schedule=$target_format;
				$state='1';
			}
		}else{  //当有今天的数据时
			$schedule=$info['schedule']+$schedule;
			
			if(in_array($type,['1','2','3'])){
				$target_format=$info['target']*60;
			}else{
				$target_format=$info['target'];
			}

			if($schedule>=$target_format){
				$schedule=$target_format;
				$state='1';
			}
		}
		
		$save['schedule']=(int)$schedule;  //进度
		$save['state']=$state; //状态
		
		
		if(!$info){
			\PhalApi\DI()->notorm->user_daily_tasks->insert($save);
		}else{
			\PhalApi\DI()->notorm->user_daily_tasks->where('id=?',$info['id'])->update($save);
		}

		
		//删除用户每日任务数据
		$key="seeDailyTasks_".$uid;
		delcache($key);
	}
	
	
	//获取动态话题标签列表
	function getDynamicLabels($where,$order,$p,$isp=0){
		
		if($isp){  //是否使用分页
			if($p<1){
				$p=1;
			}
			$nums=20;
			$start=($p-1)*$nums;
		}else{
			$start=0;
			$nums=$p;
		}
		
		//语言包
		$reportlist=\PhalApi\DI()->notorm->dynamic_label
			->select("id,name,name_en,thumb,use_nums")
			->where($where)
			->order($order)
			->limit($start,$nums)
			->fetchAll();

		$language=\PhalApi\DI()->language;
		foreach ($reportlist as $k => $v) {
			if($language=='en'){
				$reportlist[$k]['name']=$v['name_en'];
			}
		}
		
		return $reportlist;
	
	}



	//检测姓名
	function checkUsername($username){
		$preg='/^(?=.*\d.*\b)/';
		$isok = preg_match($preg,$username);
		if($isok){
			return 1;
		}else{
			return 0;
		}
	}


	//检测用户是否填写过邀请码
	function checkAgentIsExist($uid){
		$isexist=\PhalApi\DI()->notorm->agent
                    ->select('*')
                    ->where('uid=?',$uid)
                    ->fetchOne();
        if(!$isexist){
        	return 0;
        }

        return 1;
	}

	//检查语音聊天室是否在直播
	function checkVoiceIsLive($uid,$stream){
		$live_info=\PhalApi\DI()->notorm->live
    	->where("uid=? and stream=? and islive=1 and live_type=1",$uid,$stream)
    	->fetchOne();


    	if(!$live_info){
    		return 0;
    	}

    	return 1;
	}

	//获取语音聊天室麦位信息
	function getVoiceMicInfo($where){
		$mic_info=\PhalApi\DI()->notorm->voicelive_mic
    	->where($where)
    	->fetchOne();

    	return $mic_info;
	}

	//获取rtmp推流和播流地址
	function getLowLatencyStream($stream){

		$push_url=PrivateKeyA('rtmp',$stream,1);
		$play_url=PrivateKeyA('rtmp',$stream,0);
		
        $info=array(
			"pushurl" => $push_url,
			"timestamp" => time(), 
			"playurl" => $play_url,
			"stream"=>$stream
		);

		return $info;
	}

	//获取语音聊天室所有麦位上的用户信息 
	function getVoiceLiveMicList($mic_list,$num){
		$curr_position=0;
    	$new_mic_list=[];

    	$empty_userinfo=array(
				'id'=>'0',
				'uid'=>'0', //ios专用
				'user_nickname'=>'',
				'avatar'=>'',
				'sex'=>'0',
				'level'=>'0',
				'mic_status'=>'0'
			);

    	for ($i=0; $i <$num ; $i++) {
    		$empty_userinfo['position']=(string)$i;
    		$new_mic_list[]=$empty_userinfo;
    	}

    	foreach ($mic_list as $k => $v) {
    		foreach ($new_mic_list as $k1 => $v1) {
    			if($v1['position']==$v['position']){
    				if($v['uid']>0){
    					$userinfo=getUserInfo($v['uid']);
    					$new_userinfo['id']=$userinfo['id'];
    					$new_userinfo['uid']=$userinfo['id']; //ios专用
    					$new_userinfo['user_nickname']=$userinfo['user_nickname'];
    					$new_userinfo['avatar']=$userinfo['avatar'];
    					$new_userinfo['sex']=$userinfo['sex'];
    					$new_userinfo['level']=$userinfo['level'];
    					$new_userinfo['mic_status']=$v['status'];
    					$new_userinfo['position']=$v1['position'];

    					$new_mic_list[$k1]=$new_userinfo;
    				}else{
    					$new_mic_list[$k1]['mic_status']=$v['status'];
    				}

    				break;
    			}
    			
    		}
    	}

    	return $new_mic_list;
	}
	
	
	

	//获取直播类型【视频直播或语音聊天室】
	function getLiveType($liveuid,$stream){
		$live_info=\PhalApi\DI()->notorm->live
			->where("uid=? and stream=?",$liveuid,$stream)
			->select("live_type")
			->fetchOne();

		return $live_info['live_type'];
	}


    //判断用户是否创建家族/是否加入家族
    /* 家族功能已下线，恒返回 0 */
    function checkUserFamily($uid){
        return 0;
    }


    //验证数字是否整数/两位小数
    function checkNumber($num){

    	if(floor($num) ==$num){
    		return 1;
    	}

    	if (preg_match('/^[0-9]+(.[0-9]{1,2})$/', $num)) {
    		return 1;
    	}

    	return 0;
    }

    //检测首充
    function checkUserFirstCharge($uid){
    	$info=\PhalApi\DI()->notorm->user
    		->select("firstcharge_used")
    		->where(['id'=>$uid])
    		->fetchOne();

    	return $info['firstcharge_used'];
    }
    //禁播
    function getLiveBan($uid){
    	$res=array('is_ban'=>0,'endtime'=>0);
    	$live_ban=\PhalApi\DI()->notorm->live_ban
    		->where(['liveuid'=>$uid])
    		->fetchOne();

    	if($live_ban){

    		$now=time();

    		if($live_ban['endtime']==0 && $live_ban['type']=='all'){
    			$res['is_ban']=1;
    		}else if( ($live_ban['endtime'] >0) && ($live_ban['endtime']<=$now) ){
    			\PhalApi\DI()->notorm->live_ban
		    		->where(['liveuid'=>$uid])
		    		->delete();
		    	
    		}else{
    			$res['is_ban']=1;
    			$res['endtime']=$live_ban['endtime'];
    		}


    	}

    	return $res;
    }

    //直播间封禁规则
    function getLiveBanRules(){
    	$rules=[
			[
				'id'=>'1',
				'name'=>'30'.\PhalApi\T('分钟'),
				'type'=>'30min'
			],
			[
				'id'=>'2',
				'name'=>'1'.\PhalApi\T('天'),
				'type'=>'1day'
			],
			[
				'id'=>'3',
				'name'=>'7'.\PhalApi\T('天'),
				'type'=>'7day'
			],
			[
				'id'=>'4',
				'name'=>'15'.\PhalApi\T('天'),
				'type'=>'15day'
			],
			[
				'id'=>'5',
				'name'=>'30'.\PhalApi\T('天'),
				'type'=>'30day'
			],
			[
				'id'=>'6',
				'name'=>'90'.\PhalApi\T('天'),
				'type'=>'90day'
			],
			[
				'id'=>'7',
				'name'=>'180'.\PhalApi\T('天'),
				'type'=>'180day'
			],
			[
				'id'=>'8',
				'name'=>\PhalApi\T('永久'),
				'type'=>'all'
			]
		];

		return $rules;
    }

    //添加用户点赞数
    function addUserPraise($uid,$nums){
    	\PhalApi\DI()->notorm->user
    		->where(['id'=>$uid])
    		->update(
    			array('praise_num' => new \NotORM_Literal("praise_num + {$nums}"))
    		);
    }

    //减少用户点赞数
    function reduceUserPraise($uid,$nums){
    	$praise_num=\PhalApi\DI()->notorm->user
    		->where(['id'=>$uid])
    		->fetchOne('praise_num');

    	if($praise_num>=$nums){
    		\PhalApi\DI()->notorm->user
	    		->where(['id'=>$uid])
	    		->update(
	    			array('praise_num' => new \NotORM_Literal("praise_num - {$nums}"))
	    		);
    	}else{
    		\PhalApi\DI()->notorm->user
	    		->where(['id'=>$uid])
	    		->update(['praise_num'=>0]);
    	}
    }

   /**
    * 腾讯云TPNS移动推送
    * @param  string  $title 推送标题
    * @param  string  $msg   推送消息内容
    * @param  string  $type  推送类型 all 全员推送 single 单账号推送 account_list 账号列表推送
    * @param  integer $uid   单账号用户id
    * @url https://cloud.tencent.com/document/product/548/39064
    */
   function txMessageTpns($title,$msg,$type,$uid=0,$account_list=[],$json_str='',$language='zh-cn'){

   		require_once API_ROOT.'/../sdk/tencentTpns/tpns.php';
   		$configpri=getConfigPri();
   		$area=$configpri['tencentTpns_area'];
   		$accessid_android=$configpri['tencentTpns_accessid_android'];
   		$secretkey_android=$configpri['tencentTpns_secretkey_android'];
   		$accessid_ios=$configpri['tencentTpns_accessid_ios'];
   		$secretkey_ios=$configpri['tencentTpns_secretkey_ios'];
   		$ios_environment=$configpri['tencentTpns_ios_environment'];


   		if(
   			!in_array($area,['guangzhou','shanghai','hongkong','singapore']) || 
   			!$accessid_android || 
   			!$secretkey_android || 
   			!$accessid_ios || 
   			!$secretkey_ios
   		){
   			return;
   		}


   		if($area=='guangzhou'){
			$stub_android = new \tpns\Stub($accessid_android, $secretkey_android, \tpns\GUANGZHOU);
			$stub_ios = new \tpns\Stub($accessid_ios, $secretkey_ios, \tpns\GUANGZHOU);
		}else if($area=='shanghai'){
			$stub_android = new \tpns\Stub($accessid_android, $secretkey_android, \tpns\SHANGHAI);
			$stub_ios = new \tpns\Stub($accessid_ios, $secretkey_ios, \tpns\SHANGHAI);
		}else if($area=='hongkong'){
			$stub_android = new \tpns\Stub($accessid_android, $secretkey_android, \tpns\HONGKONG);
			$stub_ios = new \tpns\Stub($accessid_ios, $secretkey_ios, \tpns\HONGKONG);
		}else if($area=='singapore'){
			$stub_android = new \tpns\Stub($accessid_android, $secretkey_android, \tpns\SINGAPORE);
			$stub_ios = new \tpns\Stub($accessid_ios, $secretkey_ios, \tpns\SINGAPORE);
		}else{
			return;
		}


		if($type=='account_list' && count($account_list)==1){
			$type='single';
			$uid=$account_list[0];
		}

   		
   		if($type=='all'){

   			//Android推送
   			$android = new \tpns\AndroidMessage;
   			if($json_str){
	   			$android->custom_content = $json_str;	
	   		}

	   		//控制通知点击时乱转到指定页面
	   		$action=[
                "action_type"=> 1,// 动作类型，1，打开activity或app本身；2，打开浏览器；3，打开Intent
                "activity"=> "com.yunbao.im.activity.ImMsgNotifyActivity"
            ];

            $tagItem = new \tpns\TagItem;
            $tagItem->tags = array($language);
            $tagItem->tag_type = "xg_user_define";
            

            $tagRule = new \tpns\TagRule;
            $tagRule->tag_items = array($tagItem);

            $android->action=(object)$action;

   			$req_android = \tpns\NewRequest(
		        \tpns\WithAudienceType(\tpns\AUDIENCE_TAG),
		        \tpns\WithMessageType(\tpns\MESSAGE_NOTIFY),
		        \tpns\WithTitle($title),
		        \tpns\WithContent($msg),
		        \tpns\WithTagRules(array($tagRule)),
		        \tpns\WithAndroidMessage($android),
		        \tpns\WithEnvironment(\tpns\ENVIRONMENT_PROD)
		   	);

	   		$result_android = $stub_android->Push($req_android);
	   		//var_dump($result_android);

   			//iOS推送
   			$ios = new \tpns\iOSMessage;
   			if($json_str){
	   			$ios->custom = $json_str;	
	   		}


		   	if($ios_environment==0){ //开发
		   		$req_ios = \tpns\NewRequest(
			        \tpns\WithAudienceType(\tpns\AUDIENCE_TAG),
			        \tpns\WithMessageType(\tpns\MESSAGE_NOTIFY),
			        \tpns\WithTitle($title),
			        \tpns\WithContent($msg),
			        \tpns\WithIOSMessage($ios),
			        \tpns\WithEnvironment(\tpns\ENVIRONMENT_DEV)
			   	);
		   	}else{

		   		$req_ios = \tpns\NewRequest(
			        \tpns\WithAudienceType(\tpns\AUDIENCE_TAG),
			        \tpns\WithMessageType(\tpns\MESSAGE_NOTIFY),
			        \tpns\WithTitle($title),
			        \tpns\WithContent($msg),
			        \tpns\WithIOSMessage($ios),
			        \tpns\WithEnvironment(\tpns\ENVIRONMENT_PROD)
			   	);
		   	}

	   		$result_ios = $stub_ios->Push($req_ios);
	   		//var_dump($result_ios);

   		}else if($type=='single'){

   			if(!$uid){
   				return;
   			}

   			$uid=(string)$uid;

   			$tagItem1 = new \tpns\TagItem;
            $tagItem1->tags = array($language);
            $tagItem1->tag_type = "xg_user_define";


            $tagItem2 = new \tpns\TagItem;
            $tagItem2->tags = array($uid);
            $tagItem2->items_operator = \tpns\TAG_OPERATOR_AND; //tagItem2与tagItem1之间的逻辑关系
            $tagItem2->tag_type = "xg_user_define";
            

            $tagRule = new \tpns\TagRule;
            $tagRule->tag_items = array($tagItem1,$tagItem2);

   			//Android推送
   			$android = new \tpns\AndroidMessage;
   			if($json_str){
	   			$android->custom_content = $json_str;	
	   		}

	   		$action=[
                "action_type"=> 1,// 动作类型，1，打开activity或app本身；2，打开浏览器；3，打开Intent
                "activity"=> "com.yunbao.im.activity.ImMsgNotifyActivity"
            ];

            $android->action=(object)$action;

   			$req_android = \tpns\NewRequest(
		        \tpns\WithAudienceType(\tpns\AUDIENCE_TAG),
		        \tpns\WithMessageType(\tpns\MESSAGE_NOTIFY),
		        \tpns\WithTitle($title),
		        \tpns\WithContent($msg),
		        \tpns\WithAndroidMessage($android),
		        \tpns\WithTagRules(array($tagRule)),
		        \tpns\WithEnvironment(\tpns\ENVIRONMENT_PROD)
		    );

	   		$result_android = $stub_android->Push($req_android);
	   		//var_dump($result_android);

	   		//iOS推送
	   		$ios = new \tpns\iOSMessage;
	   		if($json_str){
	   			$ios->custom = $json_str;	
	   		}
	   		

	   		if($ios_environment==0){ //开发

	   			$req_ios = \tpns\NewRequest(
			        \tpns\WithAudienceType(\tpns\AUDIENCE_TAG),
			        \tpns\WithMessageType(\tpns\MESSAGE_NOTIFY),
			        \tpns\WithTitle($title),
			        \tpns\WithContent($msg),
			        \tpns\WithIOSMessage($ios),
			        \tpns\WithTagRules(array($tagRule)),
			        \tpns\WithEnvironment(\tpns\ENVIRONMENT_DEV)
			    );

	   		}else{
	   			$req_ios = \tpns\NewRequest(
			        \tpns\WithAudienceType(\tpns\AUDIENCE_TAG),
			        \tpns\WithMessageType(\tpns\MESSAGE_NOTIFY),
			        \tpns\WithTitle($title),
			        \tpns\WithContent($msg),
			        \tpns\WithIOSMessage($ios),
			        \tpns\WithTagRules(array($tagRule)),
			        \tpns\WithEnvironment(\tpns\ENVIRONMENT_PROD)
			    );
	   		}
	   		

	   		$result_ios = $stub_ios->Push($req_ios);
	   		//var_dump($result_ios);

   		}else if($type=='account_list'){

   			if(empty($account_list)){
   				return;
   			}

   			$tagItem1 = new \tpns\TagItem;
            $tagItem1->tags = array($language);
            $tagItem1->tag_type = "xg_user_define";


            $tagItem2 = new \tpns\TagItem;
            $tagItem2->tags = $account_list;
            $tagItem2->tags_operator = \tpns\TAG_OPERATOR_OR; //tagItem2内部标签之间的逻辑关系
            $tagItem2->items_operator = \tpns\TAG_OPERATOR_AND; //tagItem2与tagItem1之间的逻辑关系
            $tagItem2->tag_type = "xg_user_define";
            

            $tagRule = new \tpns\TagRule;
            $tagRule->tag_items = array($tagItem1,$tagItem2);

   			//Android推送
   			$android = new \tpns\AndroidMessage;
   			if($json_str){
	   			$android->custom_content = $json_str;	
	   		}

	   		$action=[
                "action_type"=> 1,// 动作类型，1，打开activity或app本身；2，打开浏览器；3，打开Intent
                "activity"=> "com.yunbao.im.activity.ImMsgNotifyActivity"
            ];

            $android->action=(object)$action;

   			$req_android = \tpns\NewRequest(
		        \tpns\WithAudienceType(\tpns\AUDIENCE_TAG),
		        \tpns\WithMessageType(\tpns\MESSAGE_NOTIFY),
		        \tpns\WithTitle($title),
		        \tpns\WithContent($msg),
		        \tpns\WithAndroidMessage($android),
		        \tpns\WithTagRules(array($tagRule)),
		        \tpns\WithEnvironment(\tpns\ENVIRONMENT_PROD)
		    );

		    $result_android = $stub_android->Push($req_android);
	   		//var_dump($result_android);

   			//iOS推送
   			$ios = new \tpns\iOSMessage;
   			if($json_str){
	   			$ios->custom = $json_str;	
	   		}

   			if($ios_environment==0){ //开发
   				$req_ios = \tpns\NewRequest(
			        \tpns\WithAudienceType(\tpns\AUDIENCE_TAG),
			        \tpns\WithMessageType(\tpns\MESSAGE_NOTIFY),
			        \tpns\WithTitle($title),
			        \tpns\WithContent($msg),
			        \tpns\WithIOSMessage($ios),
			        \tpns\WithTagRules(array($tagRule)),
			        \tpns\WithEnvironment(\tpns\ENVIRONMENT_DEV)
			    );
   			}else{
   				$req_ios = \tpns\NewRequest(
			        \tpns\WithAudienceType(\tpns\AUDIENCE_TAG),
			        \tpns\WithMessageType(\tpns\MESSAGE_NOTIFY),
			        \tpns\WithTitle($title),
			        \tpns\WithContent($msg),
			        \tpns\WithIOSMessage($ios),
			        \tpns\WithTagRules(array($tagRule)),
			        \tpns\WithEnvironment(\tpns\ENVIRONMENT_PROD)
			    );
   			}
   			

		    $result_ios = $stub_ios->Push($req_ios);
	   		//var_dump($result_ios);

   		}
   
   }
   
   /* 扣费 */
    function upCoin($uid,$total=0,$type=0){
        if($uid < 1 || $total<=0){
            return 0;
        }
        if($type==1){
            $ifok =\PhalApi\DI()->notorm->user
                    ->where('id = ? and coin >=?', $uid,$total)
                    ->update(array('coin' => new \NotORM_Literal("coin - {$total}") ) );
            
            return $ifok;
        }
        $ifok =\PhalApi\DI()->notorm->user
				->where('id = ? and coin >=?', $uid,$total)
				->update(array('coin' => new \NotORM_Literal("coin - {$total}"),'consumption' => new \NotORM_Literal("consumption + {$total}") ) );
        return $ifok;
    }
	
	/* 退费 */
    function addCoin($uid,$total=0,$type=0){
        if($uid < 1 || $total<=0){
            return 0;
        }
        if($type==1){
            $ifok =\PhalApi\DI()->notorm->user
                    ->where('id = ? ', $uid)
                    ->update(array('coin' => new \NotORM_Literal("coin + {$total}") ) );
            
            return $ifok;
        }
        $ifok =\PhalApi\DI()->notorm->user
				->where('id = ? ', $uid)
				->update(array('coin' => new \NotORM_Literal("coin + {$total}"),'consumption' => new \NotORM_Literal("consumption - {$total}") ) );
        return $ifok;
    }
	
	/* 消费记录 */
    function addCoinRecord($insert){
		
        if($insert){
			
            $rs=\PhalApi\DI()->notorm->user_coinrecord->insert($insert);
        }
        
        return $rs;
    }
	
	 /* 获取用户最新余额*/
    function getUserCoin($uid){
        $info =\PhalApi\DI()->notorm->user
				->select('consumption,coin')
				->where('id = ?', $uid)
				->fetchOne();

        return $info;
    }
	
	//获取礼物信息
	function getGiftInfo($id,$fields='*'){
		$giftinfo=\PhalApi\DI()->notorm->gift
					->select($fields)
					->where('id=?',$id)
					->fetchOne();

		if(!empty($giftinfo['gifticon'])){
			$giftinfo['thumb']=$giftinfo['gifticon'];
			$giftinfo['gifticon']=get_upload_path($giftinfo['gifticon']);
		}

		return $giftinfo;
	}

	//获取钻石多语言名称
	function getCoinName(){

		$arr=[];

		$config= \PhalApi\DI()->notorm->option
			->select('option_value')
			->where("option_name='site_info'")
			->fetchOne();

		$configpub=json_decode($config['option_value'],true);

		$arr['name_coin']=$configpub['name_coin'];
		$arr['name_coin_en']=$configpub['name_coin_en'];

		return $arr;

	}

	//生成声网通配Token
	function getShengWangRtcToken($uid,$stream){

		$key=$uid.'_'.$stream.'_swtoken';
		$token=\App\getcaches($key);

		if(!$token){
			require_once API_ROOT.'/../sdk/shengwang/src/RtcTokenBuilder2.php';
			$configpri=\App\getConfigPri();
			$appid=$configpri['sw_app_id'];
			$appCertificate=$configpri['sw_app_certificate'];
			$tokenExpirationInSeconds = 24*60*60; //24小时
			$privilegeExpirationInSeconds = 24*60*60; //24小时

			$channelName=$stream;
			$token = \RtcTokenBuilder2::buildTokenWithUid($appid, $appCertificate, $channelName, $uid, \RtcTokenBuilder2::ROLE_PUBLISHER, $tokenExpirationInSeconds, $privilegeExpirationInSeconds);

			if(!$token){
				$token='';
			}

			if($token){
			
				\App\setcaches($key,$token,24*60*60-10*60);
			}
		}	
		
		return $token;

	}
	
