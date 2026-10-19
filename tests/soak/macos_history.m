// [INPUT]: System AppKit/WebKit and a requested number of isolated data-URL navigation cycles.
// [OUTPUT]: JSON describing native history ownership after WebViews close; nonzero on retained objects.
// [POS]: Standalone diagnostic without Go/MyGo/PageDock, for WebKit bug 325374.
// [PROTOCOL]: Update tests/soak/AGENTS.md; keep binaries and raw results in .scratch/.
#import <AppKit/AppKit.h>
#import <WebKit/WebKit.h>

@interface HistoryProbe : NSObject <WKNavigationDelegate>
@property(nonatomic, strong) WKWebView *web;
@property(nonatomic, strong) WKProcessPool *pool;
@property(nonatomic, strong) NSHashTable *history;
@property(nonatomic, strong) NSHashTable *views;
@property(nonatomic) NSUInteger cycles;
@property(nonatomic) NSUInteger target;
@property(nonatomic) NSUInteger visit;
@property(nonatomic) CFIndex extraRetains;
@end

@implementation HistoryProbe
- (void)startCycle {
    @autoreleasepool {
        WKWebViewConfiguration *config = [WKWebViewConfiguration new];
        config.processPool = self.pool;
        self.web = [[WKWebView alloc] initWithFrame:NSMakeRect(0, 0, 400, 300) configuration:config];
        self.web.navigationDelegate = self;
        [self.views addObject:self.web];
        self.visit = 0;
        [self navigate];
    }
}
- (void)navigate {
    NSString *url = [NSString stringWithFormat:@"data:text/html,cycle-%lu-page-%lu", self.cycles, self.visit];
    [self.web loadRequest:[NSURLRequest requestWithURL:[NSURL URLWithString:url]]];
}
- (void)webView:(WKWebView *)webView didFinishNavigation:(WKNavigation *)navigation {
    @autoreleasepool {
        if (++self.visit < 3) {
            [self navigate];
            return;
        }
        for (WKBackForwardListItem *item in webView.backForwardList.backList) {
            [self.history addObject:item];
        }
        if (self.cycles == 0) {
            WKBackForwardListItem *item = webView.backForwardList.currentItem;
            CFIndex before = CFGetRetainCount((__bridge CFTypeRef)item);
            for (NSUInteger index = 0; index < 10; index++) {
                @autoreleasepool {
                    if (webView.backForwardList.currentItem != item) {
                        fprintf(stderr, "unexpected history identity\n");
                        exit(2);
                    }
                }
            }
            self.extraRetains = CFGetRetainCount((__bridge CFTypeRef)item) - before;
        }
        webView.navigationDelegate = nil;
        [webView stopLoading];
        self.web = nil;
        self.cycles++;
        BOOL finished = self.cycles == self.target;
        dispatch_after(dispatch_time(DISPATCH_TIME_NOW, (int64_t)((finished ? 5.0 : 0.05) * NSEC_PER_SEC)), dispatch_get_main_queue(), ^{
            if (finished) {
                [self report];
            } else {
                [self startCycle];
            }
        });
    }
}
- (void)webView:(WKWebView *)webView didFailProvisionalNavigation:(WKNavigation *)navigation withError:(NSError *)error {
    fprintf(stderr, "%s\n", error.description.UTF8String);
    exit(2);
}
- (void)report {
    @autoreleasepool {
        NSDictionary *result = @{@"cycles": @(self.cycles), @"retained_views": @(self.views.allObjects.count),
            @"retained_history": @(self.history.allObjects.count), @"extra_retains_for_10_reads": @(self.extraRetains),
            @"os": NSProcessInfo.processInfo.operatingSystemVersionString};
        NSData *json = [NSJSONSerialization dataWithJSONObject:result options:0 error:nil];
        fwrite(json.bytes, 1, json.length, stdout);
        printf("\n");
        exit(self.history.allObjects.count != 0 || self.extraRetains != 0 ? 1 : 0);
    }
}
@end

int main(int argc, const char *argv[]) {
    @autoreleasepool {
        NSUInteger cycles = argc > 1 ? (NSUInteger)strtoul(argv[1], NULL, 10) : 30;
        if (!cycles || cycles > 1000) return 2;
        [NSApplication sharedApplication];
        HistoryProbe *probe = [HistoryProbe new];
        probe.pool = [WKProcessPool new];
        probe.history = [NSHashTable weakObjectsHashTable];
        probe.views = [NSHashTable weakObjectsHashTable];
        probe.target = cycles;
        dispatch_async(dispatch_get_main_queue(), ^{ [probe startCycle]; });
        dispatch_after(dispatch_time(DISPATCH_TIME_NOW, 60 * NSEC_PER_SEC), dispatch_get_main_queue(), ^{
            fprintf(stderr, "native history diagnostic timed out\n");
            exit(2);
        });
        [NSApp run];
    }
    return 2;
}
