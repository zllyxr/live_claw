<?php
namespace App\Api;

use PhalApi\Api;
/**
 * 上传
 */

class Upload extends Api {

	public function getRules() {
		return array(

		);
	}
	
	/**
	 * 获取上传存储方式与服务端上传地址
	 * @desc App 统一上传到后端，由后端写入 local 或 MinIO
	 * @return int code 操作码，0表示成功
	 * @return string msg 提示信息
	 * @return array info 返回信息
	 */
	public function getCosInfo(){
		$rs=array("code"=>0,"msg"=>"","info"=>array());

		$configpri=\App\getConfigPri();
		$storageType=\App\getStorageTypeByCloudtype($configpri['cloudtype'] ?? '3');

		$storageInfo=array(
			'upload_url'=>\App\getUploadApiUrl(),
			'field'=>'file',
		);

		$localInfo=array(
			'upload_url'=>\App\getUploadApiUrl(),
		);

		$minioInfo=array(
			'endpoint'=>$configpri['minio_endpoint'] ?? '',
			'public_url'=>$configpri['minio_public_url'] ?? '',
			'bucket'=>$configpri['minio_bucket'] ?? '',
			'region'=>$configpri['minio_region'] ?? 'us-east-1',
		);

		$rs['info'][0]['cloudtype']=$storageType;
		$rs['info'][0]['storageInfo']=$storageInfo;
		$rs['info'][0]['localInfo']=$localInfo;
		$rs['info'][0]['minioInfo']=$minioInfo;

        return $rs;

	}

	public function uploadFile(){
		$rs=array("code"=>0,"msg"=>"","info"=>array());

		if(empty($_FILES)){
			$rs['code']=1001;
			$rs['msg']=\PhalApi\T("请选择上传文件");
			return $rs;
		}

		$file=$_FILES['file'] ?? reset($_FILES);
		if(empty($file) || empty($file['tmp_name'])){
			$rs['code']=1002;
			$rs['msg']=\PhalApi\T("上传文件无效");
			return $rs;
		}

		if(!empty($file['error'])){
			$rs['code']=1003;
			$rs['msg']=\PhalApi\T("文件上传失败");
			return $rs;
		}

		$storageType=\App\getStorageType();
		$fileKey=\App\buildUploadFileKey($file['name'] ?? 'upload.dat','appapi');
		$contentType=$file['type'] ?? '';

		if($storageType == 'minio'){
			$uploaded=\App\uploadFileToMinio($file['tmp_name'],$fileKey,$contentType);
			$storagePath='minio_'.$fileKey;
		}else{
			$uploaded=\App\moveUploadFileToLocal($file['tmp_name'],$fileKey);
			$storagePath='local_'.$fileKey;
		}

		if(!$uploaded){
			$rs['code']=1004;
			$rs['msg']=\PhalApi\T("文件保存失败");
			return $rs;
		}

		$url=\App\get_upload_path($storagePath);
		$rs['info'][0]=array(
			'file'=>$storagePath,
			'file_name'=>$storagePath,
			'filepath'=>$storagePath,
			'url'=>$url,
		);

		return $rs;
	}
}
