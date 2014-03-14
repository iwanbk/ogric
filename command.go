package ogric

import (
	"fmt"
)

//Join makes the client join the channel
func (o *Ogric) Join(channel string) {
	o.pwrite <- fmt.Sprintf("JOIN %s\r\n", channel)
}

//Names returns a list of who is on the given channel
func (o *Ogric) Names(channel string) {
	o.pwrite <- fmt.Sprintf("NAMES %s\r\n", channel)
}

//Part Causes a user to leave the channel
func (o *Ogric) Part(channel, msg string) {
	if len(msg) > 0 {
		o.pwrite <- fmt.Sprintf("PART %s %s\r\n", channel, msg)
	} else {
		o.pwrite <- fmt.Sprintf("PART %s\r\n", channel)
	}
}

func (o *Ogric) Notice(target, message string) {
	o.pwrite <- fmt.Sprintf("NOTICE %s :%s\r\n", target, message)
}

func (o *Ogric) Noticef(target, format string, a ...interface{}) {
	o.Notice(target, fmt.Sprintf(format, a...))
}

func (o *Ogric) Privmsg(target, message string) {
	o.pwrite <- fmt.Sprintf("PRIVMSG %s :%s\r\n", target, message)
}
