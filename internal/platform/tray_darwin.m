// A minimal, from-scratch NSStatusItem tray. Deliberately not using
// getlantern/systray here (used for Windows in tray_windows.go): its darwin
// backend declares its own @interface/@implementation AppDelegate and calls
// [NSApplication sharedApplication] to run its own app/delegate lifecycle —
// which collides (literally, as a duplicate Objective-C class at link time)
// with the AppDelegate Wails v2 already installs to run this app's window.
// This file only ever attaches a status item to the status bar of the
// already-running NSApplication that Wails owns; it never creates its own
// app, delegate or run loop.
#import <Cocoa/Cocoa.h>
#include "tray_darwin.h"
#include "_cgo_export.h"

@interface KarinaTrayTarget : NSObject
- (void)onMenuAction:(id)sender;
@end

@implementation KarinaTrayTarget
- (void)onMenuAction:(id)sender {
  NSMenuItem *item = (NSMenuItem *)sender;
  karinaMenuAction((int)item.tag);
}
@end

static NSStatusItem *karinaStatusItem;
static KarinaTrayTarget *karinaTarget;

static void karinaAddItem(NSMenu *menu, NSString *title, int tag) {
  NSMenuItem *item = [menu addItemWithTitle:title action:@selector(onMenuAction:) keyEquivalent:@""];
  item.target = karinaTarget;
  item.tag = tag;
}

void karinaStartTray(const unsigned char *iconBytes, int iconLen) {
  NSData *iconData = iconLen > 0 ? [NSData dataWithBytes:iconBytes length:iconLen] : nil;

  dispatch_async(dispatch_get_main_queue(), ^{
    karinaTarget = [[KarinaTrayTarget alloc] init];

    karinaStatusItem = [[NSStatusBar systemStatusBar] statusItemWithLength:NSVariableStatusItemLength];
    if (iconData) {
      NSImage *image = [[NSImage alloc] initWithData:iconData];
      image.size = NSMakeSize(18, 18);
      karinaStatusItem.button.image = image;
    }
    karinaStatusItem.button.toolTip = @"Karina · monitor de uso de IA";

    NSMenu *menu = [[NSMenu alloc] init];
    karinaAddItem(menu, @"Abrir Karina", 0);
    karinaAddItem(menu, @"Ocultar a la bandeja", 1);
    [menu addItem:[NSMenuItem separatorItem]];
    karinaAddItem(menu, @"Actualizar ahora", 2);
    karinaAddItem(menu, @"Modo widget (mini)", 3);
    [menu addItem:[NSMenuItem separatorItem]];
    karinaAddItem(menu, @"Salir", 4);
    karinaStatusItem.menu = menu;
  });
}

void karinaStopTray(void) {
  dispatch_async(dispatch_get_main_queue(), ^{
    if (karinaStatusItem) {
      [[NSStatusBar systemStatusBar] removeStatusItem:karinaStatusItem];
      karinaStatusItem = nil;
    }
    karinaTarget = nil;
  });
}

void karinaSetTooltip(const char *text) {
  NSString *tip = [NSString stringWithUTF8String:text];
  dispatch_async(dispatch_get_main_queue(), ^{
    karinaStatusItem.button.toolTip = tip;
  });
}
