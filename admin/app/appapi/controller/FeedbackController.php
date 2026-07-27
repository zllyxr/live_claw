<?php
/**
 * 用户反馈
 */
namespace app\appapi\controller;

use app\common\controller\HomeBaseController;
use think\facade\Db;
use cmf\lib\Upload;

class FeedbackController extends HomebaseController{
	
	function index(){
        
        $data = $this->request->param();
        $uid= $data['uid'] ?? '';
        $token= $data['token'] ?? '';
        $model= $data['model'] ?? '';
        $version= $data['version'] ?? '';
        $uid=(int)checkNull($uid);
        $token=checkNull($token);
        $model=checkNull($model);
        $version=checkNull($version);
         
        if( !$uid || !$token || checkToken($uid,$token)==700 ){
			$reason=lang('您的登陆状态失效，请重新登陆！');
			$this->assign('reason', $reason);
			return $this->fetch(':error');
		}
        
        $user=[
            'id'=>$uid,
        ];
        session('user',$user);
        
        $this->assign("uid",$uid);
        $this->assign("token",$token);
        $this->assign("version",$version);
        $this->assign("model",$model);
        return $this->fetch();
	}
	
	function feedbackSave(){
        $data = $this->request->param();
        $uid= $data['uid'] ?? '';
        $token= $data['token'] ?? '';
        $uid=(int)checkNull($uid);
        $token=checkNull($token);

		if( !$uid || !$token || checkToken($uid,$token)==700 ){
            echo json_encode(array("status"=>400,'errormsg'=>lang('您的登陆状态失效，请重新登陆！')));
            return;
		}
        
        $version= $data['version'] ?? '';
        $model= $data['model'] ?? '';
        $content= $data['content'] ?? '';
        $thumb= $data['thumb'] ?? '';
        
        $version=checkNull($version);
        $model=checkNull($model);
        $content=checkNull($content);
        $thumb=checkNull($thumb);
        
        $data2=[
            'uid'=>$uid,
            'version'=>$version,
            'model'=>$model,
            'content'=>$content,
            'thumb'=>set_upload_path($thumb),
            'addtime'=>time(),
        ];

		$result=Db::name("feedback")->insert($data2);
		if($result){
            echo json_encode(array("status"=>0,'msg'=>''));
		}else{
            echo json_encode(array("status"=>400,'errormsg'=>lang('提交失败')));
		}
	
	}	
    
	/* 图片上传 */
	public function upload(){

        $file=$_FILES['file'] ?? '';
        if($file){
            $name=$file['name'];
            $pathinfo = pathinfo($name);
            if(!isset($pathinfo['extension'])){
                $_FILES['file']['name']=$name.'.jpg';
            }
        }

        $configpri=getConfigPri();
        $cloudtype=$configpri['cloudtype'];

        if($cloudtype==1){ //七牛云存储

            $uploader = new Upload();
            $uploader->setFileType('image');
            $res = $uploader->upload();

            if ($res === false) {
                
                echo json_encode(array("ret"=>0,'file'=>'','msg'=>$uploader->getError()));
                return;
            }

            $result=array(
                'url'=>$res['url'],
                'filepath'=>$res['filepath']
            );

            /* $result=[
                'filepath'    => $arrInfo["file_path"],
                "name"        => $arrInfo["filename"],
                'id'          => $strId,
                'preview_url' => cmf_get_root() . '/upload/' . $arrInfo["file_path"],
                'url'         => cmf_get_root() . '/upload/' . $arrInfo["file_path"],
            ]; */

        }else{

            $res=adminUploadFiles($file,$cloudtype);
            if($res===false){
               echo json_encode(array("ret"=>0,'file'=>'','msg'=>lang('文件上传失败')));
               return;
            }

            $result=array(
                "url"=>$res['url'],
                "filepath"=>$res['storage_path']
            );

        }

        echo json_encode(
            array(
                "ret"=>200,
                'data'=>array(
                    "url"=>$result['url'],
                    "file_name"=>$result['filepath']
                ),
                'msg'=>''
            )
        );
        
	}
}
