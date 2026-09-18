//go:build !windows

package sing_tun

import (
	"fmt"
	"syscall"
)

// probeFdSocketType reports the socket type of the given fd via getsockopt(SO_TYPE).
// Returns "(n/a)" when fd is not valid. On restricted environments (e.g. OHOS
// sandbox) this surfaces the getsockopt error so TUN attach failures are
// observable instead of silent.
func probeFdSocketType(fd int) string {
	if fd <= 0 {
		return "n/a(no-fd)"
	}
	typ, err := syscall.GetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_TYPE)
	if err != nil {
		return fmt.Sprintf("probe-error(%s)", err)
	}
	return fmt.Sprintf("socket(SO_TYPE=%d)", typ)
}