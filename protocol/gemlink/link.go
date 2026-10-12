package gemlink

import (
	"github.com/arloliu/go-secs/v2/hsms"
	"github.com/arloliu/go-secs/v2/hsmsss"
	"net"
	"strconv"
	"time"
)

// New uses a fixed local session ID; global equipment identity is verified separately.
func New(endpoint string, active bool) (hsms.Connection, error) {
	host, p, e := net.SplitHostPort(endpoint)
	if e != nil {
		return nil, e
	}
	port, e := strconv.Atoi(p)
	if e != nil {
		return nil, e
	}
	mode := hsmsss.WithPassive()
	if active {
		mode = hsmsss.WithActive()
	}
	cfg, e := hsmsss.NewConfig(host, port, mode, hsmsss.WithConnectionOption(hsms.WithSessionID(1)), hsmsss.WithConnectionOption(hsms.WithT3(5*time.Second)), hsmsss.WithConnectionOption(hsms.WithT5(time.Second)), hsmsss.WithConnectionOption(hsms.WithT6(5*time.Second)), hsmsss.WithConnectionOption(hsms.WithT7(10*time.Second)), hsmsss.WithConnectionOption(hsms.WithT8(5*time.Second)), hsmsss.WithConnectionOption(hsms.WithStrictReplyMatching(true)))
	if e != nil {
		return nil, e
	}
	return hsmsss.New(cfg)
}
