<?php
/* redis */
/* redis链接 */
function connectionRedis(){
    if(!isset($GLOBALS['redisdb']) || !$GLOBALS['redisdb']){
        if(!class_exists('\Redis')){
            return false;
        }

        $REDIS_HOST= config('database.REDIS_HOST');
        $REDIS_AUTH= config('database.REDIS_AUTH');
        $REDIS_PORT= config('database.REDIS_PORT');
        $redis = new \Redis();
        $redis -> connect($REDIS_HOST,$REDIS_PORT);
        if($REDIS_AUTH !== ''){
            $redis -> auth($REDIS_AUTH);
        }

        $GLOBALS['redisdb']=$redis;        
    }

    return $GLOBALS['redisdb'];
}

function claw_redis(){
    if(isset($GLOBALS['redisdb']) && $GLOBALS['redisdb']){
        return $GLOBALS['redisdb'];
    }

    try{
        return connectionRedis();
    }catch(\Throwable $e){
        return false;
    }
}
//connectionRedis();
/* 设置缓存 */
function setcache($key,$info){
    $redis=claw_redis();
    if(!$redis){
        return 0;
    }

    $config=getConfigPri();
    if(($config['cache_switch'] ?? 0)!=1){
        return 1;
    }
    $redis->set($key,json_encode($info));
    $redis->expire($key, $config['cache_time'] ?? 0); 
	    
    return 1;
}	
/**
 * redis 字符串（String） 类型
 * 将key和value对应。如果key已经存在了，它会被覆盖，而不管它是什么类型。
 * @param $key
 * @param $info
 * @param $exp 过期时间
 */
function setcaches($key,$info,$time=0){
    $redis=claw_redis();
    if(!$redis){
        return 0;
    }

    $redis->set($key,json_encode($info));
    if($time > 0){
        $redis->expire($key, $time); 
    }
    return 1;
}
/* 获取缓存 */
function getcache($key){
    $redis=claw_redis();
    if(!$redis){
        return null;
    }

    $config=getConfigPri();
    $isexist=$redis->Get($key);
    if(($config['cache_switch'] ?? 0)!=1){
        $isexist=false;
    }
    return json_decode($isexist,true);
}	
	
/**
 * redis 字符串（String） 类型
 * 返回key的value。如果key不存在，返回特殊值nil。如果key的value不是string，就返回错误，因为GET只处理string类型的values。
 * @param $key
 */
function getcaches($key){
    $redis=claw_redis();
    if(!$redis){
        return null;
    }

    $isexist=$redis->Get($key);
    return json_decode($isexist,true);
}

/**
 * 删除一个或多个key
 * @param $keys  数组/ 数组以逗号拼接的string
 */
function delcache($key){
    $redis=claw_redis();
    if(!$redis){
        return 0;
    }

    $redis->del($key);
    return 1;
}

/**
 * redis 哈希表(hash)类型
 * 返回哈希表 $key 中，所有的域和值。
 * @param $key
 *
 */
function hGetAll($key){
    $redis=claw_redis();
    return $redis ? $redis->hGetAll($key) : [];
}

/**
 * 添加一个VALUE到HASH中。如果VALUE已经存在于HASH中，则返回FALSE。
 * @param string $key
 * @param string $hashKey
 * @param string $value
 */
function hSet( $key, $hashKey, $value ) {
    $redis=claw_redis();
    return $redis ? $redis->hSet($key, $hashKey, $value) : false;
}

/**
 * redis 哈希表(hash)类型 
 * 批量填充HASH表。不是字符串类型的VALUE，自动转换成字符串类型。使用标准的值。NULL值将被储存为一个空的字符串。
 * 可以批量添加更新 value,key 不存在将创建，存在则更新值
 * @param  $key
 * @param  $fieldArr  要设置的键对值
 * @return
 * 当key不是哈希表(hash)类型时，返回一个错误。
 */
function hMSet($key,$fieldArr){
    $redis=claw_redis();
    return $redis ? $redis->hmset($key,$fieldArr) : false;
}

/**
 * 取得HASH中的VALUE，如何HASH不存在，或者KEY不存在返回FLASE。
 * @param   string  $key
 * @param   string  $hashKey
 * @return  string  The value, if the command executed successfully BOOL FALSE in case of failure
 */
function hGet($key, $hashKey) {
    $redis=claw_redis();
    return $redis ? $redis->hGet($key,$hashKey) : false;
}

/**
 * 批量取得HASH中的VALUE，如何hashKey不存在，或者KEY不存在返回FLASE。
 * @param string $key
 * @param array $hashKey 
 */
function hMGet( $key, $hashKeys ) {
    $redis=claw_redis();
    return $redis ? $redis->hMGet($key,$hashKeys) : false;
}

/**
 * 根据HASH表的KEY，为KEY对应的VALUE自增参数VALUE。浮点型 
 * 推荐使用 hIncrByFloat  不推荐使用 hIncrBy(整型)
 * 先用 hIncrByFloat 再使用  hIncrBy  自增无效
 * @param string $key
 * @param string $hashKey
 * @param value  自增值  整型/小数
 */
function hIncrByFloat( $key, $hashKey, $value){
    $redis=claw_redis();
    return $redis ? $redis->hIncrByFloat( $key, $hashKey, $value) : false;
}

/**
 * 根据HASH表的KEY，为KEY对应的VALUE自增参数VALUE。整数型 
 * @param string $key
 * @param string $hashKey
 * @param value  自增值  整型
 */
function hIncrBy($key,$hashKey, $value){
    $redis=claw_redis();
    return $redis ? $redis->hIncrBy( $key, $hashKey, $value) : false;
}

/**
 *  删除哈希表key中的一个指定域，不存在的域将被忽略。
 * @param string $key
 * @param string $hashKey
 */
function hDel($key,$hashKey){
    $redis=claw_redis();
    return $redis ? $redis->hDel( $key, $hashKey) : false;
}



/**
 * 添加一个字符串值到LIST容器的顶部（左侧），如果KEY不存在，曾创建一个LIST容器，如果KEY存在并且不是一个LIST容器，那么返回FLASE。
 * @param string $key
 * @param string $val
 */
function lPush($key,$val){
    $redis=claw_redis();
    return $redis ? $redis->lPush($key,$val) : false;
}

/**
 * 添加一个字符串值到LIST容器的底部（右侧），如果KEY不存在，曾创建一个LIST容器，如果KEY存在并且不是一个LIST容器，那么返回FLASE。
 * 
 * @param string $key
 * @param string $val
 */
function rPush($key,$val){
    $redis=claw_redis();
    return $redis ? $redis->rPush($key,$val) : false;
}
/**
 * 返回LIST顶部（左侧）的VALUE，并且从LIST中把该VALUE弹出。
 * @param string $key
 */
function lPop($key){
    $redis=claw_redis();
    return $redis ? $redis->lPop($key) : false;
}
/**
 * 返回LIST底部（右侧）的VALUE，并且从LIST中把该VALUE弹出。
 * @param string $key
 */
function rPop($key){
    $redis=claw_redis();
    return $redis ? $redis->rPop($key) : false;
}


/*
 * 构建一个集合(有序集合)  可排序
 * @param  string $key 集合名称
 * @param  string $value1  值
 * @param  double $score1  值
 * return 被成功添加的新成员的数量，不包括那些被更新的、已经存在的成员。
 */
function zAdd($key,$score1,$value1){
    $redis=claw_redis();
    return $redis ? $redis->zAdd($key,$score1,$value1) : false;
}

/**
 * 返回key对应的有序集合中member的score值。如果member在有序集合中不存在，那么将会返回nil。
 * @param   string  $key
 * @param   string  $member
 * @return  float
 */
function zScore( $key, $member ) {
    $redis=claw_redis();
    return $redis ? $redis->zScore( $key, $member ) : false;
}

/**
 * 返回存储在key对应的有序集合中的元素的个数。
 * @param   string  $key
 * @return  int     the set's cardinality

 */
function zSize($key){
    $redis=claw_redis();
    return $redis ? $redis->zCard($key) : 0;
}

/**
 * 将key对应的有序集合中member元素的scroe加上 value     value可以是负值
 * @param   string  $key
 * @param   float   $value (double) value that will be added to the member's score
 * @param   string  $member
 * @return  float   the new value
 * @example
 * <pre>
 * $redis->delete('key');
 * $redis->zIncrBy('key', 2.5, 'member1');  // key or member1 didn't exist, so member1's score is to 0
 *                                          // before the increment and now has the value 2.5
 * $redis->zIncrBy('key', 1, 'member1');    // 3.5
 * </pre>
 * member 成员的新 score 值，以字符串形式表示。
 */
function zIncrBy( $key, $value, $member ) {
    $redis=claw_redis();
    return $redis ? $redis->zIncrBy( $key, $value, $member ) : false;
}

/**
 * 取得特定范围内的排序元素,0代表第一个元素,1代表第二个以此类推。-1代表最后一个,-2代表倒数第二个...
 * @param   string  $key
 * @param   int     $start
 * @param   int     $end
 * @param   bool    $withscores 
 * @return  array   Array containing the values in specified range.
 * @example
 * <pre>
 * $redis->zAdd('key1', 0, 'val0');
 * $redis->zAdd('key1', 2, 'val2');
 * $redis->zAdd('key1', 10, 'val10');
 * $redis->zRange('key1', 0, -1); // array('val0', 'val2', 'val10')
 * // with scores
 * $redis->zRange('key1', 0, -1, true); // array('val0' => 0, 'val2' => 2, 'val10' => 10)
 * </pre>
 * 指定区间内，带有 score 值(可选)的有序集成员的列表。
 * zRange 根据 score 正序   zRevRange 倒序
 */
function zRange( $key, $start, $end, $withscores = null ) {
    $redis=claw_redis();
    return $redis ? $redis->zRange( $key, $start, $end, $withscores) : [];
}
function zRevRange( $key, $start, $end, $withscores = null ) {
    $redis=claw_redis();
    return $redis ? $redis->zRevRange( $key, $start, $end, $withscores) : [];
}

/**
 * 从有序集合中删除指定的成员。
 * @param   string  $key
 * @param   string  $member1
 * @return  int     Number of deleted values
 */
function zRem( $key, $member1 ) {
    $redis=claw_redis();
    return $redis ? $redis->zRem( $key, $member1 ) : false;
}

/**
 * 模糊查询key类似的列表
 * @param  string $key 模糊键字符串
 * @return array      类似key的数组
 */
function blurrySearch($key){
    $redis=claw_redis();
    return $redis ? $redis->keys($key."*") : [];
}
