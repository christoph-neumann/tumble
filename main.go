package main

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/christoph-neumann/gumble/gumbleopenal"
	"github.com/jessevdk/go-flags"
	"github.com/layeh/gumble/gumble"
	"github.com/layeh/gumble/gumbleutil"
	_ "github.com/layeh/gumble/opus"
)

var opts struct {
	Insecure bool   `long:"insecure" description:"do not validate the server certificate"`
	Mute     bool   `long:"mute" description:"don't transmit audio"`
	Deafen   bool   `long:"deafen" description:"don't play audio (does not imply mute)"`
	UserCert string `long:"user-cert" description:"user certificate file (PEM format)"`
	UserKey  string `long:"user-key" description:"user key file if the key is not in the cert (PEM format)"`
	Args     struct {
		Name string `description:"mumble URL such as mumble://username:pass@domain.name:64738/channel/path. User and port are optional." positional-arg-name:"URL"`
	} `positional-args:"yes" required:"yes"`
}

func die(err error) {
	name := regexp.MustCompile("\\/").Split(os.Args[0], -1)
	fmt.Fprintf(os.Stderr, "%s: %s\n", name[len(name)-1], err)
	os.Exit(1)
}

func filterEmpty(path []string) []string {
	res := make([]string, 0)
	for _, v := range path {
		if v != "" {
			res = append(res, v)
		}
	}
	return res
}

func main() {
	_, err := flags.Parse(&opts)
	if err != nil {
		os.Exit(1)
	}

	url, err := url.Parse(opts.Args.Name)
	if err != nil {
		die(err)
	}

	host, port, err := net.SplitHostPort(url.Host)
	if err != nil {
		host = url.Host
		port = strconv.Itoa(gumble.DefaultPort)
	}

	channelPath := filterEmpty(strings.Split(url.Path, "/"))

	var tlsConfig tls.Config
	tlsConfig.InsecureSkipVerify = opts.Insecure
	if opts.UserCert != "" {
		key := opts.UserCert
		if opts.UserKey != "" {
			key = opts.UserKey
		}
		if certificate, err := tls.LoadX509KeyPair(opts.UserCert, key); err != nil {
			die(err)
		} else {
			tlsConfig.Certificates = append(tlsConfig.Certificates, certificate)
		}
	}

	onDisconnect := make(chan bool)

	config := gumble.NewConfig()
	config.Username = url.User.Username()
	config.Password, _ = url.User.Password()

	config.Attach(gumbleutil.AutoBitrate)

	config.Attach(gumbleutil.Listener{
		Connect: func(e *gumble.ConnectEvent) {
			if len(channelPath) > 0 {
				if ch := e.Client.Channels.Find(channelPath...); ch == nil {
					die(fmt.Errorf("no such channel: %v\n", strings.Join(channelPath, "/")))
				} else {
					e.Client.Self.Move(ch)
				}
			}

			if os.Getenv("ALSOFT_LOGLEVEL") == "" {
				os.Setenv("ALSOFT_LOGLEVEL", "1")
			}
			_, err := gumbleopenal.Setup(e.Client, gumbleopenal.Config{Mute: opts.Mute, Deafen: opts.Deafen})
			if err != nil {
				die(err)
			}
		},
		Disconnect: func(e *gumble.DisconnectEvent) {
			onDisconnect <- true
		},
	})

	_, err = gumble.DialWithDialer(new(net.Dialer), net.JoinHostPort(host, port), config, &tlsConfig)
	if err != nil {
		die(err)
	}

	<-onDisconnect
}
