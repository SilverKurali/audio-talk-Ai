#pragma once

typedef struct jt_audio_recorder jt_audio_recorder_t;

// device_uid selects a specific audio input device (CoreAudio device UID).
// Pass NULL or "" to use the system default input device.
int jt_audio_start(int write_fd, const char *device_uid, jt_audio_recorder_t **out_recorder);
void jt_audio_stop(jt_audio_recorder_t *recorder);

// Returns a malloc'd NUL-terminated string listing input devices, one per line
// in the form "display-name\tuid\n". The caller must free() the result.
// Returns NULL on error.
char *jt_audio_list_devices(void);
