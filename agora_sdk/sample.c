#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

#include "include_c/api2/agora_local_user.h"
#include "include_c/api2/agora_rtc_conn.h"
#include "include_c/api2/agora_service.h"
#include "include_c/base/agora_media_base.h"
//gcc sample.c -I include_c/base/ -lagora_rtc_sdk -L . -lagora-fdkaac   -g
int onPlaybackAudioFrameBeforeMixing(AGORA_HANDLE agora_local_user, uid_t uid, const audio_frame* frame){
   printf("get a frame\n");
   return 0;
}

int main() {
  agora_service_config service_conf;
  memset(&service_conf, 0, sizeof(agora_service_config));
  service_conf.app_id = "aab8b8f5a8cd4469a63042fcfafe7063";
  service_conf.enable_audio_device = 0;
  service_conf.enable_audio_processor = 1;
  service_conf.enable_video = 1;
  void* service_handle = agora_service_create();
  agora_service_initialize(service_handle, &service_conf);
  agora_service_set_log_file(service_handle, "./io.agora.rtc_sdk/agorasdk.log",
                             512 * 1024);

  rtc_conn_config con_config;
  memset(&con_config, 0, sizeof(rtc_conn_config));
  con_config.auto_subscribe_audio = 1;
  con_config.auto_subscribe_video = 0;
  con_config.client_role_type = 1;
  con_config.channel_profile = 1;

  void* conn_handle = agora_rtc_conn_create(service_handle, &con_config);
  rtc_conn_observer conn_observer;
  memset(&conn_observer, 0, sizeof(rtc_conn_observer));
  agora_rtc_conn_register_observer(conn_handle, &conn_observer);

  void* media_node_factory =
      agora_service_create_media_node_factory(service_handle);
  void* audio_pcm_sender_handle =
      agora_media_node_factory_create_audio_pcm_data_sender(media_node_factory);
  void* audio_pcm_track_handle = agora_service_create_custom_audio_track_pcm(
      service_handle, audio_pcm_sender_handle);
  void* local_user_handle = agora_rtc_conn_get_local_user(conn_handle);

agora_local_user_subscribe_all_audio(local_user_handle);
    audio_frame_observer  audio_data_observer;
  memset(&audio_data_observer, 0, sizeof(audio_frame_observer));
audio_data_observer.on_playback_audio_frame_before_mixing = onPlaybackAudioFrameBeforeMixing;
int re = agora_local_user_set_playback_audio_frame_before_mixing_parameters(local_user_handle,1,16000);

printf("it is %d \n",re);
  int r = agora_rtc_conn_connect(
      conn_handle, "aab8b8f5a8cd4469a63042fcfafe7063", "test123", "0");
  agora_local_user_publish_audio(local_user_handle, audio_pcm_track_handle);

  static FILE* file = NULL;
  file = fopen("demo.pcm", "rb");

  char frameBuf[320];
    agora_local_user_register_audio_frame_observer(local_user_handle,&audio_data_observer);

  for (int i = 0; i < 500; i++) {
    fread(frameBuf, 1, sizeof(frameBuf), file);

    agora_audio_pcm_data_sender_send(audio_pcm_sender_handle, frameBuf, 0, 160,
                                     2, 1, 16000);

    //    SendPcmAudio(frameBuf, 320);
    usleep(10 * 1000);

  }

  sleep(10);

  return 0;
}