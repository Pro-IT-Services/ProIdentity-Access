package daemon

import (
	"net"
	"strings"
	"sync"
	"time"
)

// mgmtChannel talks to openvpn's management interface one command at a time.
//
// The interface is not pipelined: a command sent before the previous one's
// "SUCCESS:"/"ERROR:" reply can be dropped. Sending the management password,
// "state on", "bytecount" and "hold release" back to back lost commands, and
// so did "username" + "password" (openvpn then waits for the password
// forever: the endless "Connecting"). Verified against openvpn 2.7.7:
// pipelined 0/5 attempts authenticated, serial 5/5.
type mgmtChannel struct {
	conn net.Conn
	cmds chan string
	acks chan string
	done chan struct{}
	once sync.Once
	wmu  sync.Mutex
}

// ackTimeout bounds the wait for a reply so a lost one can't stall the queue.
const ackTimeout = 10 * time.Second

func newMgmtChannel(conn net.Conn) *mgmtChannel {
	c := &mgmtChannel{
		conn: conn,
		cmds: make(chan string, 64),
		acks: make(chan string, 1),
		done: make(chan struct{}),
	}
	go c.writer()
	return c
}

// send queues a command; it is written once the previous one was answered.
func (c *mgmtChannel) send(cmd string) {
	select {
	case c.cmds <- cmd:
	case <-c.done:
	}
}

func (c *mgmtChannel) writer() {
	for {
		select {
		case <-c.done:
			return
		case cmd := <-c.cmds:
			c.write(cmd)
			select {
			case <-c.acks:
			case <-time.After(ackTimeout):
			case <-c.done:
				return
			}
		}
	}
}

// write sends one line immediately (bypassing the queue; used for SIGTERM).
func (c *mgmtChannel) write(cmd string) {
	c.wmu.Lock()
	defer c.wmu.Unlock()
	_, _ = c.conn.Write([]byte(cmd + "\n"))
}

// reply classifies a line from openvpn: command replies are consumed here,
// everything else (">STATE:", ">PASSWORD:", ...) is returned for handling.
func (c *mgmtChannel) reply(line string) (notification string, isReply bool) {
	// The password prompt has no newline, so it prefixes the next line.
	for strings.HasPrefix(line, "ENTER PASSWORD:") {
		line = strings.TrimPrefix(line, "ENTER PASSWORD:")
	}
	if strings.HasPrefix(line, "SUCCESS:") || strings.HasPrefix(line, "ERROR:") {
		select {
		case c.acks <- line:
		default:
		}
		return "", true
	}
	return line, false
}

func (c *mgmtChannel) close() {
	c.once.Do(func() {
		close(c.done)
		_ = c.conn.Close()
	})
}
