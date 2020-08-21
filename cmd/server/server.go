package main

import (
	"fmt"
	tp "github.com/henrylee2cn/teleport"
)

//go:generate go build $GOFILE

func main() {
	defer tp.FlushLogger()
	// graceful
	go tp.GraceSignal()

	// server peer
	srv := tp.NewPeer(tp.PeerConfig{
		CountTime:   true,
		ListenPort:  9090,
		PrintDetail: true,
	})
	// srv.SetTLSConfig(tp.GenerateTLSConfigForServer())

	// router
	//srv.RouteCall(new(Math))

	// broadcast per 5s
	//go func() {
	//	for {
	//		time.Sleep(time.Second * 5)
	//		srv.RangeSession(func(sess tp.Session) bool {
	//			sess.Push(
	//			"/push/status",
	//				fmt.Sprintf("this is a message for %s server time: %v", sess.ID(), time.Now()),
	//			)
	//			return true
	//		})
	//	}
	//}()
	// listen and serve
	srv.ListenAndServe()
	//time.Sleep(5*time.Second)
	//srv.
	srv.RangeSession(func(sess tp.Session) bool {
		fmt.Println("Test")
		sess.Push(
			"/push/createrelay",
			"box.abuse.md:26562",
		)
		return true
	})

	select {}
}

// Math handler
type Math struct {
	tp.CallCtx
}

// Add handles addition request
func (m *Math) Add(arg *[]int) (int, *tp.Status) {
	// test query parameter
	tp.Infof("author: %s", m.PeekMeta("author"))
	// add
	var r int
	for _, a := range *arg {
		r += a
	}
	// response
	return r, nil
}