
var sw_client,sw_linkmic_client;

var sw_remoteUsers = {};

var sw_options = {
  appid: null,
  channel: null,
  uid: null,
  token: null
};

var sw_linkmic_options = {
  appid: null,
  channel: null,
  uid: null,
  token: null
};

 function join() {
  // Add an event listener to play remote tracks when remote user publishes.

  sw_client.on("user-published", handleUserPublished);
  sw_client.on("user-unpublished", handleUserUnpublished);
  // Join the channel.
  sw_options.uid =  sw_client.join(
    sw_options.appid, 
    sw_options.channel, 
    sw_options.token || null, 
    sw_options.uid || null
  );

  //console.log("publish success");
}

 function join2() {
  // Add an event listener to play remote tracks when remote user publishes.
  sw_linkmic_client.on("user-published", handleUserPublished2);
  sw_linkmic_client.on("user-unpublished", handleUserUnpublished);

  sw_linkmic_options.uid =  sw_linkmic_client.join(
    sw_linkmic_options.appid, 
    sw_linkmic_options.channel, 
    sw_linkmic_options.token || null, 
    sw_linkmic_options.uid || null
  );


}

/**
 * unlinkmic 主播与主播未连麦
 * linkmic 主播与主播连麦
 */
function handleUserPublished(user, mediaType) {
  const id = user.uid;
  sw_remoteUsers[id] = user;
  //console.log("获取live pkuid",_DATA.live.pkuid);
  if(_DATA.live.pkuid==0){
      subscribe(user, mediaType,'single','unlinkmic');
  }else{
      subscribe(user, mediaType,'single','linkmic');
  }
  
}

/**
 * unlinkmic 主播与主播未连麦
 * linkmic 主播与主播连麦
 */
function handleUserPublished2(user, mediaType) {
  const id = user.uid;
  sw_remoteUsers[id] = user;
  subscribe(user, mediaType,'double','linkmic');
}

function handleUserUnpublished(user, mediaType) {
  if (mediaType === "video") {
    const id = user.uid;
    delete sw_remoteUsers[id];
    $(`#player-wrapper-${id}`).remove();
  }
}

async function subscribe(user,mediaType,source,type) {
  const current_uid = user.uid;
  // subscribe to a remote user
  if(source=='single'){
      await sw_client.subscribe(user, mediaType);
  }else{
      await sw_linkmic_client.subscribe(user, mediaType);
  }
  
  //console.log("subscribe success");
  if (mediaType === "video") {

    console.log("直播类型：",_DATA.live.live_type);

    //视频直播
    if(_DATA.live.live_type==0){

        let player_area_height;

        if(type =='unlinkmic'){
          player_area_height=$("#area-video").height();
        }else{
          player_area_height = $("#area-video").height()/2;
        }

        // console.log(type);
        // console.log("播放画面高度：",player_area_height);

        let player;

        //console.log(_DATA.linkmic_uid,current_uid);
        
        //主播与用户连麦
        if(_DATA.linkmic_uid && _DATA.linkmic_uid==current_uid){

             player = $(`
              <div id="player-wrapper-${current_uid}">
                <div style="height:355px;width:200px;position:absolute;left:20px;bottom:10px;z-index:1;border:2px solid #FFF;" id="player-${current_uid}" class="player"></div>
              </div>
            `); 

        }else{

            player = $(`
              <div id="player-wrapper-${current_uid}">
                <div style="height:${player_area_height}px;width:100%;" id="player-${current_uid}" class="player"></div>
              </div>
            `);
        }

        $("#playerzmblbkjP").append(player);

        user.videoTrack.play(`player-${current_uid}`);
        $(".agora_video_player").css({"object-fit":"contain"});

    }else{

        if(_DATA.live.voice_type==0){
          let player;
          let player_area_height=$("#area-video").height();
          player = $(`
                <div id="player-wrapper-${current_uid}">
                  <img src="${avatar}" style="width:100%;height:${player_area_height}px;"/>
                </div>
              `);

          $("#playerzmblbkjP").html(player);
        }else{

          let player_area_height;

          if(type =='unlinkmic'){
            player_area_height=$("#area-video").height();
          }else{
            player_area_height = $("#area-video").height()/2;
          }
        
            player = $(`
              <div id="player-wrapper-${current_uid}">
                <div style="height:${player_area_height}px;width:100%;" id="player-${current_uid}" class="player"></div>
              </div>
            `);

             $("#playerzmblbkjP").append(player);
             user.videoTrack.play(`player-${current_uid}`);
            $(".agora_video_player").css({"object-fit":"contain"});
        }
        
    }
    
  }
  if (mediaType === "audio") {
    user.audioTrack.play();
  }
}

