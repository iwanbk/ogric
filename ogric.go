package ogric

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"
)

const (
	VERSION = "ogric-0.1"
)

type Event struct {
	Code      string
	Message   string
	Raw       string
	Nick      string //<nick>
	Host      string //<nick>!<usr>@<host>
	Source    string //<host>
	User      string //<usr>
	Arguments []string
}

type Ogric struct {
	server      string
	Nick        string //The nickname we want.
	Nickcurrent string //The nickname we currently have.
	Password    string
	user        string

	socket net.Conn

	UseTLS    bool
	TLSConfig *tls.Config

	Error       chan error
	lastMessage time.Time
	log         *log.Logger

	pwrite                             chan string
	readerExit, writerExit, pingerExit chan bool
	endping                            chan bool

	eventChan chan Event

	stopped bool
	Debug   bool
}

func NewOgric(nick, user, server string) *Ogric {
	o := new(Ogric)
	o.Nick = nick
	o.Nickcurrent = nick
	o.user = user
	o.server = server
	o.log = log.New(os.Stdout, "", log.LstdFlags)
	o.pwrite = make(chan string, 10)
	o.eventChan = make(chan Event, 2)
	o.Error = make(chan error, 2)
	o.readerExit = make(chan bool)
	o.writerExit = make(chan bool)
	o.pingerExit = make(chan bool)
	o.endping = make(chan bool)

	return o
}

func (c *Ogric) readLoop() {
	br := bufio.NewReaderSize(c.socket, 512)

	for {
		msg, err := br.ReadString('\n')
		if err != nil {
			c.Error <- err
			break
		}

		c.lastMessage = time.Now()
		msg = msg[:len(msg)-2] //Remove \r\n
		event := Event{Raw: msg}
		if msg[0] == ':' {
			if i := strings.Index(msg, " "); i > -1 {
				event.Source = msg[1:i]
				msg = msg[i+1 : len(msg)]

			} else {
				c.log.Printf("Misformed msg from server: %#s\n", msg)
			}

			if i, j := strings.Index(event.Source, "!"), strings.Index(event.Source, "@"); i > -1 && j > -1 {
				event.Nick = event.Source[0:i]
				event.User = event.Source[i+1 : j]
				event.Host = event.Source[j+1 : len(event.Source)]
			}
		}

		args := strings.SplitN(msg, " :", 2)
		if len(args) > 1 {
			event.Message = args[1]
		}

		args = strings.Split(args[0], " ")
		event.Code = strings.ToUpper(args[0])

		if len(args) > 1 {
			event.Arguments = args[1:len(args)]
		}
		/* XXX: len(args) == 0: args should be empty */

		c.eventChan <- event
	}

	c.readerExit <- true
}

func (c *Ogric) writeLoop() {
	for {
		b, ok := <-c.pwrite
		if !ok || b == "" || c.socket == nil {
			break
		}

		c.log.Printf("--> %s\n", b)
		_, err := c.socket.Write([]byte(b))
		if err != nil {
			c.Error <- err
			break
		}
	}
	c.writerExit <- true
}

//Pings the server if we have not recived any messages for 5 minutes
func (o *Ogric) pingLoop() {
	ticker := time.NewTicker(1 * time.Minute)   //Tick every minute.
	ticker2 := time.NewTicker(15 * time.Minute) //Tick every 15 minutes.
	for {
		select {
		case <-ticker.C:
			//Ping if we haven't received anything from the server within 4 minutes
			if time.Since(o.lastMessage) >= (4 * time.Minute) {
				o.sendRawf("PING %d", time.Now().UnixNano())
			}
		case <-ticker2.C:
			//Ping every 15 minutes.
			o.sendRawf("PING %d", time.Now().UnixNano())
			//Try to recapture nickname if it's not as configured.
			if o.Nick != o.Nickcurrent {
				o.Nickcurrent = o.Nick
				o.sendRawf("NICK %s", o.Nick)
			}
		case <-o.endping:
			ticker.Stop()
			ticker2.Stop()
			o.pingerExit <- true
			return
		}
	}
}

func (o *Ogric) sendRaw(message string) {
	o.pwrite <- message + "\r\n"
}

func (o *Ogric) sendRawf(format string, a ...interface{}) {
	o.sendRaw(fmt.Sprintf(format, a...))
}

func (o *Ogric) connect() error {
	var err error
	if o.UseTLS {
		o.socket, err = tls.Dial("tcp", o.server, o.TLSConfig)
	} else {
		o.socket, err = net.Dial("tcp", o.server)
	}
	if err != nil {
		return err
	}
	o.log.Printf("Connected to %s (%s)\n", o.server, o.socket.RemoteAddr())
	o.pwrite = make(chan string, 10)

	go o.readLoop()
	go o.writeLoop()
	go o.pingLoop()

	o.pwrite <- fmt.Sprintf("NICK %s\r\n", o.Nick)
	o.pwrite <- fmt.Sprintf("USER %s 0.0.0.0 0.0.0.0 :%s\r\n", o.user, o.user)

	if len(o.Password) > 0 {
		o.pwrite <- fmt.Sprintf("PASS %s\r\n", o.Password)
	}

	return nil
}

func (o *Ogric) loop(EventChan chan Event) {
	for !o.stopped {
		select {
		case err := <-o.Error:
			if o.stopped {
				break
			}
			o.log.Printf("Error: %s\n", err)
			o.disconnect()
			o.connect()
		case event := <-o.eventChan:
			o.handleEvent(event)
			EventChan <- event
		}
	}
	e := Event{Code: "OGRIC_STOPPED"}
	EventChan <- e
}

// Sends all buffered messages (if possible),
// stops all goroutines and then closes the socket.

func (o *Ogric) disconnect() {
	close(o.pwrite)
	o.endping <- true

	o.socket.Close()
	o.socket = nil

	<-o.readerExit
	<-o.writerExit
	<-o.pingerExit
}

func (o *Ogric) Stop() {
	o.stopped = true
	o.disconnect()
	o.log.Printf("[Ogric]Stopped")
}

func (o *Ogric) Start() (chan Event, error) {
	err := o.connect()
	if err != nil {
		log.Println("can't connect:", err)
		return nil, err
	}
	evtChan := make(chan Event, 10)

	go o.loop(evtChan)
	return evtChan, nil
}
