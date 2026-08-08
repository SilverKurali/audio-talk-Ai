//go:build darwin && cgo

// Objective-C implementation of microphone permission check for macOS 10.14+.
// Compiled as part of the doctor package when building on darwin.

#import <AVFoundation/AVFoundation.h>

// Returns the current microphone authorization status:
//   0 = AVAuthorizationStatusNotDetermined
//   1 = AVAuthorizationStatusRestricted
//   2 = AVAuthorizationStatusDenied
//   3 = AVAuthorizationStatusAuthorized
// On macOS < 10.14, returns 3 (assumed granted).
int doctor_mic_permission(void) {
    // Guard against older macOS versions where AVFoundation may not be available.
    if (@available(macOS 10.14, *)) {
        AVAuthorizationStatus status = [AVCaptureDevice authorizationStatusForMediaType:AVMediaTypeAudio];
        return (int)status;
    }
    return 3;
}