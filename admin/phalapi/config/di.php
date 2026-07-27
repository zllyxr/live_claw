<?php
use PhalApi\Config\FileConfig;
use PhalApi\Logger\FileLogger;
use PhalApi\Database\NotORMDatabase;

$di = \PhalApi\DI();
$di->dotenv = Dotenv\Dotenv::createImmutable(API_ROOT);
$di->dotenv->safeLoad();
$di->config = new FileConfig(API_ROOT . DIRECTORY_SEPARATOR . 'config');
$di->debug = !empty($_GET['__debug__']) ? true : $di->config->get('sys.debug');
$di->logger = FileLogger::create($di->config->get('sys.file_logger'));
$di->notorm = new NotORMDatabase($di->config->get('dbs'), $di->config->get('sys.notorm_debug'));
\App\connectionRedis();
$di->qiniu = function() {
        return new \PhalApi\Qiniu\Lite();
};
