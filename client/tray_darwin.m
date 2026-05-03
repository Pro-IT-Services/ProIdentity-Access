#import <Foundation/Foundation.h>
#import <dispatch/dispatch.h>

// Implemented in tray_darwin.go via //export
extern void trayStartCallback(void);

// Called from Go setupTray() — dispatches trayStartCallback() to the macOS
// main queue so nativeStart() runs on the correct thread for AppKit.
void dispatchTrayStart(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        trayStartCallback();
    });
}
