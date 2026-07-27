<?php
/**
 * 会员认证
 */
namespace app\appapi\controller;

use app\common\controller\HomeBaseController;
use think\facade\Db;
use cmf\lib\Upload;

class AuthController extends HomebaseController {
	
	public function index(){
		$data = $this->request->param();
        $uid= $data['uid'] ?? '';
        $token= $data['token'] ?? '';
        $reset= $data['reset'] ?? '0';
        $uid=(int)checkNull($uid);
        $token=checkNull($token);
        $reset=checkNull($reset);
        
        $checkToken=checkToken($uid,$token);
		/*if($checkToken==700){
			$reason=lang('您的登陆状态失效，请重新登陆！');
			$this->assign('reason', $reason);
			return $this->fetch(':error');
		}*/
        
        $user=[
            'id'=>$uid,
        ];
        session('user',$user);
        
		$this->assign("uid",$uid);
		$this->assign("token",$token);


		if($reset!=1){				 
			$auth=Db::name("user_auth")->where(["uid"=>$uid])->find();
			
			if($auth){
				if($auth['status']==0){
                    return $this->fetch('success');
					
				}else if($auth['status']==1){
					
					$auth['front_view']=get_upload_path($auth['front_view']);
					$auth['back_view']=get_upload_path($auth['back_view']);
					$auth['handset_view']=get_upload_path($auth['handset_view']);
					
					$this->assign("auth",$auth);
                    return $this->fetch('authstep2');
				}else if($auth['status']==2){

                    //语言包
                    $language=$this->language;
                    if($language=='en'){
                        $auth['reason']=$auth['reason_en'];
                    }

					$this->assign("reason",nl2br($auth['reason']));
                    return $this->fetch('error');
				}
			}

		}

		return $this->fetch();
	    
	}

	/* 图片上传 */
	public function upload(){
        
        //file_put_contents(CMF_ROOT.'log/think/appapi/auth/upload_'.date('y-m-d').'.txt',date('Y-m-d H:i:s').' 提交参数信息 files:'.json_encode($_FILES)."\r\n",FILE_APPEND);
        $file=$_FILES['file'] ?? '';
        if($file){
            $name=$file['name'];
            $pathinfo = pathinfo($name);
            if(!isset($pathinfo['extension'])){
                $_FILES['file']['name']=$name.'.jpg';
            }
        }
        $uploader = new Upload();
        $uploader->setFileType('image');
        $result = $uploader->upload();
        // file_put_contents(CMF_ROOT.'log/think/appapi/auth/upload_'.date('y-m-d').'.txt',date('Y-m-d H:i:s').' 提交参数信息 result:'.json_encode($result)."\r\n",FILE_APPEND);
        if ($result === false) {
            echo json_encode(array("ret"=>0,'file'=>'','msg'=>$uploader->getError()));
            return;
        }
        
        /* $result=[
            'filepath'    => $arrInfo["file_path"],
            "name"        => $arrInfo["filename"],
            'id'          => $strId,
            'preview_url' => cmf_get_root() . '/upload/' . $arrInfo["file_path"],
            'url'         => cmf_get_root() . '/upload/' . $arrInfo["file_path"],
        ]; */
        
        echo json_encode(array("ret"=>200,'data'=>array("url"=>$result['url']),'msg'=>''));
        
	}	

	/* 成功 */
	public function succ(){ 
        return $this->fetch('success');
	}

    //获取上传驱动的token
    public function getuploadtoken(){
        
        $uploader = new Upload();
        $result = $uploader->getuploadtoken();

        if ($result === false) {
            echo json_encode(array("ret"=>0,'file'=>'','msg'=>'获取失败'));
            return;
        }
 
        echo json_encode(
            array(
                "ret"=>200,
                "token"=>$result['token'],
                'domain'=>$result['domain'],
                'msg'=>''
            )
        );

        
    }
}