#!/usr/bin/env php
<?php

namespace think;

use app\admin\controller\VirtualliveController;

define('CMF_ROOT',dirname(__DIR__).'/');
define('CMF_DATA',CMF_ROOT.'data/');
define('APP_PATH',CMF_ROOT.'app/');
define('WEB_ROOT',CMF_ROOT.'public/');
define('RUNTIME_PATH',CMF_ROOT.'data/runtime_cli/');

require CMF_ROOT.'vendor/autoload.php';

$app=new App();
$app->initialize();

$options=getopt('',[
    'target::',
    'page::',
]);
$target=max(1,min(1000,(int)($options['target'] ?? 300)));
$page=trim((string)($options['page'] ?? 'https://live.douyin.com/'));

$reflection=new \ReflectionClass(VirtualliveController::class);
$controller=$reflection->newInstanceWithoutConstructor();
$method=$reflection->getMethod('batchReplacePageTasks');
$method->setAccessible(true);
$result=$method->invoke($controller,$target,$page);

echo json_encode($result,JSON_UNESCAPED_UNICODE | JSON_PRETTY_PRINT).PHP_EOL;
exit(!empty($result['ok']) ? 0 : 1);
