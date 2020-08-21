package wireguard

import (
	"fmt"
	"net"
	"os/exec"
	"vpc/pkg/utils"
)

var routedInterfaceName string
func cmdDeleteDevLink(_interface string) *exec.Cmd {
	return exec.Command("sh", "-c", fmt.Sprintf("ip link del dev %s", _interface))
}

func cmdSetNATRouting(_interface string) *exec.Cmd {
	var err error
	rifs, err := utils.RoutedInterface("IP", net.FlagUp|net.FlagBroadcast)
	if err != nil {
		panic(err)
	}
	routedInterfaceName = rifs.Name
	ipt := fmt.Sprintf(`iptables -A FORWARD -i %s -j ACCEPT; iptables -A FORWARD -o %s -j ACCEPT; iptables -t nat -A POSTROUTING -o %s -j MASQUERADE`,
		_interface, _interface, routedInterfaceName)
	return exec.Command("sh", "-c", ipt)
}

func cmdDelNATRouting(_interface string) *exec.Cmd {
	ipt := fmt.Sprintf(`iptables -A FORWARD -i %s -j ACCEPT; iptables -A FORWARD -o %s -j ACCEPT; iptables -t nat -A POSTROUTING -o %s -j MASQUERADE`,
		_interface, _interface, routedInterfaceName)
	return exec.Command("sh", "-c", ipt)
}
