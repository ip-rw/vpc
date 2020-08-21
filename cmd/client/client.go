package main

import (
	"time"

	tp "github.com/henrylee2cn/teleport"
)

//go:generate go build $GOFILE

func main() {
	tp.SetLoggerLevel("DEBUG")

	cli := tp.NewPeer(tp.PeerConfig{})
	defer cli.Close()
	// cli.SetTLSConfig(&tls.Config{InsecureSkipVerify: true})

	cli.RoutePush(new(Push))

	sess, stat := cli.Dial(":9090")
	if !stat.OK() {
		tp.Fatalf("%v", stat)
	}
	for {
		tp.Printf("ID: %s", sess.ID())
		time.Sleep(5*time.Second)
	}
	//sess.ID()
	//var result int
	//stat = sess.Call("/math/add",
	//	[]int{1, 2, 3, 4, 5},
	//	&result,
	//	tp.WithAddMeta("author", "henrylee2cn"),
	//).Status()
	//if !stat.OK() {
	//	tp.Fatalf("%v", stat)
	//}
	//tp.Printf("result: %d", result)
	//
}

// Push push handler
type Push struct {
	tp.PushCtx
}

//// Push handles '/push/status' message
//func (p *Push) Status(arg *string) *tp.Status {
//	tp.Printf("%s", *arg)
//	return nil
//}

func (p *Push) CreateRelay(arg *string) *tp.Status {
	tp.Printf("RELAY %s", *arg)
	return nil
}