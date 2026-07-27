<?php

/**
 * 虚拟直播
 */
namespace app\admin\controller;

use cmf\controller\AdminBaseController;
use think\facade\Db;

class VirtualliveController extends AdminBaseController {
    protected $resolvedPageCache=[];

    protected function getLiveClass(){
        return Db::name("live_class")->order('list_order asc, id desc')->column('id,name');
    }

    protected function getTypes($k=''){
        $type=[
            '0'=>'普通房间',
            '1'=>'密码房间',
            '2'=>'门票房间',
            '3'=>'计时房间',
        ];

        if($k===''){
            return $type;
        }
        return $type[$k] ?? '';
    }

    protected function getSourceTypes($k=''){
        $type=[
            3=>'抖音房间号（前端直拉）',
            1=>'视频素材推流',
            2=>'OBS 外部推流',
        ];

        if($k===''){
            return $type;
        }
        return $type[(int)$k] ?? '未知';
    }

    protected function getStatusText($status){
        $map=[
            0=>'待开始',
            1=>'直播中',
            2=>'已停止',
            3=>'失败',
        ];
        return $map[(int)$status] ?? '未知';
    }

    protected function getLocalHost(){
        $configured=trim((string)getenv('PUBLIC_LIVE_HOST'));
        if($configured!==''){
            return $configured;
        }
        $host=parse_url(get_host(), PHP_URL_HOST);
        if(!$host){
            $host=$this->request->host(true);
        }
        if(!$host || $host=='127.0.0.1' || $host=='localhost'){
            $host='192.168.31.187';
        }
        return $host;
    }

    protected function getVirtualHttpPullUrl($stream){
        $config=getConfigPri();
        $base=trim($config['virtual_live_pull_url'] ?? '');
        if($base!==''){
            return rtrim($base,'/').'/'.$stream.'.flv';
        }

        $generated=PrivateKeyA('http',$stream.'.flv',0);
        $generatedHost=parse_url($generated,PHP_URL_HOST);
        if($generated!=='' && $generatedHost){
            return $generated;
        }

        return 'http://'.$this->getLocalHost().':18080/live/'.$stream.'.m3u8';
    }

    protected function getPushUrl($stream){
        $config=getConfigPri();
        $pushDomain=trim($config['virtual_live_push_url'] ?? '');
        if($pushDomain!==''){
            $pushDomain=preg_replace('#^rtmp://#i','',$pushDomain);
            return 'rtmp://'.rtrim($pushDomain,'/').'/live/'.$stream;
        }

        $generated=PrivateKeyA('rtmp',$stream,1);
        if($this->isValidRtmpUrl($generated)){
            return $generated;
        }

        return 'rtmp://srs/live/'.$stream;
    }

    protected function isValidRtmpUrl($url){
        return is_string($url) && preg_match('#^rtmp://[^/]+/.+#i',$url);
    }

    protected function ensureFfmpeg(){
        $path=trim((string)shell_exec('command -v ffmpeg 2>/dev/null'));
        return $path !== '';
    }

    protected function ensurePython(){
        $path=trim((string)shell_exec('command -v python3 2>/dev/null'));
        return $path !== '';
    }

    protected function getPagePullScript(){
        return CMF_ROOT.'scripts/live.py';
    }

    protected function getRoomDiscoveryScript(){
        return CMF_ROOT.'scripts/discover_douyin_rooms.py';
    }

    protected function getPageFleetLimit(){
        $limit=(int)getenv('VIRTUAL_LIVE_MAX_ACTIVE');
        if($limit<=0){
            $limit=$this->isDirectDeliveryMode() ? 300 : 8;
        }
        return max(1,min($this->isDirectDeliveryMode() ? 1000 : 24,$limit));
    }

    protected function isDirectDeliveryMode(){
        // 抖音 PAGE 永远只做地址解析，禁止通过配置退回服务器媒体转推。
        return true;
    }

    protected function getVirtualAvatarFiles(){
        $root=WEB_ROOT.'upload/virtual_avatar/girls';
        if(!is_dir($root)){
            return [];
        }

        $files=[];
        $iterator=new \RecursiveIteratorIterator(
            new \RecursiveDirectoryIterator($root,\FilesystemIterator::SKIP_DOTS)
        );
        foreach($iterator as $file){
            if(!$file->isFile()){
                continue;
            }
            $ext=strtolower($file->getExtension());
            if(!in_array($ext,['jpg','jpeg','png','webp'],true)){
                continue;
            }
            $relative=str_replace('\\','/',substr($file->getPathname(),strlen(WEB_ROOT.'upload/')));
            $files[]='local_'.$relative;
        }
        sort($files,SORT_NATURAL);
        return $files;
    }

    protected function buildVirtualNickname($index){
        $first=[
            '晚晚','小鹿','橙子','念念','星禾','夏沫','青柠','桃桃','浅月',
            '南栀','可可','小满','安然','微凉','糖糖','鹿鸣','初晴','暖暖',
            '七七','果果','清欢','若溪','云朵','小葵','米粒','阿梨','柚子',
            '团子','月牙','芊芊',
        ];
        $last=[
            '日记','慢生活','的晚风','在发光','小屋','随手拍','有点甜','看世界',
            '频道','碎碎念','的夏天','来啦','在路上','的小确幸','今日份','不熬夜',
            '分享站','的星光','好心情','生活志','小宇宙','的清晨','听风','放映室',
            '的日常','记录簿','漫游记','聊天室','的角落','轻松一刻',
        ];
        $index=max(0,(int)$index);
        return $first[$index % count($first)].$last[(int)floor($index / count($first)) % count($last)];
    }

    protected function repairVirtualUserProfiles(){
        $avatars=$this->getVirtualAvatarFiles();
        if(!$avatars){
            return 0;
        }
        $used=Db::name('user')
            ->where(['is_virtual'=>1])
            ->whereNotIn('avatar',['','/default.jpg','/default_thumb.jpg'])
            ->column('avatar');
        $usedMap=array_fill_keys($used,true);
        $available=array_values(array_filter($avatars,function($avatar) use ($usedMap){
            return !isset($usedMap[$avatar]);
        }));
        if(!$available){
            return 0;
        }

        $users=Db::name('user')
            ->field('id,user_nickname,avatar')
            ->where(['user_type'=>2,'is_virtual'=>1])
            ->where(function($query){
                $query->where('avatar','=','')
                    ->whereOr('avatar','=','/default.jpg')
                    ->whereOr('avatar','=','/default_thumb.jpg');
            })
            ->order('id asc')
            ->limit(count($available))
            ->select()
            ->toArray();
        $updated=0;
        foreach($users as $index=>$user){
            $data=[
                'sex'=>2,
                'avatar'=>$available[$index],
                'avatar_thumb'=>$available[$index],
            ];
            $nickname=trim((string)$user['user_nickname']);
            if($nickname==='' || strpos($nickname,'虚拟')!==false){
                $data['user_nickname']=$this->buildVirtualNickname((int)$user['id']);
            }
            if(Db::name('user')->where(['id'=>$user['id']])->update($data)!==false){
                $updated++;
            }
        }
        $placeholderNames=Db::name('user')
            ->field('id,user_nickname')
            ->where(['user_type'=>2,'is_virtual'=>1])
            ->where(function($query){
                $query->where('user_nickname','=','')
                    ->whereOr('user_nickname','like','%虚拟%')
                    ->whereOr('user_nickname','like','%测试%');
            })
            ->select()
            ->toArray();
        foreach($placeholderNames as $user){
            $nickname=$this->buildVirtualNickname((int)$user['id']);
            if(Db::name('user')->where(['id'=>$user['id']])->update([
                'sex'=>2,
                'user_nickname'=>$nickname,
            ])!==false){
                delcache('userinfo_'.$user['id']);
                $updated++;
            }
        }
        return $updated;
    }

    protected function createVirtualUsers($count,$nickPrefix=''){
        $this->repairVirtualUserProfiles();
        $count=max(0,min(1000,(int)$count));
        if($count===0){
            return ['ok'=>true,'created'=>0,'available'=>count($this->getVirtualAvatarFiles())];
        }

        $avatars=$this->getVirtualAvatarFiles();
        if(!$avatars){
            return ['ok'=>false,'created'=>0,'msg'=>'没有找到女生头像素材，请先同步到 public/upload/virtual_avatar/girls'];
        }

        $usedAvatars=Db::name('user')
            ->where(['is_virtual'=>1])
            ->where('avatar','<>','')
            ->column('avatar');
        $usedMap=array_fill_keys($usedAvatars,true);
        $available=array_values(array_filter($avatars,function($avatar) use ($usedMap){
            return !isset($usedMap[$avatar]);
        }));
        if(!$available){
            return ['ok'=>false,'created'=>0,'msg'=>'头像素材已经全部分配'];
        }

        $count=min($count,count($available));
        $existingLogins=Db::name('user')
            ->where('user_login','like','virtual_live_%')
            ->column('user_login');
        $nextSequence=1;
        foreach($existingLogins as $login){
            if(preg_match('/^virtual_live_(\d+)$/',$login,$match)){
                $nextSequence=max($nextSequence,(int)$match[1]+1);
            }
        }

        $created=0;
        $now=time();
        $password=cmf_password(bin2hex(random_bytes(24)));
        $nickPrefix=trim((string)$nickPrefix);
        Db::startTrans();
        try{
            for($i=0;$i<$count;$i++){
                $sequence=$nextSequence+$i;
                $login='virtual_live_'.str_pad((string)$sequence,6,'0',STR_PAD_LEFT);
                $nickname=$nickPrefix!=='' ?
                    $nickPrefix.str_pad((string)$sequence,3,'0',STR_PAD_LEFT) :
                    $this->buildVirtualNickname($sequence-1);
                if(Db::name('user')->where(['user_nickname'=>$nickname])->value('id')){
                    $nickname.='·'.$sequence;
                }
                $avatar=$available[$i];
                $data=[
                    'user_type'=>2,
                    'sex'=>2,
                    'birthday'=>0,
                    'score'=>0,
                    'coin'=>0,
                    'create_time'=>$now,
                    'user_status'=>1,
                    'user_login'=>$login,
                    'mobile'=>'',
                    'user_pass'=>$password,
                    'user_nickname'=>$nickname,
                    'avatar'=>$avatar,
                    'avatar_thumb'=>$avatar,
                    'signature'=>'分享真实、有趣的直播内容',
                    'province'=>'',
                    'city'=>'',
                    'login_type'=>'virtual',
                    'iszombie'=>0,
                    'iszombiep'=>0,
                    'issuper'=>0,
                    'ishot'=>1,
                    'isrecommend'=>0,
                    'source'=>'virtual_live_avatar_pool',
                    'country_code'=>86,
                    'is_ad'=>0,
                    'is_virtual'=>1,
                ];
                if(Db::name('user')->insertGetId($data)){
                    $created++;
                }
            }
            Db::commit();
        }catch(\Throwable $e){
            Db::rollback();
            return ['ok'=>false,'created'=>0,'msg'=>'生成虚拟用户失败：'.$e->getMessage()];
        }

        return [
            'ok'=>true,
            'created'=>$created,
            'available'=>count($available)-$created,
        ];
    }

    protected function taskNeedsProcess($task){
        $sourceType=(int)($task['source_type'] ?? 1);
        return $sourceType===1 || ($sourceType===3 && !$this->isDirectDeliveryMode());
    }

    protected function isValidPageRoomUrl($url){
        if(!is_string($url)){
            return false;
        }

        $url=trim($url);
        if(!preg_match('#^https?://#i',$url)){
            return false;
        }

        $parts=parse_url($url);
        if(!$parts || empty($parts['scheme']) || empty($parts['host'])){
            return false;
        }

        $host=strtolower($parts['host']);
        $path=trim($parts['path'] ?? '', '/');
        return $host==='live.douyin.com' && (
            $path===''
            || preg_match('#^\d{5,}$#',$path)
            || preg_match('#^(category|categorynew)/[A-Za-z0-9_-]+$#i',$path)
            || strtolower($path)==='hot_live'
        );
    }

    protected function roomIdFromSourcePage($page){
        $parts=parse_url(trim((string)$page));
        if(!$parts){
            return '';
        }
        $query=[];
        parse_str((string)($parts['query'] ?? ''),$query);
        $roomId=trim((string)($query['preferred_room_id'] ?? ''));
        if($roomId!==''){
            return preg_match('/^\d{5,}$/',$roomId) ? $roomId : '';
        }
        $path=trim((string)($parts['path'] ?? ''),'/');
        return preg_match('/^\d{5,}$/',$path) ? $path : '';
    }

    protected function normalizeManualRoomPage($value){
        $value=trim((string)$value);
        if(preg_match('/^\d{5,}$/',$value)){
            return [
                'ok'=>true,
                'room_id'=>$value,
                'room_page'=>'https://live.douyin.com/'.$value,
            ];
        }
        if(!$this->isValidPageRoomUrl($value)){
            return ['ok'=>false,'msg'=>'请填写抖音房间号或具体直播间链接'];
        }
        $roomId=$this->roomIdFromSourcePage($value);
        if($roomId===''){
            return ['ok'=>false,'msg'=>'人工接入只支持具体房间号，不支持首页或分类页'];
        }
        return [
            'ok'=>true,
            'room_id'=>$roomId,
            'room_page'=>'https://live.douyin.com/'.$roomId,
        ];
    }

    protected function resolveExactDouyinRoom($page,$verifyManifest=true){
        $normalized=$this->normalizeManualRoomPage($page);
        if(empty($normalized['ok'])){
            return $normalized;
        }
        $roomId=$normalized['room_id'];
        $roomPage=$normalized['room_page'];
        $cacheKey=$roomPage.'|'.($verifyManifest ? '1' : '0');
        if(isset($this->resolvedPageCache[$cacheKey])){
            return $this->resolvedPageCache[$cacheKey];
        }
        if(!$this->ensurePython()){
            return ['ok'=>false,'msg'=>'服务器未安装 Python，无法解析抖音房间'];
        }
        $script=CMF_ROOT.'scripts/resolve_live_source.py';
        if(!is_file($script)){
            return ['ok'=>false,'msg'=>'抖音房间解析脚本不存在'];
        }
        $command='timeout 70s env '
            .$this->shellQuote('PYTHONUNBUFFERED=1').' '
            .$this->shellQuote('RANDOM_ROOM=0').' '
            .$this->shellQuote('ROOM_RETRY=1').' '
            .$this->shellQuote('DOUYIN_FETCH_INTERVAL_MS=1200').' '
            .$this->shellQuote('DOUYIN_444_COOLDOWN=45').' '
            .'python3 '.$this->shellQuote($script)
            .' --page '.$this->shellQuote($roomPage)
            .' --max-height 720 --pull-format hls 2>/dev/null';
        $output=[];
        $code=0;
        exec($command,$output,$code);
        $payload=json_decode(trim(implode("\n",$output)),true);
        if(
            $code!==0
            || !is_array($payload)
            || empty($payload['ok'])
            || empty($payload['url'])
        ){
            return ['ok'=>false,'msg'=>'该房间当前未开播或暂时无法解析'];
        }
        $resolvedRoomId=trim((string)($payload['room_id'] ?? ''));
        if($resolvedRoomId!==$roomId){
            return ['ok'=>false,'msg'=>'解析结果与填写的房间号不一致，已拒绝接入'];
        }
        $mediaUrl=preg_replace('#^http://#i','https://',trim((string)$payload['url']));
        if(!filter_var($mediaUrl,FILTER_VALIDATE_URL) || stripos($mediaUrl,'.m3u8')===false){
            return ['ok'=>false,'msg'=>'没有取得可供前端播放的 HLS 地址'];
        }
        if($verifyManifest && !$this->verifyHlsManifest($mediaUrl)){
            return ['ok'=>false,'msg'=>'房间已解析，但当前视频信号不可用'];
        }
        $result=[
            'ok'=>true,
            'room_id'=>$roomId,
            'room_page'=>$roomPage,
            'nickname'=>trim((string)($payload['nickname'] ?? '')),
            'title'=>trim((string)($payload['title'] ?? '')),
            'url'=>$mediaUrl,
            'format'=>'hls',
            'height'=>(int)($payload['height'] ?? 0),
            'resolution'=>trim((string)($payload['resolution'] ?? '')),
            'cache_seconds'=>30,
            'delivery'=>'direct',
        ];
        $this->resolvedPageCache[$cacheKey]=$result;
        return $result;
    }

    protected function verifyHlsManifest($url){
        if(!function_exists('curl_init')){
            return true;
        }
        $curl=curl_init($url);
        curl_setopt_array($curl,[
            CURLOPT_RETURNTRANSFER=>true,
            CURLOPT_FOLLOWLOCATION=>true,
            CURLOPT_CONNECTTIMEOUT=>6,
            CURLOPT_TIMEOUT=>12,
            CURLOPT_RANGE=>'0-8191',
            CURLOPT_HTTPHEADER=>[
                'User-Agent: Mozilla/5.0',
                'Referer: https://live.douyin.com/',
                'Accept: application/vnd.apple.mpegurl,application/x-mpegURL,*/*',
            ],
        ]);
        $body=curl_exec($curl);
        $status=(int)curl_getinfo($curl,CURLINFO_HTTP_CODE);
        $error=curl_errno($curl);
        curl_close($curl);
        return !$error
            && $status>=200
            && $status<400
            && is_string($body)
            && strpos($body,'#EXTM3U')!==false;
    }

    protected function approveManualRoom($resolved){
        $roomId=trim((string)($resolved['room_id'] ?? ''));
        if($roomId===''){
            return false;
        }
        $now=time();
        $existing=Db::name('live_source_room_pool')
            ->where(['provider'=>'douyin','room_id'=>$roomId])
            ->find();
        $data=[
            'provider'=>'douyin',
            'room_id'=>$roomId,
            'room_page'=>'https://live.douyin.com/'.$roomId,
            'category_page'=>'https://live.douyin.com/'.$roomId,
            'gender_tag'=>'female',
            'verify_status'=>1,
            'verify_source'=>'manual_admin_room',
            'confidence'=>1,
            'status'=>1,
            'last_seen_at'=>$now,
            'last_verified_at'=>$now,
            'update_time'=>$now,
        ];
        foreach(['nickname','title'] as $field){
            $value=trim((string)($resolved[$field] ?? ''));
            if($value!==''){
                $data[$field]=$value;
            }
        }
        if($existing){
            return Db::name('live_source_room_pool')
                ->where(['id'=>$existing['id']])
                ->update($data)!==false;
        }
        $data+=[
            'nickname'=>'',
            'uniq_id'=>'',
            'title'=>'',
            'avatar'=>'',
            'cover'=>'',
            'create_time'=>$now,
        ];
        return Db::name('live_source_room_pool')->insert($data)!==false;
    }

    protected function approvedFemaleRoomMap(array $roomIds=[]){
        $query=Db::name('live_source_room_pool')
            ->where([
                'provider'=>'douyin',
                'gender_tag'=>'female',
                'verify_status'=>1,
                'status'=>1,
            ]);
        $roomIds=array_values(array_unique(array_filter(array_map('strval',$roomIds))));
        if($roomIds){
            $query->whereIn('room_id',$roomIds);
        }
        $ids=$query->column('room_id');
        return array_fill_keys(array_map('strval',$ids),true);
    }

    protected function normalizeRoomGenderState(){
        return Db::name('live_source_room_pool')
            ->where(['provider'=>'douyin'])
            ->where(function($query){
                $query->where('gender_tag','<>','female')
                    ->whereOr('verify_status','<>',1);
            })
            ->where('verify_status','<>',2)
            ->update([
                'gender_tag'=>'unknown',
                'verify_status'=>0,
                'verify_source'=>'',
                'confidence'=>0,
                'last_verified_at'=>0,
                'update_time'=>time(),
            ]);
    }

    protected function isApprovedFemaleSource($page){
        $roomId=$this->roomIdFromSourcePage($page);
        if($roomId===''){
            return false;
        }
        return isset($this->approvedFemaleRoomMap([$roomId])[$roomId]);
    }

    protected function discoverPageRooms($page,$count){
        $page=trim((string)$page);
        $count=max(1,min(1000,(int)$count));
        if(!$this->isValidPageRoomUrl($page)){
            return ['ok'=>false,'rooms'=>[],'msg'=>'抖音页面地址不正确'];
        }
        if(!$this->ensurePython()){
            return ['ok'=>false,'rooms'=>[],'msg'=>'服务器未安装 Python'];
        }

        $script=$this->getRoomDiscoveryScript();
        if(!file_exists($script)){
            return ['ok'=>false,'rooms'=>[],'msg'=>'在线房间发现脚本不存在'];
        }

        $cmd='timeout 240s env '
            .$this->shellQuote('PYTHONUNBUFFERED=1').' '
            .$this->shellQuote('RANDOM_ROOM=0').' '
            .$this->shellQuote('ROOM_RETRY=1').' '
            .'python3 '.$this->shellQuote($script)
            .' --page '.$this->shellQuote($page)
            .' --count '.$this->shellQuote((string)$count)
            .' --workers 12 --max-height 720 --pull-format hls 2>&1';
        $output=[];
        $code=0;
        exec($cmd,$output,$code);
        $payload=json_decode(trim(implode("\n",$output)),true);
        if(!is_array($payload) || empty($payload['rooms'])){
            $detail=is_array($payload) ? trim((string)($payload['error'] ?? '')) : '';
            return [
                'ok'=>false,
                'rooms'=>[],
                'msg'=>'没有发现可播放的在线房间'.($detail!=='' ? '：'.$detail : ''),
            ];
        }

        $rooms=[];
        foreach($payload['rooms'] as $room){
            $roomPage=trim((string)($room['room_page'] ?? ''));
            $roomId=trim((string)($room['room_id'] ?? ''));
            if($roomId==='' && preg_match('#live\.douyin\.com/(\d{5,})#',$roomPage,$match)){
                $roomId=$match[1];
            }
            if($roomId==='' || !$this->isValidPageRoomUrl($roomPage)){
                continue;
            }
            $rooms[$roomId]=[
                'provider'=>'douyin',
                'room_id'=>$roomId,
                'room_page'=>$roomPage,
                'category_page'=>trim((string)($room['category_page'] ?? '')),
                'nickname'=>trim((string)($room['nickname'] ?? '')),
                'title'=>trim((string)($room['title'] ?? '')),
                'uniq_id'=>trim((string)($room['uniq_id'] ?? '')),
                'avatar'=>trim((string)($room['avatar'] ?? '')),
                'cover'=>trim((string)($room['cover'] ?? '')),
                'format'=>trim((string)($room['format'] ?? '')),
                'height'=>(int)($room['height'] ?? 0),
                'resolution'=>trim((string)($room['resolution'] ?? '')),
            ];
        }
        return ['ok'=>!empty($rooms),'rooms'=>array_values($rooms),'msg'=>''];
    }

    protected function storeDiscoveredRoom($room,$categoryPage=''){
        $roomId=trim((string)($room['room_id'] ?? ''));
        if($roomId===''){
            return;
        }
        $now=time();
        $data=[
            'provider'=>'douyin',
            'room_id'=>$roomId,
            'room_page'=>trim((string)($room['room_page'] ?? '')),
            'nickname'=>trim((string)($room['nickname'] ?? '')),
            'uniq_id'=>trim((string)($room['uniq_id'] ?? '')),
            'title'=>trim((string)($room['title'] ?? '')),
            'category_page'=>trim((string)$categoryPage),
            'avatar'=>trim((string)($room['avatar'] ?? '')),
            'cover'=>trim((string)($room['cover'] ?? '')),
            'status'=>1,
            'last_seen_at'=>$now,
            'update_time'=>$now,
        ];
        $id=Db::name('live_source_room_pool')
            ->where(['provider'=>'douyin','room_id'=>$roomId])
            ->value('id');
        if($id){
            Db::name('live_source_room_pool')->where(['id'=>$id])->update($data);
            return;
        }
        $data['gender_tag']='unknown';
        $data['verify_status']=0;
        $data['verify_source']='';
        $data['confidence']=0;
        $data['last_verified_at']=0;
        $data['create_time']=$now;
        Db::name('live_source_room_pool')->insert($data);
    }

    protected function createPageFleet($requested,$page='https://live.douyin.com/',$discoveredRooms=null){
        $requested=max(1,min(1000,(int)$requested));
        $limit=$this->getPageFleetLimit();
        $running=(int)Db::name('virtual_live_task')
            ->where(['source_type'=>3,'status'=>1])
            ->count();
        $capacity=max(0,$limit-$running);
        if($capacity===0){
            return [
                'ok'=>false,
                'created'=>0,
                'started'=>0,
                'failed'=>0,
                'msg'=>'当前已达到 '.$limit.' 路直播接入上限',
            ];
        }
        $target=min($requested,$capacity);
        $discovered=is_array($discoveredRooms)
            ? ['ok'=>!empty($discoveredRooms),'rooms'=>$discoveredRooms,'msg'=>'']
            : $this->discoverPageRooms($page,min(1000,max($target*2,$target+50)));
        if(!$discovered['ok']){
            return [
                'ok'=>false,
                'created'=>0,
                'started'=>0,
                'failed'=>0,
                'msg'=>$discovered['msg'],
            ];
        }

        foreach($discovered['rooms'] as $room){
            $categoryPage=trim((string)($room['category_page'] ?? '')) ?: $page;
            $this->storeDiscoveredRoom($room,$categoryPage);
        }
        $discoveredRoomIds=array_values(array_filter(array_map(function($room){
            return trim((string)($room['room_id'] ?? ''));
        },$discovered['rooms'])));
        $approvedFemaleMap=$this->approvedFemaleRoomMap($discoveredRoomIds);

        $activePages=Db::name('virtual_live_task')
            ->where(['source_type'=>3,'status'=>1])
            ->column('source_page');
        $activePageMap=array_fill_keys(array_filter($activePages),true);
        $activeRoomMap=[];
        foreach($activePages as $activePage){
            $parts=parse_url((string)$activePage);
            $query=[];
            parse_str((string)($parts['query'] ?? ''),$query);
            $roomId=trim((string)($query['preferred_room_id'] ?? ''));
            if($roomId===''){
                $path=trim((string)($parts['path'] ?? ''),'/');
                if(preg_match('/^\d{5,}$/',$path)){
                    $roomId=$path;
                }
            }
            if($roomId!==''){
                $activeRoomMap[$roomId]=true;
            }
        }
        $rooms=array_values(array_filter($discovered['rooms'],function($room) use ($activePageMap,$activeRoomMap,$approvedFemaleMap){
            $roomId=trim((string)($room['room_id'] ?? ''));
            return !isset($activePageMap[$room['room_page']])
                && $roomId!==''
                && isset($approvedFemaleMap[$roomId])
                && !isset($activeRoomMap[$roomId]);
        }));
        if(!$rooms){
            return [
                'ok'=>false,
                'created'=>0,
                'started'=>0,
                'failed'=>0,
                'msg'=>'没有可接入的已审核女性房间；未知性别和男性房间已全部跳过',
            ];
        }

        $activeUids=Db::name('live')->where(['islive'=>1])->column('uid');
        $userQuery=Db::name('user')
            ->field('id,user_nickname,avatar')
            ->where(['user_type'=>2,'user_status'=>1,'is_virtual'=>1])
            ->order('id asc');
        if($activeUids){
            $userQuery->where('id','not in',$activeUids);
        }
        $users=$userQuery->limit(min($target,count($rooms)))->select()->toArray();
        if(!$users){
            return [
                'ok'=>false,
                'created'=>0,
                'started'=>0,
                'failed'=>0,
                'msg'=>'没有空闲的虚拟直播用户',
            ];
        }

        $created=0;
        $started=0;
        $failed=0;
        $errors=[];
        $now=time();
        foreach($users as $index=>$user){
            if(!isset($rooms[$index])){
                break;
            }
            $room=$rooms[$index];
            $categoryPage=trim((string)($room['category_page'] ?? '')) ?: $page;
            $this->storeDiscoveredRoom($room,$categoryPage);
            $sourceName=trim((string)$room['nickname']);
            $roomTitle=trim((string)($room['title'] ?? ''));
            $title=$roomTitle!=='' ? $roomTitle : ($sourceName!=='' ? $sourceName.' · 精选直播' : '正在直播 · 精彩现场');
            $sourcePage=$categoryPage;
            if(strpos($sourcePage,'preferred_room_id=')===false){
                $sourcePage.=(strpos($sourcePage,'?')===false ? '?' : '&')
                    .'preferred_room_id='.rawurlencode((string)$room['room_id']);
            }
            $task=[
                'uid'=>(int)$user['id'],
                'video_id'=>0,
                'source_type'=>3,
                'source_page'=>$sourcePage,
                'title'=>$title,
                'topic'=>'抖音授权内容',
                'thumb'=>(string)$user['avatar'],
                'liveclassid'=>0,
                'type'=>0,
                'type_val'=>'',
                'anyway'=>0,
                'province'=>'',
                'city'=>'',
                'lng'=>'',
                'lat'=>'',
                'loop_play'=>1,
                'status'=>0,
                'addtime'=>$now,
                'updatetime'=>$now,
            ];
            $taskId=Db::name('virtual_live_task')->insertGetId($task);
            if(!$taskId){
                $failed++;
                $errors[]='用户 '.$user['id'].' 创建任务失败';
                continue;
            }
            $created++;
            try{
                $result=$this->startTask($taskId);
            }catch(\Throwable $e){
                $result=['ok'=>false,'msg'=>'启动异常：'.$e->getMessage()];
            }
            if($result['ok']){
                $started++;
                continue;
            }
            $failed++;
            $errors[]='任务 '.$taskId.'：'.$result['msg'];
            Db::name('virtual_live_task')->where(['id'=>$taskId])->update([
                'status'=>3,
                'error_msg'=>$result['msg'],
                'stoptime'=>time(),
                'updatetime'=>time(),
            ]);
        }

        return [
            'ok'=>$started>0,
            'created'=>$created,
            'started'=>$started,
            'failed'=>$failed,
            'limit'=>$limit,
            'msg'=>$started>0 ? '已接入 '.$started.' 路直播' : '没有任务启动成功',
            'errors'=>$errors,
        ];
    }

    protected function batchReplacePageTasks($target,$page='https://live.douyin.com/'){
        $target=max(1,min($this->getPageFleetLimit(),(int)$target));
        $this->normalizeRoomGenderState();
        $lock=Db::query("SELECT GET_LOCK('claw_virtual_live_batch_replace',2) AS acquired");
        if((int)($lock[0]['acquired'] ?? 0)!==1){
            return ['ok'=>false,'msg'=>'另一个批量更换任务正在执行，请稍后再试'];
        }

        try{
            $activeTasks=Db::name('virtual_live_task')
                ->where(['source_type'=>3,'status'=>1])
                ->order('id asc')
                ->select()
                ->toArray();
            $activeRoomIds=[];
            foreach($activeTasks as $task){
                $roomId=$this->roomIdFromSourcePage($task['source_page'] ?? '');
                if($roomId!==''){
                    $activeRoomIds[]=$roomId;
                }
            }
            $approvedMap=$this->approvedFemaleRoomMap($activeRoomIds);
            $stoppedUnsafe=0;
            foreach($activeTasks as $task){
                $roomId=$this->roomIdFromSourcePage($task['source_page'] ?? '');
                if($roomId!=='' && isset($approvedMap[$roomId])){
                    continue;
                }
                $result=$this->stopTask((int)$task['id'],2,'未通过女性主播审核，批量下线');
                if($result['ok']){
                    $stoppedUnsafe++;
                }
            }

            $discoverCount=min(1000,max($target*2,$target+100));
            $discovered=$this->discoverPageRooms($page,$discoverCount);
            if(!$discovered['ok']){
                $final=(int)Db::name('virtual_live_task')
                    ->where(['source_type'=>3,'status'=>1])
                    ->count();
                return [
                    'ok'=>false,
                    'msg'=>$discovered['msg'],
                    'target'=>$target,
                    'stopped_unsafe'=>$stoppedUnsafe,
                    'stopped_offline'=>0,
                    'started'=>0,
                    'final'=>$final,
                    'shortage'=>max(0,$target-$final),
                ];
            }

            $onlineMap=[];
            foreach($discovered['rooms'] as $room){
                $roomId=trim((string)($room['room_id'] ?? ''));
                if($roomId===''){
                    continue;
                }
                $onlineMap[$roomId]=true;
                $categoryPage=trim((string)($room['category_page'] ?? '')) ?: $page;
                $this->storeDiscoveredRoom($room,$categoryPage);
            }

            $stoppedOffline=0;
            $remaining=Db::name('virtual_live_task')
                ->where(['source_type'=>3,'status'=>1])
                ->select()
                ->toArray();
            foreach($remaining as $task){
                $roomId=$this->roomIdFromSourcePage($task['source_page'] ?? '');
                if($roomId!=='' && isset($onlineMap[$roomId])){
                    continue;
                }
                $result=$this->stopTask((int)$task['id'],2,'来源房间已离线，批量更换');
                if($result['ok']){
                    $stoppedOffline++;
                }
            }

            $running=(int)Db::name('virtual_live_task')
                ->where(['source_type'=>3,'status'=>1])
                ->count();
            $missing=max(0,$target-$running);
            $created=['ok'=>true,'started'=>0,'failed'=>0,'errors'=>[],'msg'=>'无需补充'];
            if($missing>0){
                $created=$this->createPageFleet($missing,$page,$discovered['rooms']);
            }
            $final=(int)Db::name('virtual_live_task')
                ->where(['source_type'=>3,'status'=>1])
                ->count();
            $eligibleOnline=count(array_intersect_key(
                $onlineMap,
                $this->approvedFemaleRoomMap(array_keys($onlineMap))
            ));
            return [
                'ok'=>true,
                'msg'=>$final>0
                    ? '批量更换完成'
                    : '没有已审核且在线的女性房间，已保持空播',
                'target'=>$target,
                'discovered'=>count($discovered['rooms']),
                'eligible_online'=>$eligibleOnline,
                'stopped_unsafe'=>$stoppedUnsafe,
                'stopped_offline'=>$stoppedOffline,
                'started'=>(int)($created['started'] ?? 0),
                'failed'=>(int)($created['failed'] ?? 0),
                'final'=>$final,
                'shortage'=>max(0,$target-$final),
                'errors'=>$created['errors'] ?? [],
            ];
        }finally{
            Db::query("SELECT RELEASE_LOCK('claw_virtual_live_batch_replace')");
        }
    }

    protected function splitPushUrl($url){
        $parts=parse_url($url);
        if(!$parts || empty($parts['scheme']) || empty($parts['host']) || empty($parts['path'])){
            return ['server'=>$url,'stream_key'=>''];
        }

        $path=$parts['path'];
        $pos=strrpos($path,'/');
        if($pos===false){
            return ['server'=>$url,'stream_key'=>''];
        }

        $server=$parts['scheme'].'://'.$parts['host'];
        if(!empty($parts['port'])){
            $server.=':'.$parts['port'];
        }
        $server.=substr($path,0,$pos);

        $streamKey=substr($path,$pos+1);
        if(isset($parts['query']) && $parts['query']!==''){
            $streamKey.='?'.$parts['query'];
        }

        return ['server'=>$server,'stream_key'=>$streamKey];
    }

    protected function processRunning($pid){
        $pid=(int)$pid;
        if($pid<=0){
            return false;
        }

        if(function_exists('posix_kill')){
            return @posix_kill($pid,0);
        }

        $out=[];
        exec('ps -p '.escapeshellarg((string)$pid).' -o pid=', $out);
        return !empty($out);
    }

    protected function killProcess($pid){
        $pid=(int)$pid;
        if($pid<=0){
            return true;
        }

        if(!$this->processRunning($pid)){
            return true;
        }

        $children=[];
        exec('pgrep -P '.escapeshellarg((string)$pid).' 2>/dev/null',$children);
        foreach($children as $childPid){
            $childPid=(int)$childPid;
            if($childPid>0){
                exec('kill '.escapeshellarg((string)$childPid).' 2>/dev/null');
            }
        }
        exec('kill '.escapeshellarg((string)$pid).' 2>/dev/null');

        for($attempt=0;$attempt<20 && $this->processRunning($pid);$attempt++){
            usleep(100000);
        }

        if($this->processRunning($pid)){
            exec('kill -9 '.escapeshellarg((string)$pid).' 2>/dev/null');
        }
        foreach($children as $childPid){
            $childPid=(int)$childPid;
            if($childPid>0 && $this->processRunning($childPid)){
                exec('kill -9 '.escapeshellarg((string)$childPid).' 2>/dev/null');
            }
        }

        return !$this->processRunning($pid);
    }

    protected function shellQuote($value){
        return escapeshellarg((string)$value);
    }

    protected function buildInputSource($storagePath,$fileUrl=''){
        $storagePath=trim((string)$storagePath);
        if($storagePath===''){
            return trim((string)$fileUrl);
        }

        if(strpos($storagePath,'local_')===0){
            return WEB_ROOT.'upload/'.substr($storagePath,6);
        }

        if(strpos($storagePath,'minio_')===0){
            $key=substr($storagePath,6);
            $config=getConfigPri();
            $endpoint=trim($config['minio_endpoint'] ?? '');
            $bucket=trim($config['minio_bucket'] ?? '');
            if($endpoint!=='' && $bucket!==''){
                $parts=array_map('rawurlencode',explode('/',ltrim($key,'/')));
                return rtrim($endpoint,'/').'/'.$bucket.'/'.implode('/',$parts);
            }
        }

        return trim((string)$fileUrl);
    }

    protected function updateLiveCache(){
        $keys=['getHot_1','getRecommendChatroom'];
        foreach($keys as $key){
            delcache($key);
        }
    }

    protected function closeLiveByTask($task){
        $uid=(int)($task['uid'] ?? 0);
        $stream=(string)($task['stream'] ?? '');
        if($uid<=0 || $stream===''){
            return 1;
        }

        $liveinfo=Db::name("live")
            ->field("uid,showid,starttime,title,province,city,stream,thumb,lng,lat,type,type_val,liveclassid,live_type,deviceinfo,voice_type")
            ->where(['uid'=>$uid,'stream'=>$stream,'islive'=>1])
            ->find();

        Db::name("live")->where(['uid'=>$uid,'stream'=>$stream])->delete();

        if($liveinfo){
            $liveinfo['endtime']=time();
            $liveinfo['time']=date("Y-m-d",$liveinfo['showid']);
            $votes=Db::name("user_voterecord")
                ->where(['uid'=>$uid,'showid'=>$liveinfo['showid']])
                ->sum('total');
            $liveinfo['votes']=$votes ? $votes : 0;
            $liveinfo['nums']=function_exists('zSize') ? zSize('user_'.$stream) : 0;

            hDel("livelist",$uid);
            delcache($uid.'_zombie');
            delcache($uid.'_zombie_uid');
            delcache('attention_'.$uid);
            delcache('user_'.$stream);
            delcache('userinfo_'.$uid);

            Db::name("live_record")->insert($liveinfo);

            if((int)$liveinfo['live_type']===1){
                Db::name("voicelive_mic")->where(["live_stream"=>$stream])->delete();
                Db::name("voicelive_applymic")->where(["stream"=>$stream])->delete();
            }
        }

        $this->updateLiveCache();
        return 1;
    }

    protected function syncRunningTaskStatus(){
        $running=Db::name('virtual_live_task')
            ->where(['status'=>1])
            ->select()
            ->toArray();

        foreach($running as $task){
            if(!$this->taskNeedsProcess($task)){
                continue;
            }
            if(!$this->processRunning($task['pid'])){
                $this->closeLiveByTask($task);
                Db::name('virtual_live_task')->where(['id'=>$task['id']])->update([
                    'status'=>3,
                    'error_msg'=>'ffmpeg 进程已退出',
                    'stoptime'=>time(),
                    'updatetime'=>time(),
                ]);
            }
        }
    }

    protected function createLiveRow($task,$video,$stream,$pullUrl,$isVideo=0,$deviceinfo='virtual_live'){
        $uid=(int)$task['uid'];
        $userinfo=Db::name('user')
            ->field("ishot,isrecommend")
            ->where(["id"=>$uid,"user_type"=>2])
            ->find();
        if(!$userinfo){
            return false;
        }

        $now=time();
        try{
            $liang=getUserLiang($uid);
        }catch(\Throwable $e){
            $liang=[];
        }
        $goodnum=0;
        if(isset($liang['name']) && $liang['name']!='0'){
            $goodnum=$liang['name'];
        }

        $thumb=$task['thumb'] ?? '';
        if($thumb==='' && is_array($video) && !empty($video['cover'])){
            $thumb=$video['cover'];
        }

        $data=[
            "uid"           =>$uid,
            "ishot"         =>$userinfo['ishot'],
            "isrecommend"   =>$userinfo['isrecommend'],
            "showid"        =>$now,
            "starttime"     =>$now,
            "title"         =>$task['title'],
            "province"      =>$task['province'],
            "city"          =>$task['city'] ?: '好像在火星',
            "stream"        =>$stream,
            "thumb"         =>$thumb,
            "pull"          =>$pullUrl,
            "lng"           =>$task['lng'],
            "lat"           =>$task['lat'],
            "type"          =>$task['type'],
            "type_val"      =>$task['type_val'],
            "goodnum"       =>$goodnum,
            "isvideo"       =>$isVideo,
            "islive"        =>1,
            "anyway"        =>$task['anyway'],
            "liveclassid"   =>$task['liveclassid'],
            "deviceinfo"    =>$deviceinfo,
            "hotvotes"      =>0,
            "pkuid"         =>0,
            "pkstream"      =>'',
            "banker_coin"   =>10000000,
            "live_type"     =>0,
            "voice_type"    =>0,
        ];

        $exists=Db::name('live')->where(['uid'=>$uid])->find();
        if($exists){
            return Db::name('live')->where(['uid'=>$uid])->update($data)!==false;
        }

        return Db::name('live')->insert($data)!==false;
    }

    protected function startTask($id){
        $task=Db::name('virtual_live_task')->where(['id'=>$id])->find();
        if(!$task){
            return ['ok'=>false,'msg'=>'任务不存在'];
        }

        $sourceType=(int)($task['source_type'] ?? 1);
        if((int)$task['status']===1){
            if(!$this->taskNeedsProcess($task)){
                if($sourceType===2){
                    return ['ok'=>false,'msg'=>'OBS 推流链接已生成'];
                }
                if($sourceType===3){
                    return ['ok'=>false,'msg'=>'PAGE 直拉直播已启动'];
                }
                return ['ok'=>false,'msg'=>'任务已启动'];
            }
            if($this->processRunning($task['pid'])){
                return ['ok'=>false,'msg'=>'任务已在推流中'];
            }
            $this->closeLiveByTask($task);
            Db::name('virtual_live_task')->where(['id'=>$id])->update([
                'status'=>3,
                'error_msg'=>'进程已退出',
                'stoptime'=>time(),
                'updatetime'=>time(),
            ]);
            $task['status']=3;
        }

        if($sourceType===2){
            return $this->startObsTask($task);
        }
        if($sourceType===3){
            return $this->startPagePullTask($task);
        }
        return $this->startVideoTask($task);
    }

    protected function prepareLiveUrls($task){
        $uid=(int)$task['uid'];
        $live=Db::name('live')->where(['uid'=>$uid,'islive'=>1])->find();
        if($live){
            return ['ok'=>false,'msg'=>'该虚拟用户正在直播，请先停止原直播'];
        }

        $stream=$uid.'_'.time();
        $pushUrl=$this->getPushUrl($stream);
        if(!$this->isValidRtmpUrl($pushUrl)){
            return ['ok'=>false,'msg'=>'推流地址不是有效 RTMP: '.$pushUrl];
        }

        return [
            'ok'=>true,
            'stream'=>$stream,
            'push_url'=>$pushUrl,
            'pull_url'=>$this->getVirtualHttpPullUrl($stream),
        ];
    }

    protected function markLiveStarted($uid){
        hSet("livelist",$uid,1);
        delcache("userinfo_".$uid);
        $this->updateLiveCache();
    }

    protected function getLogFile($id,$suffix=''){
        $logDir=CMF_ROOT.'log/think/admin/virtual_live';
        if(!is_dir($logDir)){
            mkdir($logDir,0777,true);
        }
        $suffix=$suffix ? '_'.$suffix : '';
        return $logDir.'/task_'.$id.$suffix.'_'.date('Ymd_His').'.log';
    }

    protected function getLastLogLines($logFile,$count=8){
        if(!file_exists($logFile)){
            return '';
        }
        return trim(implode('',array_slice(file($logFile),-$count)));
    }

    protected function startVideoTask($task){
        if(!$this->ensureFfmpeg()){
            return ['ok'=>false,'msg'=>'服务器未安装 ffmpeg，请重建 PHP 镜像或安装 ffmpeg'];
        }

        $video=Db::name('virtual_live_video')->where(['id'=>$task['video_id'],'status'=>1])->find();
        if(!$video){
            return ['ok'=>false,'msg'=>'视频素材不存在或已停用'];
        }

        $input=$this->buildInputSource($video['file_path'],$video['file_url']);
        if($input===''){
            return ['ok'=>false,'msg'=>'视频素材地址为空'];
        }

        if(strpos($input,'http')!==0 && !file_exists($input)){
            return ['ok'=>false,'msg'=>'视频素材文件不存在: '.$input];
        }

        $urls=$this->prepareLiveUrls($task);
        if(!$urls['ok']){
            return $urls;
        }

        if(!$this->createLiveRow($task,$video,$urls['stream'],$urls['pull_url'])){
            return ['ok'=>false,'msg'=>'创建直播间失败'];
        }

        $logFile=$this->getLogFile($task['id']);
        $loop=((int)$task['loop_play']===1) ? '-stream_loop -1 ' : '';
        $cmd='nohup ffmpeg -hide_banner -nostdin -re '.$loop.'-i '.$this->shellQuote($input)
            .' -vf '.$this->shellQuote('scale=trunc(iw/2)*2:trunc(ih/2)*2')
            .' -c:v libx264 -preset veryfast -tune zerolatency -pix_fmt yuv420p'
            .' -c:a aac -ar 44100 -b:a 128k -f flv '.$this->shellQuote($urls['push_url'])
            .' > '.$this->shellQuote($logFile).' 2>&1 & echo $!';

        $pid=(int)trim((string)shell_exec($cmd));
        if($pid<=0){
            $this->closeLiveByTask(['uid'=>$task['uid'],'stream'=>$urls['stream']]);
            return ['ok'=>false,'msg'=>'ffmpeg 启动失败'];
        }

        usleep(500000);
        if(!$this->processRunning($pid)){
            $this->closeLiveByTask(['uid'=>$task['uid'],'stream'=>$urls['stream']]);
            $log=$this->getLastLogLines($logFile);
            return ['ok'=>false,'msg'=>'ffmpeg 启动后退出'.($log ? ': '.$log : '')];
        }

        Db::name('virtual_live_task')->where(['id'=>$task['id']])->update([
            'status'=>1,
            'pid'=>$pid,
            'stream'=>$urls['stream'],
            'push_url'=>$urls['push_url'],
            'pull_url'=>$urls['pull_url'],
            'input_url'=>$input,
            'command'=>$cmd,
            'log_file'=>$logFile,
            'error_msg'=>'',
            'starttime'=>time(),
            'stoptime'=>0,
            'updatetime'=>time(),
        ]);

        $this->markLiveStarted((int)$task['uid']);
        return ['ok'=>true,'msg'=>'推流已启动'];
    }

    protected function startObsTask($task){
        $urls=$this->prepareLiveUrls($task);
        if(!$urls['ok']){
            return $urls;
        }

        if(!$this->createLiveRow($task,[],$urls['stream'],$urls['pull_url'])){
            return ['ok'=>false,'msg'=>'创建直播间失败'];
        }

        Db::name('virtual_live_task')->where(['id'=>$task['id']])->update([
            'status'=>1,
            'pid'=>0,
            'stream'=>$urls['stream'],
            'push_url'=>$urls['push_url'],
            'pull_url'=>$urls['pull_url'],
            'input_url'=>'',
            'command'=>'',
            'log_file'=>'',
            'error_msg'=>'',
            'starttime'=>time(),
            'stoptime'=>0,
            'updatetime'=>time(),
        ]);

        $this->markLiveStarted((int)$task['uid']);
        return ['ok'=>true,'msg'=>'OBS 推流链接已生成'];
    }

    protected function startPageDirectTask($task){
        $uid=(int)$task['uid'];
        $live=Db::name('live')->where(['uid'=>$uid,'islive'=>1])->find();
        if($live){
            return ['ok'=>false,'msg'=>'该虚拟用户正在直播，请先停止原直播'];
        }
        $page=trim((string)($task['source_page'] ?? ''));
        $resolved=$this->resolveExactDouyinRoom($page,true);
        if(empty($resolved['ok'])){
            return ['ok'=>false,'msg'=>$resolved['msg'] ?? '抖音房间当前无法解析'];
        }
        if(!$this->approveManualRoom($resolved)){
            return ['ok'=>false,'msg'=>'保存人工审核房间失败'];
        }
        $stream=$uid.'_direct_'.time();
        if(!$this->createLiveRow(
            $task,
            [],
            $stream,
            $page,
            1,
            'virtual_live_direct'
        )){
            return ['ok'=>false,'msg'=>'创建前端直拉直播间失败'];
        }

        Db::name('virtual_live_task')->where(['id'=>$task['id']])->update([
            'status'=>1,
            'pid'=>0,
            'stream'=>$stream,
            'push_url'=>'',
            'pull_url'=>$page,
            'input_url'=>$page,
            'command'=>'',
            'log_file'=>'',
            'error_msg'=>'',
            'starttime'=>time(),
            'stoptime'=>0,
            'updatetime'=>time(),
        ]);
        setcaches('virtual_live_direct_source_'.$task['id'],[
            'url'=>$resolved['url'],
            'format'=>'hls',
            'height'=>(string)$resolved['height'],
            'resolution'=>$resolved['resolution'],
            'provider'=>'douyin',
            'room_id'=>$resolved['room_id'],
            'room_page'=>$resolved['room_page'],
            'cache_seconds'=>'30',
            'delivery'=>'direct',
        ],30);
        $this->markLiveStarted($uid);
        return ['ok'=>true,'msg'=>'房间已校验并开播，媒体由前端直拉'];
    }

    protected function startPagePullTask($task){
        $page=trim($task['source_page'] ?? '');
        if(!$this->isValidPageRoomUrl($page)){
            return ['ok'=>false,'msg'=>'请填写抖音直播首页、分类页或具体房间地址'];
        }
        if(!$this->isApprovedFemaleSource($page)){
            return ['ok'=>false,'msg'=>'该来源未通过女性主播审核，禁止接入'];
        }

        $runningQuery=Db::name('virtual_live_task')
            ->where(['source_type'=>3,'status'=>1])
            ->where('id','<>',(int)$task['id']);
        if((int)$runningQuery->count()>=$this->getPageFleetLimit()){
            return ['ok'=>false,'msg'=>'已达到 '.$this->getPageFleetLimit().' 路直播接入上限'];
        }
        $sameSource=Db::name('virtual_live_task')
            ->where(['source_type'=>3,'status'=>1,'source_page'=>$page])
            ->where('id','<>',(int)$task['id'])
            ->value('id');
        if($sameSource){
            return ['ok'=>false,'msg'=>'这个来源房间已经接入'];
        }
        $roomId=$this->roomIdFromSourcePage($page);
        if($roomId!==''){
            $runningPages=Db::name('virtual_live_task')
                ->where(['source_type'=>3,'status'=>1])
                ->where('id','<>',(int)$task['id'])
                ->column('source_page');
            foreach($runningPages as $runningPage){
                if($this->roomIdFromSourcePage($runningPage)===$roomId){
                    return ['ok'=>false,'msg'=>'这个抖音房间号已经接入'];
                }
            }
        }

        if($this->isDirectDeliveryMode()){
            return $this->startPageDirectTask($task);
        }

        if(!$this->ensurePython()){
            return ['ok'=>false,'msg'=>'服务器未安装 Python，无法启动 PAGE 拉流'];
        }
        if(!$this->ensureFfmpeg()){
            return ['ok'=>false,'msg'=>'服务器未安装 ffmpeg，无法启动 PAGE 转推'];
        }

        $script=$this->getPagePullScript();
        if(!file_exists($script)){
            return ['ok'=>false,'msg'=>'PAGE 拉流脚本不存在'];
        }

        $environment=[
            'PYTHONUNBUFFERED=1',
            'PAGE='.$page,
            'MAX_HEIGHT=720',
            'PULL_FORMAT=hls',
            'DURATION=0',
            'LIVE_START_INDEX=-2',
            'HLS_MAX_RELOAD=20',
            'HLS_HOLD_COUNTERS=20',
            'RANDOM_ROOM=0',
            'ROOM_RETRY=1',
            'DOUYIN_FETCH_INTERVAL_MS=1200',
            'DOUYIN_444_COOLDOWN=45',
            'STRICT_GENDER=1',
            'ROOM_POOL_ONLY=1',
            'GENDER_FILTER=female',
        ];
        $environmentArgs=[];
        foreach($environment as $value){
            $environmentArgs[]=$this->shellQuote($value);
        }
        $envCommand='env '.implode(' ',$environmentArgs);

        $preflight=$envCommand.' python3 '.$this->shellQuote($script).' --resolve-only 2>&1';
        $preflightOutput=[];
        $preflightCode=0;
        exec($preflight,$preflightOutput,$preflightCode);
        if($preflightCode!==0){
            $detail=trim(implode("\n",array_slice($preflightOutput,-6)));
            return [
                'ok'=>false,
                'msg'=>'来源预检失败'.($detail!=='' ? '：'.$detail : ''),
            ];
        }

        $urls=$this->prepareLiveUrls($task);
        if(!$urls['ok']){
            return $urls;
        }

        if(!$this->createLiveRow($task,[],$urls['stream'],$urls['pull_url'],1,'virtual_live_restream')){
            return ['ok'=>false,'msg'=>'创建直播间失败'];
        }

        $logFile=$this->getLogFile($task['id'],'page');
        $pushEnvironment=$environment;
        $pushEnvironment[]='PUSH_URL='.$urls['push_url'];
        $pushArgs=[];
        foreach($pushEnvironment as $value){
            $pushArgs[]=$this->shellQuote($value);
        }
        $cmd='nohup env '.implode(' ',$pushArgs)
            .' python3 '.$this->shellQuote($script)
            .' > '.$this->shellQuote($logFile).' 2>&1 & echo $!';
        $pid=(int)trim((string)shell_exec($cmd));
        if($pid<=0){
            $this->closeLiveByTask(['uid'=>$task['uid'],'stream'=>$urls['stream']]);
            return ['ok'=>false,'msg'=>'PAGE 转推进程启动失败'];
        }

        usleep(800000);
        if(!$this->processRunning($pid)){
            $this->closeLiveByTask(['uid'=>$task['uid'],'stream'=>$urls['stream']]);
            $log=$this->getLastLogLines($logFile);
            return ['ok'=>false,'msg'=>'PAGE 转推进程启动后退出'.($log ? '：'.$log : '')];
        }

        Db::name('virtual_live_task')->where(['id'=>$task['id']])->update([
            'status'=>1,
            'pid'=>$pid,
            'stream'=>$urls['stream'],
            'push_url'=>$urls['push_url'],
            'pull_url'=>$urls['pull_url'],
            'input_url'=>$page,
            'command'=>$cmd,
            'log_file'=>$logFile,
            'error_msg'=>'',
            'starttime'=>time(),
            'stoptime'=>0,
            'updatetime'=>time(),
        ]);

        $this->markLiveStarted((int)$task['uid']);
        return ['ok'=>true,'msg'=>'抖音 PAGE 拉流转推已启动'];
    }

    protected function stopTask($id,$status=2,$msg=''){
        $task=Db::name('virtual_live_task')->where(['id'=>$id])->find();
        if(!$task){
            return ['ok'=>false,'msg'=>'任务不存在'];
        }

        $this->killProcess($task['pid']);
        $this->closeLiveByTask($task);

        Db::name('virtual_live_task')->where(['id'=>$id])->update([
            'status'=>$status,
            'pid'=>0,
            'error_msg'=>$msg,
            'stoptime'=>time(),
            'updatetime'=>time(),
        ]);

        return ['ok'=>true,'msg'=>'已停止'];
    }

    public function index(){
        $this->syncRunningTaskStatus();

        $data=$this->request->param();
        $map=[];

        $uid=$data['uid'] ?? '';
        if($uid!==''){
            $map[]=['uid','=',$uid];
        }

        $status=$data['status'] ?? '';
        if($status!==''){
            $map[]=['status','=',$status];
        }

        $keyword=trim($data['keyword'] ?? '');
        if($keyword!==''){
            $map[]=['title|topic','like','%'.$keyword.'%'];
        }

        $lists=Db::name('virtual_live_task')
            ->where($map)
            ->order('id desc')
            ->paginate(20);

        $lists->each(function($v){
            $v['userinfo']=getUserInfo($v['uid']);
            $v['video']=Db::name('virtual_live_video')->where(['id'=>$v['video_id']])->find();
            $v['status_text']=$this->getStatusText($v['status']);
            $v['source_type']=(int)($v['source_type'] ?? 1);
            $v['source_type_text']=$this->getSourceTypes($v['source_type']);
            $v['is_running']=$v['status']==1 && (!$this->taskNeedsProcess($v) || $this->processRunning($v['pid']));
            return $v;
        });

        $lists->appends($data);
        $this->assign('lists',$lists);
        $this->assign('page',$lists->render());
        $this->assign('statusList',[0=>'待开始',1=>'直播中',2=>'已停止',3=>'失败']);
        $this->assign('fleetLimit',$this->getPageFleetLimit());
        $this->assign('fleetRunning',Db::name('virtual_live_task')->where(['source_type'=>3,'status'=>1])->count());

        return $this->fetch();
    }

    public function add(){
        $users=Db::name('user')
            ->field('id,user_nickname')
            ->where(['user_type'=>2,'user_status'=>1,'is_virtual'=>1])
            ->order('id desc')
            ->limit(500)
            ->select()
            ->toArray();

        $videos=Db::name('virtual_live_video')
            ->where(['status'=>1])
            ->order('id desc')
            ->select()
            ->toArray();

        $this->assign('users',$users);
        $this->assign('videos',$videos);
        $this->assign('liveclass',$this->getLiveClass());
        $this->assign('type',$this->getTypes());
        $this->assign('sourceTypes',$this->getSourceTypes());
        return $this->fetch();
    }

    public function addPost(){
        if(!$this->request->isPost()){
            $this->error('请求方式错误');
        }

        $data=$this->request->param();
        $uid=(int)($data['uid'] ?? 0);
        $videoId=(int)($data['video_id'] ?? 0);
        $sourceType=(int)($data['source_type'] ?? 1);
        $sourcePage=trim($data['source_page'] ?? '');
        $manualRoom=[];
        if(!array_key_exists($sourceType,$this->getSourceTypes())){
            $sourceType=1;
        }

        if($uid<=0){
            $this->error('请选择虚拟用户');
        }
        if($sourceType===1 && $videoId<=0){
            $this->error('请选择视频素材');
        }

        $user=Db::name('user')->where(['id'=>$uid,'user_type'=>2,'user_status'=>1,'is_virtual'=>1])->find();
        if(!$user){
            $this->error('虚拟用户不存在');
        }

        $video=[];
        if($sourceType===1){
            $video=Db::name('virtual_live_video')->where(['id'=>$videoId,'status'=>1])->find();
            if(!$video){
                $this->error('视频素材不存在或已停用');
            }
        }else{
            $videoId=0;
        }

        if($sourceType===3){
            $normalized=$this->normalizeManualRoomPage($sourcePage);
            if(empty($normalized['ok'])){
                $this->error($normalized['msg'] ?? '请填写抖音房间号');
            }
            $sourcePage=$normalized['room_page'];
            $manualRoom=$this->resolveExactDouyinRoom($sourcePage,true);
            if(empty($manualRoom['ok'])){
                $this->error($manualRoom['msg'] ?? '该房间当前无法播放');
            }
            if(!$this->approveManualRoom($manualRoom)){
                $this->error('保存人工审核房间失败');
            }
        }
        if($sourceType!==3){
            $sourcePage='';
        }

        $type=(int)($data['type'] ?? 0);
        $typeVal=trim($data['type_val'] ?? '');
        if($type==1 && $typeVal===''){
            $this->error('密码房间需要填写密码');
        }
        if($type>1 && (float)$typeVal<=0){
            $this->error('收费房间价格必须大于0');
        }
        if($type==0){
            $typeVal='';
        }

        $thumb=trim($data['thumb'] ?? '');
        if($thumb!==''){
            $thumb=set_upload_path($thumb);
        }

        $title=trim($data['title'] ?? '');
        if($title===''){
            if($sourceType===1 && !empty($video['title'])){
                $title=$video['title'];
            }elseif($sourceType===2){
                $title='OBS 虚拟直播';
            }elseif($sourceType===3){
                $title=trim((string)($manualRoom['title'] ?? ''));
                if($title===''){
                    $nickname=trim((string)($manualRoom['nickname'] ?? ''));
                    $title=$nickname!=='' ? $nickname.'正在直播' : '抖音精选直播';
                }
            }else{
                $title='虚拟直播';
            }
        }

        $task=[
            'uid'=>$uid,
            'video_id'=>$videoId,
            'source_type'=>$sourceType,
            'source_page'=>$sourcePage,
            'title'=>$title,
            'topic'=>trim($data['topic'] ?? ''),
            'thumb'=>$thumb,
            'liveclassid'=>(int)($data['liveclassid'] ?? 0),
            'type'=>$type,
            'type_val'=>$typeVal,
            'anyway'=>(int)($data['anyway'] ?? 0),
            'province'=>trim($data['province'] ?? ''),
            'city'=>trim($data['city'] ?? '好像在火星'),
            'lng'=>trim($data['lng'] ?? ''),
            'lat'=>trim($data['lat'] ?? ''),
            'loop_play'=>(int)($data['loop_play'] ?? 1),
            'status'=>0,
            'addtime'=>time(),
            'updatetime'=>time(),
        ];

        $id=Db::name('virtual_live_task')->insertGetId($task);
        if(!$id){
            $this->error('创建任务失败');
        }

        setAdminLog('创建虚拟直播任务：'.$id);

        if(($data['op'] ?? '')==='start'){
            $res=$this->startTask($id);
            if(!$res['ok']){
                $this->error('任务已创建，但启动失败：'.$res['msg'],url('Virtuallive/index'));
            }
            $this->success('创建并开播成功',url('Virtuallive/index'));
        }

        $this->success('创建成功',url('Virtuallive/index'));
    }

    public function bulkPageStart(){
        if(!$this->request->isPost()){
            $this->error('请求方式错误');
        }
        $count=$this->request->param('count',8,'intval');
        $page=trim($this->request->param('source_page','https://live.douyin.com/'));
        if($count<1 || $count>1000){
            $this->error('启动数量必须在 1-1000 之间');
        }
        $result=$this->createPageFleet($count,$page);
        if(!$result['ok']){
            $this->error($result['msg']);
        }
        setAdminLog('批量发现并接入抖音直播：'.$result['started'].' 路');
        $message='已启动 '.$result['started'].' 路';
        if(!empty($result['failed'])){
            $message.='，失败 '.$result['failed'].' 路';
        }
        $this->success($message,url('Virtuallive/index'));
    }

    public function batchReplace(){
        if(!$this->request->isPost()){
            $this->error('请求方式错误');
        }
        $target=$this->request->param('target',300,'intval');
        $page=trim($this->request->param('source_page','https://live.douyin.com/'));
        if($target<1 || $target>1000){
            $this->error('目标数量必须在 1-1000 之间');
        }
        if(!$this->isValidPageRoomUrl($page)){
            $this->error('抖音页面地址不正确');
        }
        $result=$this->batchReplacePageTasks($target,$page);
        if(!$result['ok']){
            $this->error($result['msg']);
        }
        setAdminLog(
            '批量更换抖音女性直播：目标 '.$result['target']
            .'，下线未审核 '.$result['stopped_unsafe']
            .'，下线离线 '.$result['stopped_offline']
            .'，新增 '.$result['started']
            .'，最终 '.$result['final']
        );
        $message='批量更换完成：下线未审核 '.$result['stopped_unsafe'].' 路'
            .'，下线离线 '.$result['stopped_offline'].' 路'
            .'，新增 '.$result['started'].' 路'
            .'，当前 '.$result['final'].' / '.$result['target'].' 路';
        if($result['shortage']>0){
            $message.='；缺少 '.$result['shortage'].' 路已审核且在线的女性房间，保持不播';
        }
        $this->success($message,url('Virtuallive/index'));
    }

    public function start(){
        $id=$this->request->param('id',0,'intval');
        $res=$this->startTask($id);
        if(!$res['ok']){
            $this->error($res['msg']);
        }
        setAdminLog('启动虚拟直播任务：'.$id);
        $this->success($res['msg']);
    }

    public function stop(){
        $id=$this->request->param('id',0,'intval');
        $res=$this->stopTask($id);
        if(!$res['ok']){
            $this->error($res['msg']);
        }
        setAdminLog('停止虚拟直播任务：'.$id);
        $this->success($res['msg']);
    }

    public function restart(){
        $id=$this->request->param('id',0,'intval');
        $res=$this->stopTask($id,2,'重启前停止');
        if(!$res['ok']){
            $this->error($res['msg']);
        }
        $res=$this->startTask($id);
        if(!$res['ok']){
            $this->error('停止成功，重新启动失败：'.$res['msg']);
        }
        setAdminLog('重启虚拟直播任务：'.$id);
        $this->success('重启成功');
    }

    public function del(){
        $id=$this->request->param('id',0,'intval');
        $task=Db::name('virtual_live_task')->where(['id'=>$id])->find();
        if(!$task){
            $this->error('任务不存在');
        }
        if((int)$task['status']===1){
            $this->stopTask($id,2,'删除前停止');
        }
        Db::name('virtual_live_task')->where(['id'=>$id])->delete();
        setAdminLog('删除虚拟直播任务：'.$id);
        $this->success('删除成功');
    }

    public function video(){
        $lists=Db::name('virtual_live_video')
            ->order('id desc')
            ->paginate(20);

        $lists->each(function($v){
            $v['cover_url']=$v['cover'] ? get_upload_path($v['cover']) : '';
            $v['play_url']=$v['file_url'] ?: get_upload_path($v['file_path']);
            return $v;
        });

        $this->assign('lists',$lists);
        $this->assign('page',$lists->render());
        return $this->fetch();
    }

    public function videoAdd(){
        return $this->fetch();
    }

    public function videoAddPost(){
        if(!$this->request->isPost()){
            $this->error('请求方式错误');
        }

        $data=$this->request->param();
        $title=trim($data['title'] ?? '');
        if($title===''){
            $this->error('请填写素材名称');
        }

        $file=$_FILES['file'] ?? null;
        if(!$file || empty($file['tmp_name'])){
            $this->error('请上传视频文件');
        }

        $ext=strtolower(pathinfo($file['name'],PATHINFO_EXTENSION));
        if(!in_array($ext,['mp4','mov','m4v','flv'])){
            $this->error('仅支持 mp4/mov/m4v/flv 视频');
        }

        $config=getConfigPri();
        $upload=adminUploadFiles($file,$config['cloudtype'] ?? '3','virtual_live');
        if(!$upload){
            $this->error('视频上传失败');
        }

        $cover=trim($data['cover'] ?? '');
        if($cover!==''){
            $cover=set_upload_path($cover);
        }

        $id=Db::name('virtual_live_video')->insertGetId([
            'title'=>$title,
            'file_path'=>$upload['storage_path'],
            'file_url'=>$upload['url'],
            'cover'=>$cover,
            'duration'=>(int)($data['duration'] ?? 0),
            'filesize'=>(int)($file['size'] ?? 0),
            'status'=>1,
            'addtime'=>time(),
            'updatetime'=>time(),
        ]);

        if(!$id){
            $this->error('保存素材失败');
        }

        setAdminLog('上传虚拟直播素材：'.$id);
        $this->success('上传成功',url('Virtuallive/video'));
    }

    public function videoDel(){
        $id=$this->request->param('id',0,'intval');
        $used=Db::name('virtual_live_task')->where(['video_id'=>$id,'status'=>1])->count();
        if($used>0){
            $this->error('素材正在推流，不能删除');
        }
        Db::name('virtual_live_video')->where(['id'=>$id])->delete();
        setAdminLog('删除虚拟直播素材：'.$id);
        $this->success('删除成功');
    }

    public function roomPool(){
        $this->normalizeRoomGenderState();
        $query=Db::name('live_source_room_pool')->where(['provider'=>'douyin']);
        $status=trim((string)$this->request->param('audit',''));
        if($status==='female'){
            $query->where(['gender_tag'=>'female','verify_status'=>1,'status'=>1]);
        }elseif($status==='rejected'){
            $query->where('verify_status',2);
        }elseif($status==='disabled'){
            $query->where('status',0);
        }elseif($status==='pending'){
            $query->where('verify_status',0);
        }
        $keyword=trim((string)$this->request->param('keyword',''));
        if($keyword!==''){
            $query->where('room_id|nickname|title|uniq_id','like','%'.$keyword.'%');
        }
        $lists=$query->order('last_seen_at desc,id desc')->paginate(100);
        $this->assign('lists',$lists);
        $this->assign('page',$lists->render());
        $this->assign('poolCount',Db::name('live_source_room_pool')->where(['provider'=>'douyin'])->count());
        $this->assign('femaleCount',Db::name('live_source_room_pool')->where([
            'provider'=>'douyin','gender_tag'=>'female','verify_status'=>1,'status'=>1,
        ])->count());
        $this->assign('pendingCount',Db::name('live_source_room_pool')->where([
            'provider'=>'douyin','verify_status'=>0,
        ])->count());
        $this->assign('rejectedCount',Db::name('live_source_room_pool')->where([
            'provider'=>'douyin','verify_status'=>2,
        ])->count());
        return $this->fetch('rooms');
    }

    public function discoverCandidates(){
        if(!$this->request->isPost()){
            $this->error('请求方式错误');
        }
        $count=$this->request->param('count',300,'intval');
        $page=trim($this->request->param('source_page','https://live.douyin.com/'));
        if($count<1 || $count>1000){
            $this->error('发现数量必须在 1-1000 之间');
        }
        $result=$this->discoverPageRooms($page,$count);
        if(!$result['ok']){
            $this->error($result['msg']);
        }
        foreach($result['rooms'] as $room){
            $categoryPage=trim((string)($room['category_page'] ?? '')) ?: $page;
            $this->storeDiscoveredRoom($room,$categoryPage);
        }
        setAdminLog('批量发现抖音直播候选：'.count($result['rooms']).' 路');
        $this->success(
            '已发现并更新 '.count($result['rooms']).' 个在线候选；新候选默认待审核，不会自动上线',
            url('Virtuallive/roomPool')
        );
    }

    public function auditRooms(){
        if(!$this->request->isPost()){
            $this->error('请求方式错误');
        }
        $ids=$this->request->param('ids/a',[]);
        $ids=array_values(array_unique(array_filter(array_map('intval',$ids))));
        if(!$ids || count($ids)>1000){
            $this->error('请选择 1-1000 个房间');
        }
        $action=trim((string)$this->request->param('audit_action',''));
        $now=time();
        if($action==='female'){
            $data=[
                'gender_tag'=>'female',
                'verify_status'=>1,
                'verify_source'=>'manual_batch',
                'confidence'=>1,
                'status'=>1,
                'last_verified_at'=>$now,
                'update_time'=>$now,
            ];
        }elseif($action==='reject'){
            $data=[
                'gender_tag'=>'male',
                'verify_status'=>2,
                'verify_source'=>'manual_batch',
                'confidence'=>1,
                'status'=>0,
                'last_verified_at'=>$now,
                'update_time'=>$now,
            ];
        }elseif($action==='pending'){
            $data=[
                'gender_tag'=>'unknown',
                'verify_status'=>0,
                'verify_source'=>'',
                'confidence'=>0,
                'status'=>1,
                'last_verified_at'=>0,
                'update_time'=>$now,
            ];
        }else{
            $this->error('审核操作不正确');
        }
        $roomIds=Db::name('live_source_room_pool')
            ->where(['provider'=>'douyin'])
            ->whereIn('id',$ids)
            ->column('room_id');
        $updated=Db::name('live_source_room_pool')
            ->where(['provider'=>'douyin'])
            ->whereIn('id',$ids)
            ->update($data);
        $stopped=0;
        if($action!=='female' && $roomIds){
            $tasks=Db::name('virtual_live_task')
                ->where(['source_type'=>3,'status'=>1])
                ->select()
                ->toArray();
            $roomMap=array_fill_keys(array_map('strval',$roomIds),true);
            foreach($tasks as $task){
                $roomId=$this->roomIdFromSourcePage($task['source_page'] ?? '');
                if(!isset($roomMap[$roomId])){
                    continue;
                }
                $result=$this->stopTask((int)$task['id'],2,'来源房间审核未通过，批量下线');
                if($result['ok']){
                    $stopped++;
                }
            }
        }
        setAdminLog('批量审核抖音房间：'.$action.'，'.$updated.' 条');
        $this->success('已更新 '.$updated.' 个房间'.($stopped>0 ? '，并下线 '.$stopped.' 路直播' : ''),url('Virtuallive/roomPool'));
    }

    public function importFemaleRooms(){
        if(!$this->request->isPost()){
            $this->error('请求方式错误');
        }
        $text=(string)$this->request->param('rooms','');
        $lines=preg_split('/[\s,;]+/u',$text,-1,PREG_SPLIT_NO_EMPTY);
        $roomIds=[];
        foreach($lines as $line){
            $line=trim($line);
            if(preg_match('/^\d{5,}$/',$line)){
                $roomIds[]=$line;
            }elseif(preg_match('#live\.douyin\.com/(\d{5,})#i',$line,$match)){
                $roomIds[]=$match[1];
            }
        }
        $roomIds=array_values(array_unique($roomIds));
        if(!$roomIds || count($roomIds)>1000){
            $this->error('请填写 1-1000 个已人工确认的女性房间号或链接');
        }
        $now=time();
        $saved=0;
        foreach($roomIds as $roomId){
            $existing=Db::name('live_source_room_pool')
                ->where(['provider'=>'douyin','room_id'=>$roomId])
                ->value('id');
            $data=[
                'room_page'=>'https://live.douyin.com/'.$roomId,
                'gender_tag'=>'female',
                'verify_status'=>1,
                'verify_source'=>'manual_import',
                'confidence'=>1,
                'status'=>1,
                'last_verified_at'=>$now,
                'update_time'=>$now,
            ];
            if($existing){
                Db::name('live_source_room_pool')->where(['id'=>$existing])->update($data);
            }else{
                $data['provider']='douyin';
                $data['room_id']=$roomId;
                $data['create_time']=$now;
                Db::name('live_source_room_pool')->insert($data);
            }
            $saved++;
        }
        setAdminLog('批量导入已确认女性抖音房间：'.$saved.' 条');
        $this->success('已导入 '.$saved.' 个女性白名单房间',url('Virtuallive/roomPool'));
    }

    public function accounts(){
        $lists=Db::name('user')
            ->where(['user_type'=>2,'is_virtual'=>1])
            ->order('id desc')
            ->paginate(30);

        $lists->each(function($v){
            $v['avatar_url']=get_upload_path($v['avatar']);
            return $v;
        });

        $this->assign('lists',$lists);
        $this->assign('page',$lists->render());
        $this->assign('count',Db::name('user')->where(['user_type'=>2,'is_virtual'=>1])->count());
        $this->assign('avatarCount',count($this->getVirtualAvatarFiles()));
        $this->assign('avatarUsed',Db::name('user')
            ->where(['user_type'=>2,'is_virtual'=>1])
            ->where('avatar','like','local_virtual_avatar/girls/%')
            ->count());
        return $this->fetch();
    }

    public function generateUsers(){
        $count=$this->request->param('count',300,'intval');
        if($count<1 || $count>1000){
            $this->error('生成数量必须在 1-1000 之间');
        }

        $result=$this->createVirtualUsers(
            $count,
            trim($this->request->param('nick_prefix',''))
        );
        if(!$result['ok']){
            $this->error($result['msg']);
        }
        setAdminLog('批量生成女生头像虚拟直播用户：'.$result['created']);
        $this->success('已生成 '.$result['created'].' 个女生头像虚拟用户，剩余可用头像 '.$result['available'].' 张');
    }

    public function log(){
        $id=$this->request->param('id',0,'intval');
        $task=Db::name('virtual_live_task')->where(['id'=>$id])->find();
        if(!$task){
            $this->error('任务不存在');
        }

        $log='暂无日志';
        if(!empty($task['log_file']) && file_exists($task['log_file'])){
            $lines=file($task['log_file']);
            $log=implode('',array_slice($lines,-200));
        }

        $pushInfo=$this->splitPushUrl($task['push_url'] ?? '');
        $task['source_type_text']=$this->getSourceTypes($task['source_type'] ?? 1);
        $this->assign('task',$task);
        $this->assign('pushInfo',$pushInfo);
        $this->assign('log',$log);
        return $this->fetch();
    }
}
