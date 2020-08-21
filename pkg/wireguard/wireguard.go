package wireguard

import (
	"encoding/json"
	"fmt"
	"github.com/my-network/wgcreate"
	hub "github.com/sentinel-official/hub/types"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"log"
	"net"
	"net/http"
	"os"
)

var (
	serverKeysPath = os.TempDir()
	endPoint       string
)

// Bandwidth ...
type Bandwidth struct {
	upload   int64
	download int64
}

// Keys ...
type Keys struct {
	PrivateKey wgtypes.Key
	PublicKey  wgtypes.Key
}

// Wireguard ...
type
Wireguard struct {
	Logger     *zap.Logger
	Client     *wgctrl.Client
	Iface      string
	Port       int
	IP         net.IP
	IPNet      net.IPNet
	PeerConfig string
}

type PublicIP struct {
	IP string `json:"IP"`
}

// NewWireguard ...
func NewWireguard(logger *zap.Logger, iface string, port int, ip net.IP, ipnet net.IPNet) (*Wireguard, error) {
	//func NewWireguard() (*Wireguard, error) {
	client, err := wgctrl.New()
	if err != nil {
		log.Println(err)
		return &Wireguard{}, err
	}
	return &Wireguard{
		Logger: logger.With(zap.String("iface", iface)),
		Client: client,
		Iface:  iface,
		Port:   port,
		IP:     ip,
		IPNet:  ipnet,
	}, nil
}

//func (wg *Wireguard) saveKeys(public, private wgtypes.Key) error {
//	err := ioutil.WriteFile(serverKeysPath+"/privkey", []byte(private.String()), os.ModePerm)
//	if err != nil {
//		return err
//	}
//	err = ioutil.WriteFile(serverKeysPath+"/pubkey", []byte(public.String()), os.ModePerm)
//	if err != nil {
//		return err
//	}
//	return nil
//}

func (wg *Wireguard) Device() *wgtypes.Device {
	dev, err := wg.Client.Device(wg.Iface)
	if err != nil {
		wg.Logger.Error("device", zap.Error(err))
	}
	return dev
}

func (wg *Wireguard) generateKeys() (Keys, error) {
	privateKey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return Keys{}, err
	}
	publicKey := privateKey.PublicKey()
	return Keys{
		PrivateKey: privateKey,
		PublicKey:  publicKey,
	}, nil
}

func (wg *Wireguard) setNATRouting() error {
	cmd := cmdSetNATRouting(wg.Iface)
	o, e := cmd.CombinedOutput()
	fmt.Printf("%s", o)
	return e
}

func (wg *Wireguard) addInterface(mtu uint32) error {
	getLogger := func(level zapcore.Level) *log.Logger {
		l, _ := zap.NewStdLogAt(wg.Logger, level)
		return l
	}
	_, err := wgcreate.Create(wg.Iface, mtu, true, &device.Logger{
		Debug: getLogger(zap.DebugLevel),
		Info:  getLogger(zap.InfoLevel),
		Error: getLogger(zap.ErrorLevel),
	})
	if err != nil {
		return err
	}
	return wgcreate.AddIP(wg.Iface, wg.IP, wg.IPNet);
}

func (wg *Wireguard) addWireGuardDevice() error {
	wg.Logger.Debug("adding wireguard device.")
	err := wg.addInterface(1420)
	if err != nil {
		wg.Logger.Error("error creating interface", zap.Error(err))
	}
	wg.Logger.Debug("created interface", zap.String("ip", wg.IP.String()))
	return err
}

// Init ...
func (wg *Wireguard) Init() error {
	wg.Logger.Info("initializing wireguard")
	if err := wg.addWireGuardDevice(); err != nil {
		return err
	}

	resp, err := http.Get("https://api.ipify.org/?format=json")
	if err != nil {
		return err
	}
	var res PublicIP
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return err
	}
	defer resp.Body.Close()
	endPoint = fmt.Sprintf("%s:%d", res.IP, wg.Port)
	return wg.setNATRouting()
	//return err
}

func (wg *Wireguard) generateConfig() (wgtypes.Config, error) {
	keys, err := wg.generateKeys()
	if err != nil {
		return wgtypes.Config{}, err
	}
	//if err := wg.saveKeys(keys.PublicKey, keys.PrivateKey); err != nil {
	//	return wgtypes.Config{}, err
	//}
	return wgtypes.Config{
		PrivateKey:   &keys.PrivateKey,
		ListenPort:   &wg.Port,
		ReplacePeers: false,
		Peers:        []wgtypes.PeerConfig{},
	}, nil
}

// Start ...
func (wg *Wireguard) Start() error {
	wg.Logger.Info("starting wireguard device")
	cfg, err := wg.generateConfig()
	if err != nil {
		return err
	}
	return wg.Client.ConfigureDevice(wg.Iface, cfg)
}

// Stop ...
func (wg *Wireguard) Stop() error {
	wg.Logger.Info("stopping wireguard device")
	wg.Client.Close()
	var err error
	cmd := cmdDeleteDevLink(wg.Iface)
	o, err := cmd.CombinedOutput();
	//fmt.Printf("%s", o)
	if err != nil {
		wg.Logger.Error("failed to remove interface", zap.Error(err))
	} else {
		wg.Logger.Debug("removed inferface")
	}
	cmd = cmdDelNATRouting(wg.Iface)
	o, err = cmd.CombinedOutput();
	fmt.Printf("%s", o)
	if err != nil {
		wg.Logger.Error("failed to reset iptables", zap.Error(err))
	} else {
		wg.Logger.Debug("reset iptables")
	}
	return err
}

func (wg *Wireguard) generateAllowedIP() ([]net.IPNet, error) {
	var allowedIPs []net.IP
	dev, err := wg.Client.Device(wg.Iface)
	if err != nil {
		return []net.IPNet{}, err
	}
	for _, peer := range dev.Peers {
		allowedIPs = append(allowedIPs, peer.AllowedIPs[0].IP)
	}
	for i := 2; i < 255; i++ {
		ip := net.IPv4(byte(10), byte(7), byte(0), byte(i))
		if !contains(allowedIPs, ip) {
			ipMask := net.IPv4Mask(byte(255), byte(255), byte(255), byte(255))
			return []net.IPNet{{IP: ip, Mask: ipMask}}, nil
		}
	}
	return []net.IPNet{}, fmt.Errorf("server is busy")
}

// GenerateClientKey ...
func (wg *Wireguard) GenerateClientKey() ([]byte, error) {
	wg.Logger.Info("adding peer")
	keys, err := wg.generateKeys()
	if err != nil {
		return []byte{}, err
	}
	availableIP, err := wg.generateAllowedIP()
	if err != nil {
		return []byte{}, err
	}
	peer := wgtypes.PeerConfig{
		PublicKey:  keys.PublicKey,
		AllowedIPs: availableIP,
	}
	cfg := wgtypes.Config{
		ReplacePeers: false,
		Peers:        []wgtypes.PeerConfig{peer},
	}
	err = wg.Client.ConfigureDevice(wg.Iface, cfg)
	if err != nil {
		log.Println("err:", err)
		return []byte(""), err
	}
	dev, _ := wg.Client.Device(wg.Iface)

	allowedIP := fmt.Sprint(peer.AllowedIPs[0].IP)
	clientConfig := fmt.Sprintf(clientConfigTemplate, keys.PrivateKey.String(), allowedIP,
		dev.PublicKey.String(), endPoint)
	//wg.Logger.Info(clientConfig)
	return []byte(clientConfig), nil
}

// ClientsList ...
func (wg *Wireguard) ClientsList() (map[string]hub.Bandwidth, error) {
	// fmt.Print("\nGetting clients usage list ...\n")
	clientsUsageMap := map[string]hub.Bandwidth{}
	wgData, err := wg.Client.Device(wg.Iface)
	if err != nil {
		return clientsUsageMap, err
	}

	for _, peer := range wgData.Peers {
		pubkey := peer.PublicKey
		usage := hub.NewBandwidthFromInt64(peer.ReceiveBytes, peer.TransmitBytes)
		if peer.LastHandshakeTime.Minute() < 3 {
			clientsUsageMap[pubkey.String()] = usage
		} else {
			wg.DisconnectClient(pubkey.String())
		}
	}
	return clientsUsageMap, nil
}

// DisconnectClient ...
func (wg *Wireguard) DisconnectClient(pubkey string) error {
	fmt.Print("Disconnecting client")
	publicKey, err := wgtypes.ParseKey(pubkey)
	if err != nil {
		return err
	}
	peer := wgtypes.PeerConfig{
		PublicKey: publicKey,
		Remove:    true,
	}
	cfg := wgtypes.Config{
		ReplacePeers: false,
		Peers:        []wgtypes.PeerConfig{peer},
	}
	err = wg.Client.ConfigureDevice(wg.Iface, cfg)
	if err != nil {
		log.Println("err:", err)
		return err
	}
	return nil
}

func contains(arr []net.IP, ip net.IP) bool {
	for _, a := range arr {
		if a.String() == ip.String() {
			return true
		}
	}
	return false
}
