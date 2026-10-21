//go:build darwin && cgo

package window

/*
#cgo CFLAGS: -x objective-c -fblocks
#cgo LDFLAGS: -framework Cocoa
#import <Cocoa/Cocoa.h>

static void mitmany_window_ready(void) {
 dispatch_async(dispatch_get_main_queue(), ^{
    NSWindow *window = nil;
    for (NSWindow *candidate in NSApp.windows) {
        if ([NSStringFromClass(candidate.class) isEqualToString:@"WailsWindow"]) { window = candidate; break; }
    }
    if (window == nil) return;
    // Keep the existing HTML traffic lights; AppKit owns the window shape and shadow.
    for (NSNumber *type in @[@(NSWindowCloseButton), @(NSWindowMiniaturizeButton), @(NSWindowZoomButton)]) {
        [window standardWindowButton:type.unsignedIntegerValue].hidden = YES;
    }
    NSString *name = @"mitmAny.MainWindow.v1";
    [window setFrameUsingName:name];
    [window setFrameAutosaveName:name];
    // AppKit also adjusts restored frames when the connected displays have changed.
    __block BOOL transitioning = NO;
    NSNotificationCenter *center = NSNotificationCenter.defaultCenter;
    for (NSNotificationName notification in @[NSWindowWillEnterFullScreenNotification, NSWindowWillExitFullScreenNotification]) {
        [center addObserverForName:notification object:window queue:NSOperationQueue.mainQueue usingBlock:^(NSNotification *event) { transitioning = YES; }];
    }
    for (NSNotificationName notification in @[NSWindowDidEnterFullScreenNotification, NSWindowDidExitFullScreenNotification]) {
        [center addObserverForName:notification object:window queue:NSOperationQueue.mainQueue usingBlock:^(NSNotification *event) { transitioning = NO; }];
    }
    for (NSNotificationName notification in @[NSWindowDidMoveNotification, NSWindowDidResizeNotification, NSWindowWillCloseNotification]) {
        [center addObserverForName:notification object:window queue:NSOperationQueue.mainQueue usingBlock:^(NSNotification *event) {
            if (!transitioning && !(window.styleMask & NSWindowStyleMaskFullScreen) && !window.isMiniaturized) {
                [window saveFrameUsingName:name];
            }
        }];
    }
    // No visible borderless frame: restoration finishes before the first show.
    [window makeKeyAndOrderFront:nil];
    [window saveFrameUsingName:name];
 });
}
*/
import "C"
import "context"

func Ready(_ context.Context) { C.mitmany_window_ready() }
