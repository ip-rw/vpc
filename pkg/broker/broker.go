package broker

import (
	"fmt"
	"github.com/Denis101/freeport"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
	"time"
	"vpc/pkg/proxy"
	"vpc/pkg/utils"
	"vpc/pkg/wireguard"
)

var Logger = GetLogger(zap.DebugLevel)

func GetLogger(ll zapcore.Level) *zap.Logger {
	l := zap.New(zapcore.NewCore(
		zapcore.NewJSONEncoder(zap.NewDevelopmentEncoderConfig()),
		zapcore.Lock(os.Stdout),
		ll,
	))
	return l
}

func CreateProxy(dhost string, dport int) (*proxy.Proxy, error) {
	port, err := freeport.GetFreePortForProtocol("udp")
	if err != nil {
		return nil, err
	}

	proxy := proxy.NewProxy(true, Logger, port, "0.0.0.0", dhost, dport, 4096, time.Second, time.Second*30)
	err = proxy.Start()
	if err != nil {
		proxy.Logger.Error("failed to start proxy", zap.Int("source port", port), zap.String("dest", fmt.Sprintf("%s:%d", dhost, dport)))
	}
	return proxy, err
}


func CreateWireguard(pubkey string) (*wireguard.Wireguard, error) {
	port, err := freeport.GetFreePortForProtocol("udp")
	if err != nil {
		return nil, err
	}
	iface := utils.RandomString(6)
	ipnet, err := utils.RandomPrivateNet()
	if err != nil {
		Logger.Error("failed to find subnet", zap.Error(err))
	}
	wg, err := wireguard.NewWireguard(Logger, iface, port, ipnet.FirstAddress(), ipnet.IPNet)
	err = wg.Init()
	if err != nil {
		wg.Logger.Error("failed to init wg", zap.Error(err))
	}

	err = wg.Start()
	if err != nil {
		wg.Logger.Error("failed to start wg", zap.Error(err))
	} else {
		wg.Logger.Debug("successfully started wireguard", zap.Error(err))
	}
	return wg, err
}
