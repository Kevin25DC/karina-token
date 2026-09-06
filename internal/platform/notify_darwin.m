#import <UserNotifications/UserNotifications.h>

// UNUserNotificationCenter (not the older NSUserNotificationCenter) is used
// here. See notify_darwin.go for why: NSUserNotification, despite needing no
// permission prompt, was found in practice to be silently inert for an
// unsigned, ad-hoc app bundle on current macOS — no error, no crash, the
// banner just never appears. UNUserNotificationCenter is the version Apple
// actually keeps working for third-party apps, signed or not; the tradeoff
// is the one-time "Karina would like to send you notifications" prompt.
void karinaRequestNotificationPermission(void) {
  UNUserNotificationCenter *center = [UNUserNotificationCenter currentNotificationCenter];
  [center requestAuthorizationWithOptions:(UNAuthorizationOptionAlert | UNAuthorizationOptionSound)
                         completionHandler:^(BOOL granted, NSError *_Nullable error) {
    // Fire-and-forget: the OS prompt (or its remembered answer) is the only
    // feedback the user needs. A denial here is a normal, expected outcome,
    // not something Karina can or should react to.
  }];
}

void karinaShowNotification(const char *title, const char *message) {
  NSString *titleStr = [NSString stringWithUTF8String:title];
  NSString *bodyStr = [NSString stringWithUTF8String:message];

  UNUserNotificationCenter *center = [UNUserNotificationCenter currentNotificationCenter];
  [center requestAuthorizationWithOptions:(UNAuthorizationOptionAlert | UNAuthorizationOptionSound)
                         completionHandler:^(BOOL granted, NSError *_Nullable error) {
    if (!granted) {
      return;
    }

    UNMutableNotificationContent *content = [[UNMutableNotificationContent alloc] init];
    content.title = titleStr;
    content.body = bodyStr;
    content.sound = [UNNotificationSound defaultSound];

    UNNotificationRequest *request =
        [UNNotificationRequest requestWithIdentifier:[[NSUUID UUID] UUIDString]
                                              content:content
                                              trigger:nil]; // nil trigger = deliver now
    [center addNotificationRequest:request withCompletionHandler:nil];
  }];
}
