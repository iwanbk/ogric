package ogric

import (
	"strconv"
	"strings"
	"time"
)

func (o *Ogric) handleEvent(e Event) {
	if e.Code == "PRIVMSG" && len(e.Message) > 0 && e.Message[0] == '\x01' {
		e.Code = "CTCP" //Unknown CTCP

		if i := strings.LastIndex(e.Message, "\x01"); i > -1 {
			e.Message = e.Message[1:i]
		}

		if e.Message == "VERSION" {
			e.Code = "CTCP_VERSION"

		} else if e.Message == "TIME" {
			e.Code = "CTCP_TIME"

		} else if e.Message[0:4] == "PING" {
			e.Code = "CTCP_PING"

		} else if e.Message == "USERINFO" {
			e.Code = "CTCP_USERINFO"

		} else if e.Message == "CLIENTINFO" {
			e.Code = "CTCP_CLIENTINFO"
		}
	}

	if o.Debug {
		o.log.Printf("%v (0) >> %#v\n", e.Code, e)
	}
	switch e.Code {
	case "PING":
		o.sendRaw("PONG :" + e.Message)
	case "CTCP_VERSION":
		o.sendRawf("NOTICE %s :\x01VERSION %s\x01", e.Nick, VERSION)
	case "CTCP_USERINFO":
		o.sendRawf("NOTICE %s :\x01USERINFO %s\x01", e.Nick, o.user)
	case "CTCP_CLIENTINFO":
		o.sendRawf("NOTICE %s :\x01CLIENTINFO PING VERSION TIME USERINFO CLIENTINFO\x01", e.Nick)
	case "CTCP_TIME":
		ltime := time.Now()
		o.sendRawf("NOTICE %s :\x01TIME %s\x01", e.Nick, ltime.String())
	case "CTCP_PING":
		o.sendRawf("NOTICE %s :\x01%s\x01", e.Nick, e.Message)
	case "437":
		o.Nickcurrent = o.Nickcurrent + "_"
		o.sendRawf("NICK %s", o.Nickcurrent)
	case "433":
		if len(o.Nickcurrent) > 8 {
			o.Nickcurrent = "_" + o.Nickcurrent

		} else {
			o.Nickcurrent = o.Nickcurrent + "_"
		}
		o.sendRawf("NICK %s", o.Nickcurrent)
	case "PONG":
		ns, _ := strconv.ParseInt(e.Message, 10, 64)
		delta := time.Duration(time.Now().UnixNano() - ns)
		o.log.Printf("Lag: %vs\n", delta)
	case "NICK":
		if e.Nick == o.Nick {
			o.Nickcurrent = e.Message
		}
	case "001":
		o.Nickcurrent = e.Arguments[0]
	}

}
