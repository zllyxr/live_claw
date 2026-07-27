#!/usr/bin/env php
<?php

namespace think;

use app\admin\controller\VirtualliveController;
use think\facade\Db;

define('CMF_ROOT',dirname(__DIR__).'/');
define('CMF_DATA',CMF_ROOT.'data/');
define('APP_PATH',CMF_ROOT.'app/');
define('WEB_ROOT',CMF_ROOT.'public/');
define('RUNTIME_PATH',CMF_ROOT.'data/runtime_cli/');

require CMF_ROOT.'vendor/autoload.php';

$app=new App();
$app->initialize();

$options=getopt('',[
    'target-users::',
    'active::',
    'page::',
]);
$targetUsers=max(0,min(1000,(int)($options['target-users'] ?? 300)));
$targetActive=max(0,min(1000,(int)($options['active'] ?? 8)));
$page=trim((string)($options['page'] ?? 'https://live.douyin.com/'));

$reflection=new \ReflectionClass(VirtualliveController::class);
$controller=$reflection->newInstanceWithoutConstructor();
$invoke=function($method,array $arguments=[]) use ($reflection,$controller){
    $target=$reflection->getMethod($method);
    $target->setAccessible(true);
    return $target->invokeArgs($controller,$arguments);
};

$repairedUsers=(int)$invoke('repairVirtualUserProfiles');
if($repairedUsers>0){
    $invoke('updateLiveCache');
}
$currentUsers=(int)Db::name('user')
    ->where(['user_type'=>2,'is_virtual'=>1])
    ->count();
$userResult=['ok'=>true,'created'=>0,'available'=>0];
$missingUsers=max(0,$targetUsers-$currentUsers);
if($missingUsers>0){
    $userResult=$invoke('createVirtualUsers',[$missingUsers,'']);
}

$running=(int)Db::name('virtual_live_task')
    ->where(['source_type'=>3,'status'=>1])
    ->count();
$fleetResult=['ok'=>true,'created'=>0,'started'=>0,'failed'=>0,'msg'=>'无需新增直播接入'];
$missingActive=max(0,$targetActive-$running);
if($missingActive>0){
    $fleetResult=$invoke('createPageFleet',[$missingActive,$page]);
}

$result=[
    'ok'=>!empty($userResult['ok']) && !empty($fleetResult['ok']),
    'users'=>[
        'target'=>$targetUsers,
        'before'=>$currentUsers,
        'created'=>(int)($userResult['created'] ?? 0),
        'repaired'=>$repairedUsers,
        'after'=>(int)Db::name('user')
            ->where(['user_type'=>2,'is_virtual'=>1])
            ->count(),
        'message'=>(string)($userResult['msg'] ?? ''),
    ],
    'fleet'=>[
        'target'=>$targetActive,
        'before'=>$running,
        'created'=>(int)($fleetResult['created'] ?? 0),
        'started'=>(int)($fleetResult['started'] ?? 0),
        'failed'=>(int)($fleetResult['failed'] ?? 0),
        'after'=>(int)Db::name('virtual_live_task')
            ->where(['source_type'=>3,'status'=>1])
            ->count(),
        'message'=>(string)($fleetResult['msg'] ?? ''),
        'errors'=>$fleetResult['errors'] ?? [],
    ],
];

echo json_encode($result,JSON_UNESCAPED_UNICODE | JSON_PRETTY_PRINT).PHP_EOL;
exit($result['ok'] ? 0 : 1);
