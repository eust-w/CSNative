//go:build darwin

package desktop

/*
#cgo darwin CFLAGS: -x objective-c
#cgo darwin LDFLAGS: -framework Cocoa
#import <Cocoa/Cocoa.h>

static void csnativeSetDockIcon(void *bytes, long length) {
	@autoreleasepool {
		if (bytes == NULL || length <= 0) {
			return;
		}
		NSData *data = [NSData dataWithBytes:bytes length:(NSUInteger)length];
		NSImage *image = [[NSImage alloc] initWithData:data];
		if (image != nil) {
			[NSApp setApplicationIconImage:image];
		}
	}
}
*/
import "C"

import "unsafe"

func setDockIcon(icon []byte) {
	if len(icon) == 0 {
		return
	}
	C.csnativeSetDockIcon(unsafe.Pointer(&icon[0]), C.long(len(icon)))
}
