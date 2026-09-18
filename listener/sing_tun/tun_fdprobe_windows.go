//go:build windows

package sing_tun

func probeFdSocketType(fd int) string {
	return "n/a(windows)"
}