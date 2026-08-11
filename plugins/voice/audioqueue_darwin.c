//go:build darwin && cgo

#include "audioqueue_darwin.h"

#include <AudioToolbox/AudioToolbox.h>
#include <CoreAudio/CoreAudio.h>
#include <CoreAudio/CoreAudioTypes.h>
#include <pthread.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

#define JT_BUFFER_COUNT 3
#define JT_BUFFER_BYTES 4096

// kAudioObjectPropertyElementMain was introduced in macOS 10.15 (SDK 10.15).
// On older SDKs, use kAudioObjectPropertyElementMaster.
#ifndef kAudioObjectPropertyElementMain
#define kAudioObjectPropertyElementMain kAudioObjectPropertyElementMaster
#endif

struct jt_audio_recorder {
	AudioQueueRef queue;
	AudioQueueBufferRef buffers[JT_BUFFER_COUNT];
	int write_fd;
	pthread_mutex_t mu;
	int running;
};

static void jt_audio_cb(void *user_data, AudioQueueRef queue, AudioQueueBufferRef buffer,
				const AudioTimeStamp *start_time, UInt32 packet_count,
				const AudioStreamPacketDescription *packet_desc) {
	(void)start_time;
	(void)packet_count;
	(void)packet_desc;
	jt_audio_recorder_t *rec = (jt_audio_recorder_t *)user_data;

	pthread_mutex_lock(&rec->mu);
	int running = rec->running;
	int fd = rec->write_fd;
	pthread_mutex_unlock(&rec->mu);

	if (running && fd >= 0 && buffer->mAudioDataByteSize > 0) {
		const char *p = (const char *)buffer->mAudioData;
		UInt32 left = buffer->mAudioDataByteSize;
		while (left > 0) {
			ssize_t n = write(fd, p, left);
			if (n <= 0) break;
			p += n;
			left -= (UInt32)n;
		}
	}

	if (running) {
		AudioQueueEnqueueBuffer(queue, buffer, 0, NULL);
	}
}

int jt_audio_start(int write_fd, const char *device_uid, jt_audio_recorder_t **out_recorder) {
	if (out_recorder == NULL) return -1;
	*out_recorder = NULL;

	jt_audio_recorder_t *rec = (jt_audio_recorder_t *)calloc(1, sizeof(jt_audio_recorder_t));
	if (rec == NULL) return -1;
	rec->write_fd = write_fd;
	rec->running = 1;
	pthread_mutex_init(&rec->mu, NULL);

	AudioStreamBasicDescription fmt;
	memset(&fmt, 0, sizeof(fmt));
	fmt.mSampleRate = 16000.0;
	fmt.mFormatID = kAudioFormatLinearPCM;
	fmt.mFormatFlags = kLinearPCMFormatFlagIsSignedInteger | kLinearPCMFormatFlagIsPacked;
	fmt.mBitsPerChannel = 16;
	fmt.mChannelsPerFrame = 1;
	fmt.mFramesPerPacket = 1;
	fmt.mBytesPerFrame = 2;
	fmt.mBytesPerPacket = 2;

	OSStatus err = AudioQueueNewInput(&fmt, jt_audio_cb, rec, NULL, kCFRunLoopCommonModes, 0, &rec->queue);
	if (err != noErr) {
		pthread_mutex_destroy(&rec->mu);
		free(rec);
		return (int)err;
	}

	// Select a specific audio input device if requested.
	if (device_uid != NULL && device_uid[0] != '\0') {
		CFStringRef uid = CFStringCreateWithCString(kCFAllocatorDefault, device_uid, kCFStringEncodingUTF8);
		if (uid != NULL) {
			err = AudioQueueSetProperty(rec->queue, kAudioQueueProperty_CurrentDevice, &uid, sizeof(uid));
			CFRelease(uid);
			if (err != noErr) {
				AudioQueueDispose(rec->queue, true);
				pthread_mutex_destroy(&rec->mu);
				free(rec);
				return (int)err;
			}
		}
	}

	for (int i = 0; i < JT_BUFFER_COUNT; i++) {
		err = AudioQueueAllocateBuffer(rec->queue, JT_BUFFER_BYTES, &rec->buffers[i]);
		if (err != noErr) {
			AudioQueueDispose(rec->queue, true);
			pthread_mutex_destroy(&rec->mu);
			free(rec);
			return (int)err;
		}
		err = AudioQueueEnqueueBuffer(rec->queue, rec->buffers[i], 0, NULL);
		if (err != noErr) {
			AudioQueueDispose(rec->queue, true);
			pthread_mutex_destroy(&rec->mu);
			free(rec);
			return (int)err;
		}
	}

	err = AudioQueueStart(rec->queue, NULL);
	if (err != noErr) {
		AudioQueueDispose(rec->queue, true);
		pthread_mutex_destroy(&rec->mu);
		free(rec);
		return (int)err;
	}

	*out_recorder = rec;
	return 0;
}

void jt_audio_stop(jt_audio_recorder_t *rec) {
	if (rec == NULL) return;
	pthread_mutex_lock(&rec->mu);
	rec->running = 0;
	int fd = rec->write_fd;
	rec->write_fd = -1;
	pthread_mutex_unlock(&rec->mu);

	if (rec->queue != NULL) {
		AudioQueueStop(rec->queue, true);
		AudioQueueDispose(rec->queue, true);
	}
	if (fd >= 0) {
		close(fd);
	}
	pthread_mutex_destroy(&rec->mu);
	free(rec);
}

// ---- Device enumeration ----

char *jt_audio_list_devices(void) {
	// Query all audio devices
	AudioObjectPropertyAddress addr = {
		kAudioHardwarePropertyDevices,
		kAudioObjectPropertyScopeGlobal,
		kAudioObjectPropertyElementMain
	};
	UInt32 dataSize = 0;
	OSStatus err = AudioObjectGetPropertyDataSize(kAudioObjectSystemObject, &addr, 0, NULL, &dataSize);
	if (err != noErr) return NULL;

	UInt32 deviceCount = dataSize / sizeof(AudioDeviceID);
	AudioDeviceID *devices = (AudioDeviceID *)malloc(dataSize);
	if (devices == NULL) return NULL;

	err = AudioObjectGetPropertyData(kAudioObjectSystemObject, &addr, 0, NULL, &dataSize, devices);
	if (err != noErr) {
		free(devices);
		return NULL;
	}

	// Build output buffer
	size_t cap = 4096;
	size_t len = 0;
	char *buf = (char *)malloc(cap);
	if (buf == NULL) {
		free(devices);
		return NULL;
	}
	buf[0] = '\0';

	// Output one line per device: "display-name\tuid\n"
	// This is easy to parse on the Go side with strings.SplitN.
	UInt32 deviceMetaSize = 0;
	AudioObjectPropertyAddress hasInputAddr = {
		kAudioDevicePropertyStreams,
		kAudioDevicePropertyScopeInput,
		kAudioObjectPropertyElementMain
	};

	for (UInt32 i = 0; i < deviceCount; i++) {
		AudioDeviceID dev = devices[i];

		// Check if device has input streams
		AudioObjectGetPropertyDataSize(dev, &hasInputAddr, 0, NULL, &deviceMetaSize);
		if (deviceMetaSize == 0) continue; // no input, skip

		// Get device name
		CFStringRef nameRef = NULL;
		AudioObjectPropertyAddress nameAddr = {
			kAudioDevicePropertyDeviceNameCFString,
			kAudioObjectPropertyScopeGlobal,
			kAudioObjectPropertyElementMain
		};
		UInt32 nameSize = sizeof(CFStringRef);
		err = AudioObjectGetPropertyData(dev, &nameAddr, 0, NULL, &nameSize, &nameRef);
		if (err != noErr) continue;

		// Get device UID
		CFStringRef uidRef = NULL;
		AudioObjectPropertyAddress uidAddr = {
			kAudioDevicePropertyDeviceUID,
			kAudioObjectPropertyScopeGlobal,
			kAudioObjectPropertyElementMain
		};
		UInt32 uidSize = sizeof(CFStringRef);
		err = AudioObjectGetPropertyData(dev, &uidAddr, 0, NULL, &uidSize, &uidRef);
		if (err != noErr) {
			CFRelease(nameRef);
			continue;
		}

		// Convert to C strings
		CFIndex nameLen = CFStringGetMaximumSizeForEncoding(CFStringGetLength(nameRef), kCFStringEncodingUTF8) + 1;
		char *nameStr = (char *)malloc(nameLen);
		if (nameStr == NULL) {
			CFRelease(nameRef); CFRelease(uidRef);
			continue;
		}
		CFStringGetCString(nameRef, nameStr, nameLen, kCFStringEncodingUTF8);

		CFIndex uidLen = CFStringGetMaximumSizeForEncoding(CFStringGetLength(uidRef), kCFStringEncodingUTF8) + 1;
		char *uidStr = (char *)malloc(uidLen);
		if (uidStr == NULL) {
			free(nameStr); CFRelease(nameRef); CFRelease(uidRef);
			continue;
		}
		CFStringGetCString(uidRef, uidStr, uidLen, kCFStringEncodingUTF8);

		CFRelease(nameRef);
		CFRelease(uidRef);

		// Write line: display-name\tuid\n
		// We need to ensure the buffer is large enough
		// Estimate that each line needs roughly: name_len + uid_len + 3
		size_t needed = len + strlen(nameStr) + strlen(uidStr) + 3;
		if (needed >= cap) {
			cap = needed + 512;
			char *newBuf = (char *)realloc(buf, cap);
			if (newBuf == NULL) {
				free(nameStr); free(uidStr); free(buf); free(devices);
				return NULL;
			}
			buf = newBuf;
		}

		int n = snprintf(buf + len, cap - len, "%s\t%s\n", nameStr, uidStr);
		if (n > 0) len += n;

		free(nameStr);
		free(uidStr);
	}

	free(devices);
	return buf;
}