package wireguard

var clientConfigTemplate = `[Interface]
PrivateKey = %s
Address = %s/24
	
[Peer]
PublicKey = %s
EndPoint = %s
AllowedIPs=0.0.0.0/0, ::0/0`
