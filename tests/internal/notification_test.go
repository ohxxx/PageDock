// [INPUT]: Website notification contents, origins, and tags.
// [OUTPUT]: Content sanitization, Unicode truncation, independent notifications, and origin isolation tests.
// [POS]: In-package behavior tests in tests/, compiled into internal through Go overlay.
// [PROTOCOL]: Update tests/internal/AGENTS.md and the notification implementation.
package pagedock

import (
	"strings"
	"testing"
)

func TestCleanNotificationText(t *testing.T) {
	if got := cleanNotificationText("  标题\x00\r\n正文\t  ", 100); got != "标题\n正文" {
		t.Fatalf("cleaned notification text = %q", got)
	}
	if got := cleanNotificationText("消息内容🙂123", 5); got != "消息内容🙂" {
		t.Fatalf("limited Unicode notification text = %q", got)
	}
}

func TestPWANotificationIDs(t *testing.T) {
	const source = "https://example.com"
	tagged := pwaNotificationID(source, "message-42")
	if tagged != pwaNotificationID(source, "message-42") {
		t.Fatal("same tag did not replace notification")
	}
	if tagged == pwaNotificationID("https://other.example", "message-42") {
		t.Fatal("notification tags crossed origins")
	}
	a, b := pwaNotificationID(source, ""), pwaNotificationID(source, "")
	if a == b || !strings.HasPrefix(a, pwaOriginPrefix(source)) {
		t.Fatalf("invalid untagged IDs: %q, %q", a, b)
	}
	if pwaNotificationID(source, strings.Repeat("x", 80)+"a") == pwaNotificationID(source, strings.Repeat("x", 80)+"b") {
		t.Fatal("long distinct tags collided")
	}
}
