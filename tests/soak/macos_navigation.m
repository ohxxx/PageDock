// [INPUT]: Two loopback origins, cycle count, and optional development/pool switches.
// [OUTPUT]: Native navigation progress or an exact timed-out URL, without Go/MyGo/PageDock.
// [POS]: Standalone WebKit navigation diagnosis; uses a nonpersistent website data store.
// [PROTOCOL]: Update tests/soak/AGENTS.md; keep binaries and logs in .scratch/.
#import <AppKit/AppKit.h>
#import <WebKit/WebKit.h>
#include <sys/resource.h>

@interface NavigationProbe : NSObject <WKNavigationDelegate>
@property(nonatomic, strong) WKWebView *web;
@property(nonatomic, strong) WKProcessPool *pool;
@property(nonatomic, strong) WKWebsiteDataStore *store;
@property(nonatomic, strong) NSWindow *window;
@property(nonatomic, copy) NSString *source;
@property(nonatomic, copy) NSString *foreign;
@property(nonatomic, copy) NSString *pendingURL;
@property(nonatomic) NSUInteger cycle, visit, target, navigation;
@property(nonatomic) BOOL development, separatePools, noAnimations;
@end

@implementation NavigationProbe
- (void)startCycle {
    @autoreleasepool {
        WKWebViewConfiguration *config = [WKWebViewConfiguration new];
        config.websiteDataStore = self.store;
        if (!self.separatePools) config.processPool = self.pool;
        [config.preferences setValue:@(self.development) forKey:@"developerExtrasEnabled"];
        self.web = [[WKWebView alloc] initWithFrame:NSMakeRect(0, 0, 400, 300) configuration:config];
        self.web.inspectable = self.development;
        self.web.navigationDelegate = self;
        self.window = [[NSWindow alloc] initWithContentRect:NSMakeRect(0, 0, 400, 300)
            styleMask:NSWindowStyleMaskTitled backing:NSBackingStoreBuffered defer:NO];
        self.window.releasedWhenClosed = NO;
        if (self.noAnimations) self.window.animationBehavior = NSWindowAnimationBehaviorNone;
        self.window.title = @"PageDock isolated native navigation probe";
        self.window.contentView = self.web;
        [self.window orderFrontRegardless];
        self.visit = 0;
        [self navigate];
    }
}
- (void)navigate {
    self.pendingURL = [NSString stringWithFormat:@"%@/cycle-%lu/page-%lu", self.visit < 3 ? self.source : self.foreign, self.cycle, self.visit];
    NSUInteger navigation = ++self.navigation;
    [self.web loadRequest:[NSURLRequest requestWithURL:[NSURL URLWithString:self.pendingURL]]];
    dispatch_after(dispatch_time(DISPATCH_TIME_NOW, 20 * NSEC_PER_SEC), dispatch_get_main_queue(), ^{
        if (navigation != self.navigation || self.web == nil) return;
        fprintf(stderr, "timeout cycle=%lu visit=%lu target=%s native=%s loading=%d\n",
            self.cycle, self.visit, self.pendingURL.UTF8String, self.web.URL.absoluteString.UTF8String, self.web.loading);
        fflush(stderr);
        // Let the external runner collect stacks and descriptors before exiting.
        dispatch_after(dispatch_time(DISPATCH_TIME_NOW, 10 * NSEC_PER_SEC), dispatch_get_main_queue(), ^{ exit(1); });
    });
}
- (void)webView:(WKWebView *)webView didFinishNavigation:(WKNavigation *)navigation {
    @autoreleasepool {
        if (++self.visit < 4) { [self navigate]; return; }
        webView.navigationDelegate = nil;
        [webView stopLoading];
        [self.window close];
        self.window.contentView = nil;
        self.window = nil;
        self.web = nil;
        self.cycle++;
        printf("cycles=%lu navigations=%lu\n", self.cycle, self.navigation);
        fflush(stdout);
        if (self.cycle == self.target) exit(0);
        dispatch_after(dispatch_time(DISPATCH_TIME_NOW, NSEC_PER_SEC / 2), dispatch_get_main_queue(), ^{ [self startCycle]; });
    }
}
- (void)webView:(WKWebView *)webView didFailProvisionalNavigation:(WKNavigation *)navigation withError:(NSError *)error {
    fprintf(stderr, "%s\n", error.description.UTF8String);
    exit(2);
}
@end

int main(int argc, const char *argv[]) {
    @autoreleasepool {
        if (argc < 4) return 2;
        [NSApplication sharedApplication];
        struct rlimit files;
        getrlimit(RLIMIT_NOFILE, &files);
        fprintf(stderr, "pid=%d nofile_soft=%llu nofile_hard=%llu\n", getpid(), files.rlim_cur, files.rlim_max);
        NavigationProbe *probe = [NavigationProbe new];
        probe.source = @(argv[1]);
        probe.foreign = @(argv[2]);
        for (NSString *origin in @[probe.source, probe.foreign]) {
            NSURL *url = [NSURL URLWithString:origin];
            if (![url.scheme isEqual:@"http"] || ![url.host isEqual:@"127.0.0.1"] || url.user || url.password) return 2;
        }
        probe.target = (NSUInteger)strtoul(argv[3], NULL, 10);
        if (!probe.target || probe.target > 1000) return 2;
        probe.development = argc > 4 && strcmp(argv[4], "development") == 0;
        probe.separatePools = argc > 5 && strcmp(argv[5], "separate") == 0;
        probe.noAnimations = argc > 6 && strcmp(argv[6], "no-animations") == 0;
        probe.pool = [WKProcessPool new];
        probe.store = WKWebsiteDataStore.nonPersistentDataStore;
        dispatch_async(dispatch_get_main_queue(), ^{ [probe startCycle]; });
        [NSApp run];
    }
    return 2;
}
