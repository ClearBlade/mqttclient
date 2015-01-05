package mqttclient

import (
	mqtt "github.com/clearblade/mqtt_parsing"
	"log"
	"net"
	"strings"
	"testing"
	"time"
)

//using an actual tcp listener isn't the best case, but we can't just us and io.readwritecloser
//where we use connection
func startDummyListener(addr string, close chan struct{}, o chan mqtt.Message, t *testing.T) {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		t.Error(err.Error())
		return
	}
	conchan := make(chan net.Conn, 1)

	defer listener.Close()
	go func() {
		for {
			con, err := listener.Accept()
			if err != nil {
				return
			}
			conchan <- con
		}
	}()

	for {

		select {
		case con := <-conchan:
			go func(c net.Conn) {
				for {
					msg, err := mqtt.DecodePacket(c)
					if err != nil {
						c.Close()
						return
					}
					o <- msg
				}
			}(con)
		case <-close:
			return
		default:
		}
	}

}

func Test_ShutdownTimes(t *testing.T) {
	ch := make(chan struct{}, 1)
	port := ":63756"
	msgs := make(chan mqtt.Message, 20)

	go startDummyListener(port, ch, msgs, t)

	<-time.After(time.Millisecond * 20)

	c := NewClient("boop", "beep", "zoop", "foo", 60)

	err := c.Start(port, nil)
	if err != nil {
		t.Error(err.Error())
	}

	c.Shutdown(true)
	<-time.After(time.Millisecond * 20)

	if len(c.internalOutgoingBuf) != 0 {
		t.Error("didn't clear from internalOutogingBuf")
	}

	if len(c.internalErrorBuffer) != 0 {
		for len(c.internalErrorBuffer) > 0 {
			e := <-c.internalErrorBuffer
			log.Printf("unremoved err %+v\n", e.err)
		}
		t.Error("Didn't clean out internal error buffer")
	}
	if len(c.shutdown_reader) != 0 {
		t.Error("reader probably didn't shutdown")
	}
	if len(c.shutdown_writer) != 0 {
		t.Error("writer probably didn't shutdown")
	}
	ch <- struct{}{}
	<-time.After(time.Millisecond * 10)
}

func Test_timeout(t *testing.T) {
	port := ":63757"
	msgs := make(chan mqtt.Message, 20)
	ch := make(chan struct{}, 1)
	go startDummyListener(port, ch, msgs, t)
	<-time.After(time.Millisecond * 20)
	defer func() { ch <- struct{}{} }()

	c := NewClient("boop", "beep", "zoop", "foo", 1)

	err := c.Start(port, nil)
	if err != nil {
		t.Error(err.Error())
	}

	SendConnect(c, false, false, 0, "", "")
	//the server won't respond, so, it should automatically disconnect after
	//2 seconds.
	//one to send the pingreq
	//and the other to decide it's not coming back
	<-time.After(time.Second * 4) //four seconds to accomodate for awaiting the response from the disconnect
	if _, err := c.C.Write([]byte("test")); err == nil {
		t.Error("should've gotten an error trying to write, didn't, so we didn't timeout")
	} else if !strings.Contains(err.Error(), "use of closed network connection") {
		t.Error("got wrong error. expected \"use of closed network connection\" but got " + err.Error() + " instead")
	}
	ch <- struct{}{}
	<-time.After(time.Millisecond * 10)
}
