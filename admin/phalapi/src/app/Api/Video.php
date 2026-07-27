<?php
namespace App\Api;

use PhalApi\Api;
use App\Domain\Video as Domain_Video;

/**
 * 短视频
 */
class Video extends Api {
	public function getRules() {
		return array(
            'getCon' => array(
				'uid' => array('name' => 'uid', 'type' => 'int','desc' => '用户ID'),
			),
            
			'setVideo' => array(
				'uid' => array('name' => 'uid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '用户ID'),
				'token' => array('name' => 'token', 'type' => 'string', 'require' => true, 'desc' => '用户Token'),
				'title' => array('name' => 'title', 'type' => 'string',  'desc' => '标题'),
				'thumb' => array('name' => 'thumb', 'type' => 'string',  'require' => true, 'desc' => '封面图'),
				'href' => array('name' => 'href', 'type' => 'string',  'require' => true, 'desc' => '视频链接'),
				'href_w' => array('name' => 'href_w', 'type' => 'string',   'desc' => '水印视频链接'), 
				'lat' => array('name' => 'lat', 'type' => 'string',  'desc' => '纬度'),
				'lng' => array('name' => 'lng', 'type' => 'string',  'desc' => '经度'),
				'city' => array('name' => 'city', 'type' => 'string',  'desc' => '城市'),
				'music_id' => array('name' => 'music_id', 'type' => 'int','default'=>0, 'desc' => '背景音乐id'),
				'anyway' => array('name' => 'anyway', 'type' => 'string', 'default'=>'1.1','desc' => '横竖屏(封面-高/宽)，大于1表示竖屏,小于1表示横屏'),
			),
            'setComment' => array(
                'uid' => array('name' => 'uid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '用户ID'),
				'token' => array('name' => 'token', 'type' => 'string', 'require' => true, 'desc' => '用户Token'),
				'videoid' => array('name' => 'videoid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '视频ID'),
				'touid' => array('name' => 'touid', 'type' => 'int', 'default'=>0, 'desc' => '回复的评论UID'),
                'commentid' => array('name' => 'commentid', 'type' => 'int',  'default'=>0,  'desc' => '回复的评论commentid'),
                'parentid' => array('name' => 'parentid', 'type' => 'int',  'default'=>0,  'desc' => '回复的评论ID'),
                'content' => array('name' => 'content', 'type' => 'string',  'default'=>'', 'desc' => '内容'),
                'at_info'=>array('name'=>'at_info','type'=>'string','desc'=>'被@的用户json信息'),
            ),
            'addView' => array(
            	'uid' => array('name' => 'uid', 'type' => 'int', 'require' => true, 'desc' => '用户ID'),
            	'token' => array('name' => 'token', 'type' => 'string', 'require' => true, 'desc' => '用户Token'),
                'videoid' => array('name' => 'videoid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '视频ID'),
                'random_str'=>array('name' => 'random_str', 'type' => 'string', 'require' => true, 'desc' => '加密串'),

            ),
            'addLike' => array(
            	'uid' => array('name' => 'uid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '用户ID'),
            	'token' => array('name' => 'token', 'type' => 'string', 'require' => true, 'desc' => '用户Token'),
                'videoid' => array('name' => 'videoid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '视频ID'),
            ),

			
			'addShare' => array(
            	'uid' => array('name' => 'uid', 'type' => 'int',  'desc' => '用户ID'),
                'videoid' => array('name' => 'videoid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '视频ID'),
                'random_str'=>array('name' => 'random_str', 'type' => 'string', 'require' => true, 'desc' => '加密串'),
            ),
			
			'setBlack' => array(
            	'uid' => array('name' => 'uid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '用户ID'),
                'videoid' => array('name' => 'videoid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '视频ID'),
            ),
			
			'addCommentLike' => array(
            	'uid' => array('name' => 'uid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '用户ID'),
            	'token' => array('name' => 'token', 'type' => 'string', 'require' => false, 'desc' => '用户Token'),
                'commentid' => array('name' => 'commentid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '评论/回复 ID'),
            ),
            'getVideoList' => array(
            	'uid' => array('name' => 'uid', 'type' => 'int',  'desc' => '用户ID'),
            	'p' => array('name' => 'p', 'type' => 'int', 'min' => 1, 'default'=>1, 'desc' => '页数'),
            ),
            'getAttentionVideo' => array(
            	'uid' => array('name' => 'uid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '用户ID'),
            	'token' => array('name' => 'token', 'type' => 'string', 'require' => false, 'desc' => '用户Token'),
            	'p' => array('name' => 'p', 'type' => 'int', 'min' => 1, 'default'=>1, 'desc' => '页数'),
            ),
            'getVideo' => array(
            	'uid' => array('name' => 'uid', 'type' => 'int','desc' => '用户ID'),
                'videoid' => array('name' => 'videoid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '视频ID'),
            ),
            'getComments' => array(
                'uid' => array('name' => 'uid', 'type' => 'int','desc' => '用户ID'),
                'videoid' => array('name' => 'videoid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '视频ID'),
                'p' => array('name' => 'p', 'type' => 'int', 'min' => 1, 'default'=>1, 'desc' => '页数'),
            ),
			
			'getReplys' => array(
				'uid' => array('name' => 'uid', 'type' => 'int',  'require' => true, 'desc' => '用户ID'),
                'commentid' => array('name' => 'commentid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '评论ID'),
                'p' => array('name' => 'p', 'type' => 'int', 'min' => 1, 'default'=>1, 'desc' => '页数'),
            ),
			
			'getMyVideo' => array(
                'uid' => array('name' => 'uid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '用户ID'),
				'token' => array('name' => 'token', 'type' => 'string', 'require' => true, 'desc' => '用户Token'),
                'p' => array('name' => 'p', 'type' => 'int', 'min' => 1, 'default'=>1, 'desc' => '页数'),
            ),
            'del' => array(
                'uid' => array('name' => 'uid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '用户ID'),
                'token' => array('name' => 'token', 'type' => 'string', 'min' => 1, 'require' => true, 'desc' => 'token'),
                'videoid' => array('name' => 'videoid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '视频ID'),
            ),
			
			'report' => array(
                'uid' => array('name' => 'uid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '用户ID'),
                'token' => array('name' => 'token', 'type' => 'string', 'require' => true, 'desc' => 'token'),
                'videoid' => array('name' => 'videoid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '视频ID'),
                'content' => array('name' => 'content', 'type' => 'string', 'min' => 1, 'require' => true, 'desc' => '举报内容'),
            ),
			
			'getHomeVideo' => array(
                'uid' => array('name' => 'uid', 'type' => 'int',  'desc' => '用户ID'),
                'touid' => array('name' => 'touid', 'type' => 'int', 'require' => true, 'desc' => '对方ID'),
				'p' => array('name' => 'p', 'type' => 'int', 'min' => 1, 'default'=>1, 'desc' => '页数'),
            ),
			'getCreateNonreusableSignature' => array(
                'imgname' => array('name' => 'imgname', 'type' => 'string', 'desc' => '图片名称'),
                'videoname' => array('name' => 'videoname', 'type' => 'string', 'desc' => '视频名称'),
				'folderimg' => array('name' => 'folderimg', 'type' => 'string','desc' => '图片文件夹'),
				'foldervideo' => array('name' => 'foldervideo', 'type' => 'string', 'desc' => '视频文件夹'),
            ),

            'getNearby'=>array(
            	'uid' => array('name' => 'uid', 'type' => 'int','desc' => '用户ID'),
                'lng' => array('name' => 'lng', 'type' => 'string', 'desc' => '经度值'),
                'lat' => array('name' => 'lat', 'type' => 'string','desc' => '纬度值'),
				'p' => array('name' => 'p', 'type' => 'int', 'default'=>'1' ,'desc' => '页数'),
            ),

            'setConversion'=>array(
            	'uid' => array('name' => 'uid', 'type' => 'int', 'require' => true, 'desc' => '用户ID'),
            	'token' => array('name' => 'token', 'type' => 'string', 'min' => 1, 'require' => true, 'desc' => 'token'),
                'videoid' => array('name' => 'videoid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '视频ID'),
                'random_str'=>array('name' => 'random_str', 'type' => 'string', 'require' => true, 'desc' => '加密串'),
            ),
			
			'getClassVideo'=>array(
                'videoclassid' => array('name' => 'videoclassid', 'type' => 'int', 'default'=>'0' ,'desc' => '视频分类ID'),
				'uid' => array('name' => 'uid', 'type' => 'int', 'require' => true, 'desc' => '用户ID'),
                'p' => array('name' => 'p', 'type' => 'int', 'default'=>'1' ,'desc' => '页数'),
            ),
			
			'startWatchVideo'=>array(
            	'uid' => array('name' => 'uid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '会员ID'),
                'token' => array('name' => 'token', 'require' => true, 'min' => 1, 'desc' => '会员token'),
            ),
			
			'endWatchVideo'=>array(
            	'uid' => array('name' => 'uid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '会员ID'),
                'token' => array('name' => 'token', 'require' => true, 'min' => 1, 'desc' => '会员token'),
            ),
			'delComments' => array(
                'uid' => array('name' => 'uid', 'type' => 'int','desc' => '用户ID'),
				'token' => array('name' => 'token', 'type' => 'string', 'require' => false, 'desc' => '用户Token'),
				'videoid' => array('name' => 'videoid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '视频ID'),
                'commentid' => array('name' => 'commentid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '评论ID'),
                'commentuid' => array('name' => 'commentuid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '评论者用户ID'),
                
            ),

            'getLikeVideos' => array(
                'uid' => array('name' => 'uid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '用户ID'),
				'token' => array('name' => 'token', 'type' => 'string', 'require' => true, 'desc' => '用户Token'),
                'p' => array('name' => 'p', 'type' => 'int', 'min' => 1, 'default'=>1, 'desc' => '页数'),
            ),

            
		);
	}
    
    /**
	 * 获取视频配置
	 * @desc 用于获取视频配置
	 * @return int code 操作码，0表示成功
	 * @return array info 
	 * @return string msg 提示信息
	 */
	public function getCon() { 
		$rs = array('code' => 0, 'msg' => '', 'info' => array());
        $uid=\App\checkNull($this->uid);
        
			$rs['info'][0]=$cdnset;


		return $rs;
	}
	
	/**
	 * 发布短视频
	 * @desc 用于发布短视频
	 * @return int code 操作码，0表示成功
	 * @return array info 
	 * @return string info[0].id 视频记录ID
	 * @return string msg 提示信息
	 */
	public function setVideo() {
		$rs = array('code' => 0, 'msg' => '', 'info' => array());
		
		$uid=\App\checkNull($this->uid);
		$token=\App\checkNull($this->token);
		$title=\App\checkNull($this->title);
		$thumb=\App\checkNull($this->thumb);
		$href=\App\checkNull($this->href);
		$href_w=\App\checkNull($this->href_w);
		$lat=\App\checkNull($this->lat);
		$lng=\App\checkNull($this->lng);
		$city=\App\checkNull($this->city);
		$music_id=\App\checkNull($this->music_id);
		$anyway=\App\checkNull($this->anyway);
        

		
		$checkToken=\App\checkToken($uid,$token);
		if($checkToken==700){
			$rs['code'] = $checkToken;
			$rs['msg'] = \PhalApi\T('您的登陆状态失效，请重新登陆！');
			return $rs;
		}
		
		$sensitivewords=\App\sensitiveField($title);
		if($sensitivewords==1001){
			$rs['code'] = 10011;
			$rs['msg'] = \PhalApi\T('输入非法，请重新输入');
			return $rs;
		}
		
        $thumb_s='';
        if($thumb){
        	
        	$configpri=\App\getConfigPri();
        	$cloudtype=$configpri['cloudtype'];
        	if($cloudtype==1){ //七牛云存储
        		$thumb_s=$thumb.'?imageView2/2/w/200/h/200';
        	}else{
        		$thumb_s=$thumb;
        	}
            
        }
		

		$data=array(
			"uid"=>$uid,
			"title"=>$title,
			"thumb"=>$thumb,
			"thumb_s"=>$thumb_s,
			"href"=>$href,
			"href_w"=>$href_w,
			"lat"=>$lat,
			"lng"=>$lng,
			"city"=>$city,
			"likes"=>0,
			"views"=>1, //因为涉及到推荐排序问题，所以初始值要为1
			"comments"=>0,
			"addtime"=>time(),
				"music_id"=>$music_id,
					"anyway"=>$anyway,
					"type"=>0,

			);
        
        
		
		$domain = new Domain_Video();
		$info = $domain->setVideo($data,$music_id);
		if($info==1007){
			$rs['code']=1007;
			$rs['msg']=\PhalApi\T('视频分类不存在');
			return $rs;
		}else if(!$info){
			$rs['code']=1001;
			$rs['msg']=\PhalApi\T('发布失败');
			return $rs;
		}

		$rs['info'][0]['id']=$info['id'];
		$rs['info'][0]['thumb_s']=\App\get_upload_path($thumb_s);
		$rs['info'][0]['title']=$title;
		return $rs;
	}		
	
   	/**
     * 评论/回复
     * @desc 用于用户评论/回复 别人视频
     * @return int code 操作码，0表示成功
     * @return array info 
     * @return int info[0].isattent 对方是否关注我
     * @return int info[0].u2t 我是否拉黑对方
     * @return int info[0].t2u 对方是否拉黑我
     * @return int info[0].comments 评论总数
     * @return int info[0].replys 回复总数
     * @return string msg 提示信息
     */
	public function setComment() {
        $rs = array('code' => 0, 'msg' => \PhalApi\T('评论成功'), 'info' => array());
		
		$uid=\App\checkNull($this->uid);
		$token=\App\checkNull($this->token);
		$touid=\App\checkNull($this->touid);
		$videoid=\App\checkNull($this->videoid);
		$commentid=\App\checkNull($this->commentid);
		$parentid=\App\checkNull($this->parentid);
		$content=\App\checkNull($this->content);
		$at_info=$this->at_info;

		if(!$at_info){
			$at_info='';
		}
		
		$checkToken=\App\checkToken($uid,$token);
		if($checkToken==700){
			$rs['code'] = $checkToken;
			$rs['msg'] = \PhalApi\T('您的登陆状态失效，请重新登陆！');
			return $rs;
		}
		$sensitivewords=\App\sensitiveField($content);
		if($sensitivewords==1001){
			$rs['code'] = 10011;
			$rs['msg'] = \PhalApi\T('输入非法，请重新输入');
			return $rs;
        }
		
		if($commentid==0 && $commentid!=$parentid){
			$commentid=$parentid;
		}
		
		$data=array(
			'uid'=>$uid,
			'touid'=>$touid,
			'videoid'=>$videoid,
			'commentid'=>$commentid,
			'parentid'=>$parentid,
			'content'=>$content,
			'addtime'=>time(),
			'at_info'=>urldecode($at_info)
		);

        $domain = new Domain_Video();
        $result = $domain->setComment($data);
		
        if($result==1001){
            $rs['code']=1001;
            $rs['msg']= \PhalApi\T("评论失败");
            return $rs;
        }
		
		$info=array(
			'isattent'=>'0',
			'u2t'=>'0',
			't2u'=>'0',
			'comments'=>$result['comments'],
			'replys'=>$result['replys'],
		);
		if($touid>0){
			$isattent=\App\isAttention($touid,$uid);
			$u2t = \App\isBlack($uid,$touid);
			$t2u = \App\isBlack($touid,$uid);
			
			$info['isattent']=(string)$isattent;
			$info['u2t']=(string)$u2t;
			$info['t2u']=(string)$t2u;
		}
		
		$rs['info'][0]=$info;
		
		if($parentid!=0){
			 $rs['msg']=\PhalApi\T('回复成功');			
		}
        return $rs;
    }	
	
   	/**
     * 阅读
     * @desc 用于视频阅读数累计
     * @return int code 操作码，0表示成功
     * @return string msg 提示信息
     */
	public function addView() {
        $rs = array('code' => 0, 'msg' => \PhalApi\T('更新视频阅读次数成功'), 'info' => array());

		$uid=\App\checkNull($this->uid);
		$token=\App\checkNull($this->token);
		$videoid=\App\checkNull($this->videoid);
		$random_str=\App\checkNull($this->random_str);

		//md5加密验证字符串
		$str=md5($uid.'-'.$videoid.'-'.'#2hgfk85cm23mk58vncsark');

		if($random_str!==$str){
			$rs['code'] = 1001;
			$rs['msg'] = \PhalApi\T('更新视频阅读次数失败');
			return $rs;
		}

		$checkToken=\App\checkToken($uid,$token);
		if($checkToken==700){
			$rs['code'] = $checkToken;
			$rs['msg'] = \PhalApi\T('您的登陆状态失效，请重新登陆！');
			return $rs;
		}


        $domain = new Domain_Video();
        $res = $domain->addView($uid,$videoid);

        return $rs;
    }	
   	/**
     * 点赞
     * @desc 用于视频点赞数累计
     * @return int code 操作码，0表示成功
     * @return array info 
     * @return string info[0].islike 是否点赞 
     * @return string info[0].likes 点赞数量
     * @return string msg 提示信息
     */
	public function addLike() {
        $rs = array('code' => 0, 'msg' => \PhalApi\T('点赞成功'), 'info' => array());
        $uid=\App\checkNull($this->uid);
        $token=\App\checkNull($this->token);
        $videoid=\App\checkNull($this->videoid);
		$isBan=\App\isBan($uid);
		 if($isBan=='0'){
			$rs['code'] = 700;
			$rs['msg'] = \PhalApi\T('该账号已被禁用');
			return $rs;
		}

		$checkToken=\App\checkToken($uid,$token);
		if($checkToken==700){
			$rs['code'] = $checkToken;
			$rs['msg'] = \PhalApi\T('您的登陆状态失效，请重新登陆！');
			return $rs;
		}
		
        $domain = new Domain_Video();
        $result = $domain->addLike($uid,$videoid);
		if($result==1001){
			$rs['code'] = 1001;
			$rs['msg'] = \PhalApi\T("视频已删除");
			return $rs;
		}else if($result==1002){
			$rs['code'] = 1002;
			$rs['msg'] = \PhalApi\T("不能给自己点赞");
			return $rs;
		}

		$rs['info'][0]=$result;
        return $rs;
    }	

   	

   	/**
     * 视频分享
     * @desc 用于视频分享数累计
     * @return int code 操作码，0表示成功
     * @return array info 
     * @return string info[0].isshare 是否分享
     * @return string info[0].shares 分享数量
     * @return string msg 提示信息
     */
	public function addShare() {
        $rs = array('code' => 0, 'msg' => \PhalApi\T('分享成功'), 'info' => array());

        $uid=\App\checkNull($this->uid);
		$videoid=\App\checkNull($this->videoid);
		$random_str=\App\checkNull($this->random_str);

		//md5加密验证字符串
		$str=md5($uid.'-'.$videoid.'-'.'#2hgfk85cm23mk58vncsark');

		if($random_str!=$str){
			$rs['code'] = 1001;
			$rs['msg'] = \PhalApi\T('视频分享数修改失败');
			return $rs;
		}
		
        $domain = new Domain_Video();
        $rs['info'][0] = $domain->addShare($uid,$videoid);

        return $rs;
    }	

   	/**
     * 拉黑视频
     * @desc 用于拉黑视频
     * @return int code 操作码，0表示成功
     * @return array info 
     * @return string info[0].isblack 是否拉黑
     * @return string msg 提示信息
     */
	public function setBlack() {
        $rs = array('code' => 0, 'msg' => \PhalApi\T('操作成功'), 'info' => array());

        $uid=\App\checkNull($this->uid);
        $videoid=\App\checkNull($this->videoid);

		$isBan=\App\isBan($uid);
		 if($isBan=='0'){
			$rs['code'] = 700;
			$rs['msg'] = \PhalApi\T('该账号已被禁用');
			return $rs;
		}

        $domain = new Domain_Video();
        $rs['info'][0] = $domain->setBlack($uid,$videoid);

        return $rs;
    }	
	
   	/**
     * 评论/回复 点赞
     * @desc 用于评论/回复 点赞数累计
     * @return int code 操作码，0表示成功
     * @return array info 
     * @return string info[0].islike 是否点赞 
     * @return string info[0].likes 点赞数量
     * @return string msg 提示信息
     */
	public function addCommentLike() {
        $rs = array('code' => 0, 'msg' => \PhalApi\T('点赞成功'), 'info' => array());

        $uid=\App\checkNull($this->uid);
        $token=\App\checkNull($this->token);
        $commentid=\App\checkNull($this->commentid);

        $isBan=\App\isBan($uid);
		 if($isBan=='0'){
			$rs['code'] = 700;
			$rs['msg'] = \PhalApi\T('该账号已被禁用');
			return $rs;
		}

		$checkToken=\App\checkToken($uid,$token);
		if($checkToken==700){
			$rs['code'] = $checkToken;
			$rs['msg'] = \PhalApi\T('您的登陆状态失效，请重新登陆！');
			return $rs;
		}

        $domain = new Domain_Video();
         $res= $domain->addCommentLike($uid,$commentid);
         if($res==1001){
         	$rs['code']=1001;
         	$rs['msg']=\PhalApi\T('评论信息不存在');
         	return $rs;
         }
         $rs['info'][0]=$res;

        return $rs;
    }	
	/**
     * 获取app首页视频
     * @desc 用于获取app首页视频
     * @return int code 操作码，0表示成功
     * @return array info 视频列表
     * @return object info[].userinfo 用户信息
     * @return string info[].datetime 格式后的发布时间
     * @return string info[].islike 是否点赞
     * @return string info[].isattent 是否关注
     * @return string info[].thumb_s 封面小图，分享用
     * @return string info[].comments 评论总数
     * @return string info[].likes 点赞数
     * @return string msg 提示信息
     */
	public function getVideoList() {

        $rs = array('code' => 0, 'msg' => '', 'info' => array());

        $uid=\App\checkNull($this->uid);
        $p=\App\checkNull($this->p);

        if($uid>0){
        	$isBan=\App\isBan($this->uid);
			 if($isBan=='0'){
				$rs['code'] = 700;
				$rs['msg'] = \PhalApi\T('该账号已被禁用');
				return $rs;
			}
        }
		

		$key='videoHot_'.$p;

			$info=\App\getcaches($key);

		if(!$info){
			$domain = new Domain_Video();
			$info= $domain->getVideoList($uid,$p);

			if($info==10010){
				$rs['code'] = 0;
				$rs['msg'] =  \PhalApi\T("暂无视频列表");
				return $rs;
			}
			
			\App\setcaches($key,$info,2);
		}

        
		$rs['info'] =$info;
        return $rs;
    }	
	/**
     * 获取关注视频
     * @desc 用于获取关注视频
     * @return int code 操作码，0表示成功
     * @return array info 视频列表
     * @return array info[].userinfo 用户信息
     * @return string info[].datetime 格式后的发布时间
	 * @return string info[].islike 是否点赞 
	 * @return string info[].comments 评论总数
     * @return string info[].likes 点赞数
     * @return string msg 提示信息
     */
	public function getAttentionVideo() {
        $rs = array('code' => 0, 'msg' => '', 'info' => array());

        $uid=\App\checkNull($this->uid);
		$token=\App\checkNull($this->token);
		$p=\App\checkNull($this->p);
		$checkToken=\App\checkToken($uid,$token);
		if($checkToken==700){
			$rs['code'] = $checkToken;
			$rs['msg'] = \PhalApi\T('您的登陆状态失效，请重新登陆！');
			return $rs;
		}

		$key='attention_vidseoLists_'.$uid.'_'.$p;
        $info=\App\getcaches($key);

        if(!$info){
        	$domain = new Domain_Video();
        	$info=$domain->getAttentionVideo($uid,$p);
        	if($info==0){
        		 $rs['code']=0;
                $rs['msg']= \PhalApi\T("暂无视频列表");
                return $rs;
        	}
        }
        
        $rs['info'] = $info;

        return $rs;
    }		
	/**
     * 视频详情
     * @desc 用于获取视频详情
     * @return int code 操作码，0表示成功，1000表示视频不存在
     * @return array info[0] 视频详情
     * @return object info[0].userinfo 用户信息
     * @return string info[0].datetime 格式后的时间差
     * @return string info[0].isattent 是否关注
     * @return string info[0].likes 点赞数
     * @return string info[0].comments 评论数
     * @return string info[0].views 阅读数
     * @return string info[0].shares 分享数量
     * @return string info[0].islike 是否点赞
     * @return string msg 提示信息
     */
	public function getVideo() {
        $rs = array('code' => 0, 'msg' => '', 'info' => array());

        $uid=\App\checkNull($this->uid);
        $videoid=\App\checkNull($this->videoid);

        $domain = new Domain_Video();
        $result = $domain->getVideo($uid,$videoid);
		if($result==1000){
			$rs['code'] = 1000;
			$rs['msg'] =  \PhalApi\T("视频已删除");
			return $rs;
			
		}
		$rs['info'][0]=$result;

        return $rs;
    }
	/**
     * 视频评论列表
     * @desc 用于获取视频评论列表
     * @return int code 操作码，0表示成功
     * @return array info 
     * @return string info[0].comments 评论总数
     * @return array info[0].commentlist 评论列表
     * @return object info[0].commentlist[].userinfo 用户信息
	 * @return string info[0].commentlist[].datetime 格式后的时间差
	 * @return string info[0].commentlist[].replys 回复总数
	 * @return string info[0].commentlist[].likes 点赞数
	 * @return string info[0].commentlist[].islike 是否点赞
	 * @return array info[0].commentlist[].replylist 回复列表
     * @return string msg 提示信息
     */
	public function getComments() {
        $rs = array('code' => 0, 'msg' => '', 'info' => array());

        $uid=\App\checkNull($this->uid);
        $videoid=\App\checkNull($this->videoid);
        $p=\App\checkNull($this->p);

		$isBan=\App\isBan($uid);
		 if($isBan=='0'){
			$rs['code'] = 700;
			$rs['msg'] = \PhalApi\T('该账号已被禁用');
			return $rs;
		}

        $domain = new Domain_Video();
        $rs['info'][0] = $domain->getComments($uid,$videoid,$p);

        return $rs;
    }	
	
	/**
     * 回复列表
     * @desc 用于获取视频评论列表
     * @return int code 操作码，0表示成功
     * @return array info 评论列表
     * @return object info[].userinfo 用户信息
	 * @return string info[].datetime 格式后的时间差
	 * @return object info[].tocommentinfo 回复的评论的信息
	 * @return object info[].tocommentinfo.content 评论内容
	 * @return string info[].likes 点赞数
	 * @return string info[].islike 是否点赞
     * @return string msg 提示信息
     */
	public function getReplys() {
        $rs = array('code' => 0, 'msg' => '', 'info' => array());

        $uid=\App\checkNull($this->uid);
        $commentid=\App\checkNull($this->commentid);
        $p=\App\checkNull($this->p);

		$isBan=\App\isBan($uid);
		 if($isBan=='0'){
			$rs['code'] = 700;
			$rs['msg'] = \PhalApi\T('该账号已被禁用');
			return $rs;
		}

        $domain = new Domain_Video();
        $rs['info'] = $domain->getReplys($uid,$commentid,$p);

        return $rs;
    }	
	
	
	/**
     * 我的视频(弃用)
     * @desc 用于获取我发布的视频
     * @return int code 操作码，0表示成功
     * @return array info 视频列表
     * @return array info[].userinfo 用户信息
     * @return string info[].datetime 格式后的发布时间
     * @return string info[].islike 是否点赞
     * @return string msg 提示信息
     */
	public function getMyVideo() {
        $rs = array('code' => 0, 'msg' => '', 'info' => array());
		
		$uid=\App\checkNull($this->uid);
		$token=\App\checkNull($this->token);
		$p=\App\checkNull($this->p);
		
		$checkToken=\App\checkToken($uid,$token);
		if($checkToken==700){
			$rs['code'] = $checkToken;
			$rs['msg'] = \PhalApi\T('您的登陆状态失效，请重新登陆！');
			return $rs;
		}

        $domain = new Domain_Video();
        $rs['info'] = $domain->getMyVideo($uid,$p);

        return $rs;
    }	
	
	/**
     * 删除视频
     * @desc 用于删除视频以及相关信息
     * @return int code 操作码，0表示成功
     * @return string msg 提示信息
     */
	public function del() {
        $rs = array('code' => 0, 'msg' => \PhalApi\T('删除成功'), 'info' => array());
		
		$uid=\App\checkNull($this->uid);
		$token=\App\checkNull($this->token);
		$videoid=\App\checkNull($this->videoid);

		$checkToken=\App\checkToken($uid,$token);
		if($checkToken==700){
			$rs['code'] = $checkToken;
			$rs['msg'] = \PhalApi\T('您的登陆状态失效，请重新登陆！');
			return $rs;
		}
		
        $domain = new Domain_Video();
        $info = $domain->del($uid,$videoid);

        return $rs;
    }	

	/**
     * 举报视频
     * @desc 用于举报视频
     * @return int code 操作码，0表示成功
     * @return string msg 提示信息
     */
	public function report() {
        $rs = array('code' => 0, 'msg' => '', 'info' => array());
		
		$uid=\App\checkNull($this->uid);
		$token=\App\checkNull($this->token);
		$videoid=\App\checkNull($this->videoid);
		$content=\App\checkNull($this->content);

		$checkToken=\App\checkToken($uid,$token);
		if($checkToken==700){
			$rs['code'] = $checkToken;
			$rs['msg'] = \PhalApi\T('您的登陆状态失效，请重新登陆！');
			return $rs;
		}
        
		$data=array(
			'uid'=>$uid,
			'videoid'=>$videoid,
			'content'=>$content,
			'addtime'=>time(),
		);
        $domain = new Domain_Video();
        $info = $domain->report($data);
		
		if($info==1000){
			$rs['code'] = 1000;
			$rs['msg'] = \PhalApi\T('视频不存在');
			return $rs;
		}

        return $rs;
    }	


	/**
     * 个人主页视频
     * @desc 用于获取个人主页视频
     * @return int code 操作码，0表示成功
     * @return string msg 提示信息
     */
	public function getHomeVideo() {
        $rs = array('code' => 0, 'msg' => '', 'info' => array());

        $uid=\App\checkNull($this->uid);
        $touid=\App\checkNull($this->touid);
		$p=\App\checkNull($this->p);

		if($uid>0){
			$isBan=\App\isBan($uid);
			if($isBan=='0'){
				$rs['code'] = 700;
				$rs['msg'] = \PhalApi\T('该账号已被禁用');
				return $rs;
			}
		}


        $domain = new Domain_Video();
        $info = $domain->getHomeVideo($uid,$touid,$p);
		
		
		$rs['info']=$info;

        return $rs;
    }	
	
	/* 检测文件后缀 */
	protected function checkExt($filename){
		$config=array("jpg","png","jpeg");
		$ext   =   pathinfo(strip_tags($filename), PATHINFO_EXTENSION);
		 
		return empty($config) ? true : in_array(strtolower($ext), $config);
	}	
	
	/**
     * 获取七牛上传Token
     * @desc 用于获取七牛上传Token
     * @return int code 操作码，0表示成功
     * @return int code 操作码，0表示成功
     * @return string msg 提示信息
     */
	public function getQiniuToken(){
	
	   	$rs = array('code' => 0, 'msg' => '', 'info' =>array());

	   	//获取后台配置的腾讯云存储信息
		//$configPri=\App\getConfigPri();
		 
		//$token = DI()->qiniu->getQiniuToken2($configPri['qiniu_accesskey'],$configPri['qiniu_secretkey'],$configPri['qiniu_bucket']);
		$token = \PhalApi\DI()->qiniu->getQiniuToken();
		$rs['info'][0]['token']=$token ; 
		return $rs; 
		
	}


	/**
	 * 获取附近的视频列表
	 * @desc 用于获取附近的视频列表
	 * @return int code 状态码，0表示成功
	 * @return string msg 提示信息
	 * @return array info 返回信息
	 */
	public function getNearby(){
		$rs = array('code' => 0, 'msg' => '', 'info' => array());
		
		$uid=\App\checkNull($this->uid);
		$lng=\App\checkNull($this->lng);
		$lat=\App\checkNull($this->lat);
		$p=\App\checkNull($this->p);

		if($lng==''){
			return $rs;
		}
		
		if($lat==''){
			return $rs;
		}
		
		if(!$p){
			$p=1;
		}

		$key='videoNearby_'.$lng.'_'.$lat.'_'.$p;

		$info=\App\getcaches($key);

		if(!$info){
			$domain = new Domain_Video();
			$info = $domain->getNearby($uid,$lng,$lat,$p);

			if($info==1001){
				return $rs;
			}
			
			\App\setcaches($key,$info,2);
		}

		$rs['info'] = $info;
        return $rs;
	}

	/**
     * 获取视频举报分类列表
     * @desc 获取视频举报分类列表
     * @return int code 操作码，0表示成功
     * @return string msg 提示信息
     * @return array info 返回信息
     */
	public function getReportContentlist() {
        $rs = array('code' => 0, 'msg' => '', 'info' => array());

        $domain = new Domain_Video();
        $res = $domain->getReportContentlist();

        if($res==1001){
        	$rs['code']=1001;
        	$rs['msg']=\PhalApi\T('暂无举报分类列表');
        	return $rs;
        }
        $rs['info']=$res;
        return $rs;
    }

    /**
     * 更新视频看完次数
     * @desc 更新视频看完次数
     * @return int code 操作码，0表示成功
     * @return string msg 提示信息
     * @return array info 返回信息
     */
    public function setConversion(){

    	$rs = array('code' => 0, 'msg' => \PhalApi\T('视频完整观看次数更新成功'), 'info' => array());
    	$uid=\App\checkNull($this->uid);
    	$token=\App\checkNull($this->token);
		$videoid=\App\checkNull($this->videoid);
		$random_str=\App\checkNull($this->random_str);

		//md5加密验证字符串
		$str=md5($uid.'-'.$videoid.'-'.'#2hgfk85cm23mk58vncsark');

		if($random_str!==$str){
			$rs['code'] = 1001;
			$rs['msg'] = \PhalApi\T('视频完整观看次数更新失败');
			return $rs;
		}

		
		$checkToken=\App\checkToken($uid,$token);
		if($checkToken==700){
			$rs['code'] = $checkToken;
			$rs['msg'] = \PhalApi\T('您的登陆状态失效，请重新登陆！');
			return $rs;
		}

		

		$domain = new Domain_Video();
        $res = $domain->setConversion($videoid);
        

        return $rs;

    }

	 /**
     * 获取分类下的视频--废弃
     * @desc 获取分类下的视频
     * @return int code 操作码 0表示成功
     * @return string msg 提示信息 
     * @return array info
     **/
    
    public function getClassVideo(){
        $rs = array('code' => 0, 'msg' => '', 'info' => array());

        $videoclassid=\App\checkNull($this->videoclassid);
        $uid=\App\checkNull($this->uid);
        $p=\App\checkNull($this->p);
        
        if(!$videoclassid){
            return $rs;
        }
        $domain=new Domain_Video();
        $res=$domain->getClassVideo($videoclassid,$uid,$p);

        $rs['info']=$res;
        return $rs;
    }
	
	
	
	/**
	 * 用户开始观看视频
	 * @desc 用于每日任务统计用户观看时长
	 * @return int code 状态码，0表示成功
	 * @return string msg 提示信息
	 * @return array info 返回信息
	 */
	public function startWatchVideo(){
		$rs = array('code' => 0, 'msg' => '', 'info' => array());

		$uid=\App\checkNull($this->uid);
        $token=\App\checkNull($this->token);


        $checkToken=\App\checkToken($uid,$token);
		if($checkToken==700){
			$rs['code'] = $checkToken;
			$rs['msg'] = \PhalApi\T('您的登陆状态失效，请重新登陆！');
			return $rs;
		}

		$configpri=\App\getConfigPri();
		$dailytask_switch=$configpri['dailytask_switch'];
		if($dailytask_switch){
			//观看视频计时---每日任务
			$key='watch_video_daily_tasks_'.$uid;
			$time=time();
			\App\setcaches($key,$time);
		}
		return $rs;	
	}
	
	
	/**
	 * 用户结束观看视频
	 * @desc 用于每日任务统计用户观看时长
	 * @return int code 状态码，0表示成功
	 * @return string msg 提示信息
	 * @return array info 返回信息
	 */
	public function endWatchVideo(){
		$rs = array('code' => 0, 'msg' => '', 'info' => array());
		
		$uid=\App\checkNull($this->uid);
        $token=\App\checkNull($this->token);


        $checkToken=\App\checkToken($uid,$token);
		if($checkToken==700){
			$rs['code'] = $checkToken;
			$rs['msg'] = \PhalApi\T('您的登陆状态失效，请重新登陆！');
			return $rs;
		}

	
		/*观看视频计时---每日任务--取出用户起始时间*/
		$key='watch_video_daily_tasks_'.$uid;
		$starttime=\App\getcaches($key);
		if($starttime){ 
			$endtime=time();  //当前时间
			$data=[
				'type'=>'2',
				'starttime'=>$starttime,
				'endtime'=>$endtime,
			];
			\App\dailyTasks($uid,$data);
			//删除当前存入的时间
			\App\delcache($key);

		}

		return $rs;	
	}
	
	/**
     * 删除评论
     * @desc 用于删除评论以及子级评论
     * @return int code 操作码，0表示成功
     * @return string msg 提示信息
     */
	public function delComments() {
        $rs = array('code' => 0, 'msg' => \PhalApi\T('删除成功'), 'info' => array());
		
		$uid=\App\checkNull($this->uid);
		$token=\App\checkNull($this->token);
		$videoid=\App\checkNull($this->videoid);
		$commentid=\App\checkNull($this->commentid);
		$commentuid=\App\checkNull($this->commentuid);


		$checkToken=\App\checkToken($uid,$token);
		if($checkToken==700){
			$rs['code'] = $checkToken;
			$rs['msg'] = \PhalApi\T('您的登陆状态失效，请重新登陆！');
			return $rs;
		}
		
        $domain = new Domain_Video();
        $info = $domain->delComments($uid,$videoid,$commentid,$commentuid);
		
		if($info==1001){
			$rs['code'] = 1001;
			$rs['msg'] = \PhalApi\T('视频信息错误,请稍后操作');
		}else if($info==1002){
			$rs['code'] = 1002;
			$rs['msg'] = \PhalApi\T('您无权进行删除操作');
		}

        return $rs;
    }


    /**
     * 获取我喜欢的视频列表
     * @desc 用于获取我喜欢的视频列表
     * @return int code 操作码，0表示成功
     * @return array info 视频列表
     * @return array info[].userinfo 用户信息
     * @return string info[].datetime 格式后的发布时间
     * @return string info[].islike 是否点赞
     * @return string msg 提示信息
     */
	public function getLikeVideos() {
        $rs = array('code' => 0, 'msg' => '', 'info' => array());
		
		$uid=\App\checkNull($this->uid);
		$token=\App\checkNull($this->token);
		$p=\App\checkNull($this->p);
		
		$checkToken=\App\checkToken($uid,$token);
		if($checkToken==700){
			$rs['code'] = $checkToken;
			$rs['msg'] = \PhalApi\T('您的登陆状态失效，请重新登陆！');
			return $rs;
		}

        $domain = new Domain_Video();
        $rs['info'] = $domain->getLikeVideos($uid,$p);

        return $rs;
    }
}
