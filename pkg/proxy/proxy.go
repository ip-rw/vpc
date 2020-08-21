package proxy

import (
	"fmt"
	"net"
	"sync"
	"time"

	"go.uber.org/zap"
)

type connection struct {
	udp          *net.UDPConn
	lastActivity time.Time
}

type packet struct {
	src  *net.UDPAddr
	data []byte
}

type Proxy struct {
	Logger                 *zap.Logger
	BindPort               int
	BindAddress            string
	UpstreamAddress        string
	UpstreamPort           int
	Debug                  bool
	listenerConn           *net.UDPConn
	client                 *net.UDPAddr
	upstream               *net.UDPAddr
	BufferSize             int
	ConnTimeout            time.Duration
	ResolveTTL             time.Duration
	connsMap               map[string]connection
	connectionsLock        *sync.RWMutex
	closed                 bool
	clientMessageChannel   chan (packet)
	upstreamMessageChannel chan (packet)
}

func NewProxy(debug bool, logger *zap.Logger, bindPort int, bindAddress string, upstreamAddress string, upstreamPort int, bufferSize int, connTimeout time.Duration, resolveTTL time.Duration) *Proxy {
	proxy := &Proxy{
		Debug:                  debug,
		Logger:                 logger,
		BindPort:               bindPort,
		BindAddress:            bindAddress,
		BufferSize:             bufferSize,
		ConnTimeout:            connTimeout,
		UpstreamAddress:        upstreamAddress,
		UpstreamPort:           upstreamPort,
		connectionsLock:        new(sync.RWMutex),
		connsMap:               make(map[string]connection),
		closed:                 false,
		ResolveTTL:             resolveTTL,
		clientMessageChannel:   make(chan packet),
		upstreamMessageChannel: make(chan packet),
	}

	return proxy
}

func (p *Proxy) updateClientLastActivity(clientAddrString string) {
	p.Logger.Debug("updating client last activity", zap.String("client", clientAddrString))
	p.connectionsLock.Lock()
	if _, found := p.connsMap[clientAddrString]; found {
		connWrapper := p.connsMap[clientAddrString]
		connWrapper.lastActivity = time.Now()
		p.connsMap[clientAddrString] = connWrapper
	}
	p.connectionsLock.Unlock()
}

func (p *Proxy) clientConnectionReadLoop(clientAddr *net.UDPAddr, upstreamConn *net.UDPConn) {
	clientAddrString := clientAddr.String()
	for {
		buffer := make([]byte, p.BufferSize)
		size, _, err := upstreamConn.ReadFromUDP(buffer)
		if err != nil {
			p.connectionsLock.Lock()
			upstreamConn.Close()
			delete(p.connsMap, clientAddrString)
			p.connectionsLock.Unlock()
			return
		}
		p.updateClientLastActivity(clientAddrString)
		p.upstreamMessageChannel <- packet{
			src:  clientAddr,
			data: buffer[:size],
		}
	}
}

func (p *Proxy) handlerUpstreamPackets() {
	for pa := range p.upstreamMessageChannel {
		p.Logger.Debug("forwarded data from upstream", zap.Int("size", len(pa.data)), zap.String("data", string(pa.data)))
		p.listenerConn.WriteTo(pa.data, pa.src)
	}
}

func (p *Proxy) handleClientPackets() {
	for pa := range p.clientMessageChannel {
		packetSourceString := pa.src.String()
		p.Logger.Debug("packet received",
			zap.String("src address", packetSourceString),
			zap.Int("src port", pa.src.Port),
			zap.String("packet", string(pa.data)),
			zap.Int("size", len(pa.data)),
		)

		p.connectionsLock.RLock()
		conn, found := p.connsMap[packetSourceString]
		p.connectionsLock.RUnlock()

		if !found {
			conn, err := net.ListenUDP("udp", p.client)
			p.Logger.Debug("new client connection",
				zap.String("local port", conn.LocalAddr().String()),
			)

			if err != nil {
				p.Logger.Error("upd proxy failed to dial", zap.Error(err))
				return
			}

			p.connectionsLock.Lock()
			p.connsMap[packetSourceString] = connection{
				udp:          conn,
				lastActivity: time.Now(),
			}
			p.connectionsLock.Unlock()

			conn.WriteTo(pa.data, p.upstream)
			go p.clientConnectionReadLoop(pa.src, conn)
		} else {
			conn.udp.WriteTo(pa.data, p.upstream)
			p.connectionsLock.RLock()
			shouldUpdateLastActivity := false
			if _, found := p.connsMap[packetSourceString]; found {
				if p.connsMap[packetSourceString].lastActivity.Before(
					time.Now().Add(-p.ConnTimeout / 4)) {
					shouldUpdateLastActivity = true
				}
			}
			p.connectionsLock.RUnlock()
			if shouldUpdateLastActivity {
				p.updateClientLastActivity(packetSourceString)
			}
		}
	}
}

func (p *Proxy) readLoop() {
	for !p.closed {
		buffer := make([]byte, p.BufferSize)
		size, srcAddress, err := p.listenerConn.ReadFromUDP(buffer)
		if err != nil {
			p.Logger.Error("error", zap.Error(err))
			continue
		}
		p.clientMessageChannel <- packet{
			src:  srcAddress,
			data: buffer[:size],
		}
	}
}

func (p *Proxy) resolveUpstreamLoop() {
	for !p.closed {
		time.Sleep(p.ResolveTTL)
		upstreamAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", p.UpstreamAddress, p.UpstreamPort))
		if err != nil {
			p.Logger.Error("resolve error", zap.Error(err))
			continue
		}
		if p.upstream.String() != upstreamAddr.String() {
			p.upstream = upstreamAddr
			p.Logger.Info("upstream addr changed", zap.String("upstreamAddr", p.upstream.String()))
		}
	}
}

func (p *Proxy) freeIdleSocketsLoop() {
	for !p.closed {
		time.Sleep(p.ConnTimeout)
		var clientsToTimeout []string

		p.connectionsLock.RLock()
		for client, conn := range p.connsMap {
			if conn.lastActivity.Before(time.Now().Add(-p.ConnTimeout)) {
				clientsToTimeout = append(clientsToTimeout, client)
			}
		}
		p.connectionsLock.RUnlock()

		p.connectionsLock.Lock()
		for _, client := range clientsToTimeout {
			p.Logger.Debug("client timeout", zap.String("client", client))
			p.connsMap[client].udp.Close()
			delete(p.connsMap, client)
		}
		p.connectionsLock.Unlock()
	}
}

func (p *Proxy) Close() {
	p.Logger.Warn("Closing proxy")
	p.connectionsLock.Lock()
	p.closed = true
	for _, conn := range p.connsMap {
		conn.udp.Close()
	}
	if p.listenerConn != nil {
		p.listenerConn.Close()
	}
	p.connectionsLock.Unlock()
}

func (p *Proxy) Start() error {
	p.Logger.Info("starting udp proxy")

	ProxyAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", p.BindAddress, p.BindPort))
	if err != nil {
		p.Logger.Error("error resolving bind address", zap.Error(err))
		return err
	}
	p.upstream, err = net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", p.UpstreamAddress, p.UpstreamPort))
	if err != nil {
		p.Logger.Error("error resolving upstream address", zap.Error(err))
	}
	p.client = &net.UDPAddr{
		IP:   ProxyAddr.IP,
		Port: 0,
		Zone: ProxyAddr.Zone,
	}
	p.listenerConn, err = net.ListenUDP("udp", ProxyAddr)
	if err != nil {
		p.Logger.Error("error listening on bind port", zap.Error(err))
		return err
	}
	p.Logger.Info("udp proxy started")
	if p.ConnTimeout.Nanoseconds() > 0 {
		go p.freeIdleSocketsLoop()
	} else {
		p.Logger.Warn("be warned that running without timeout to clients may be dangerous")
	}
	if p.ResolveTTL.Nanoseconds() > 0 {
		go p.resolveUpstreamLoop()
	} else {
		p.Logger.Warn("not refreshing upstream addr")
	}
	go p.handlerUpstreamPackets()
	go p.handleClientPackets()
	go p.readLoop()
	return nil
}
