//go:build !linux

package wireguard

import "fmt"

func SetupInterface(iface, subnet string, port int, privateKeyB64 string) error {
	return fmt.Errorf("SetupInterface not supported on this platform")
}

func TeardownInterface(iface string) error {
	return fmt.Errorf("TeardownInterface not supported on this platform")
}
