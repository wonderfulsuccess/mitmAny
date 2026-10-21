//go:build darwin && cgo

package window

/*
#cgo CFLAGS: -x objective-c -fblocks
#cgo LDFLAGS: -framework Cocoa -framework QuartzCore
#import <Cocoa/Cocoa.h>
#import <QuartzCore/QuartzCore.h>

static void mitmany_round_window(void) {
 dispatch_async(dispatch_get_main_queue(), ^{
    NSWindow *window = NSApp.mainWindow;
    if (window == nil) {
        for (NSWindow *candidate in NSApp.windows) {
            if ([NSStringFromClass(candidate.class) isEqualToString:@"WailsWindow"]) { window = candidate; break; }
        }
    }
    if (window == nil) return;
    window.opaque = NO;
    window.backgroundColor = NSColor.clearColor;
    window.hasShadow = YES;
    NSView *content = window.contentView;
    content.wantsLayer = YES;
    content.layer.cornerRadius = (window.styleMask & NSWindowStyleMaskFullScreen) ? 0 : 12;
    content.layer.masksToBounds = YES;
    [window invalidateShadow];
    static BOOL observing = NO;
    if (!observing) {
        observing = YES;
        for (NSNotificationName name in @[NSWindowDidEnterFullScreenNotification, NSWindowDidExitFullScreenNotification]) {
            [NSNotificationCenter.defaultCenter addObserverForName:name object:window queue:NSOperationQueue.mainQueue usingBlock:^(NSNotification *notification) {
                NSWindow *changed = notification.object;
                changed.contentView.layer.cornerRadius = (changed.styleMask & NSWindowStyleMaskFullScreen) ? 0 : 12;
            }];
        }
    }
 });
}
*/
import "C"

func Round() { C.mitmany_round_window() }
