package startcmd

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/M-ERCURY/core/api/auth"
	"github.com/M-ERCURY/core/api/client"
	"github.com/M-ERCURY/core/api/consume"
	"github.com/M-ERCURY/core/api/contractinfo"
	"github.com/M-ERCURY/core/api/dirinfo"
	"github.com/M-ERCURY/core/api/relayentry"
	"github.com/M-ERCURY/core/api/relaylist"
	"github.com/M-ERCURY/core/api/status"
	"github.com/M-ERCURY/core/cli"
	"github.com/M-ERCURY/core/cli/commonsub/startcmd"
	"github.com/M-ERCURY/core/cli/fsdir"
	"github.com/M-ERCURY/core/cli/upgrade"
	"github.com/M-ERCURY/core/mrnet/transport"
	"github.com/M-ERCURY/poc/circuit"
	"github.com/M-ERCURY/poc/clientcfg"
	"github.com/M-ERCURY/poc/clientlib"
	"github.com/M-ERCURY/poc/dnscachedial"
	"github.com/M-ERCURY/poc/filenames"
	"github.com/M-ERCURY/poc/sub/initcmd/embedded"
	"github.com/M-ERCURY/poc/sub/tuncmd"
	"github.com/M-ERCURY/poc/version"
)

func Cmd() *cli.Subcmd {
	run := func(fm fsdir.T) {
		var (
			cl   = client.New(nil, auth.Client)
			ci   *contractinfo.T
			rl   relaylist.T
			di   dirinfo.T
			circ circuit.T
			mu   sync.Mutex
		)

		if err := cli.UnpackEmbeddedV2(embedded.FS, fm, false); err != nil {
			log.Fatal(err)
		}

		c := clientcfg.Defaults()

		if _, err := os.Stat(fm.Path(filenames.Config)); err != nil {
			fm.Set(c, filenames.Config)
		}

		err := fm.Get(&c, filenames.Config)
		if err != nil {
			log.Fatal(err)
		}

		if c.Address.Socks == nil && c.Address.H2C == nil {
			log.Fatal("both address.socks and address.h2c are nil in config, please set one or both")
		}

		tt := transport.New(transport.Options{Timeout: time.Duration(c.Timeout)})
		// cache dns resolution in netstack transport
		cache := dnscachedial.New()
		tt.Transport.DialContext = cache.Cover(tt.Transport.DialContext)
		tt.Transport.DialTLSContext = cache.Cover(tt.Transport.DialTLSContext)
		cl.Transport = tt.Transport
		dialf := tt.DialSM
		// force target protocol if needed
		tproto, ok := os.LookupEnv("MERCURY_TARGET_PROTOCOL")
		if ok {
			dialf = func(proto string, remote *url.URL) (net.Conn, error) {
				if remote.Scheme == "target" {
					proto = tproto
				}
				return dialf(proto, remote)
			}
		}

		// make circuit
		syncinfo := func() error {
			if c.Contract == nil {
				return fmt.Errorf("contract is not defined")
			}
			if ci, rl, err = clientlib.GetContractInfo(cl, c.Contract); err != nil {
				return fmt.Errorf("could not get contract info: %w", err)
			}
			if di, err = consume.DirectoryInfo(cl, c.Contract); err != nil {
				return fmt.Errorf("could not get contract directory info: %w", err)
			}
			if err = clientlib.SaveContractInfo(fm, ci, rl); err != nil {
				return fmt.Errorf("could not save contract info: %w", err)
			}
			return nil
		}
		if c.Contract != nil {
			if err = syncinfo(); err != nil {
				log.Fatalf("could not get contract info: %s", err)
			}
		}

		circuitf := func() (r []*relayentry.T, err error) {
			// use existing if available
			if circ != nil {
				return circ, nil
			}
			// if not, avoid race conditions
			mu.Lock()
			defer mu.Unlock()
			if err = syncinfo(); err != nil {
				fmt.Println("syncinfo error", err)
				return nil, err
			}
			var all circuit.T
			if c.Circuit.Whitelist != nil {
				if len(*c.Circuit.Whitelist) > 0 {
					for _, addr := range *c.Circuit.Whitelist {
						if rl[addr] != nil {
							all = append(all, rl[addr])
						}
					}
				}
			} else {
				all = rl.All()
			}
			if r, err = circuit.Make(c.Circuit.Hops, all); err != nil {
				fmt.Println("circuit.Make error", err)
				return
			}
			circ = r
			// expose bypass for mercury_tun
			sc := cache.Get(c.Contract.Hostname())
			dir := cache.Get(di.Endpoint.Hostname())
			bypass := append(sc, dir...)
			if len(r) > 0 {
				bypass = append(bypass, cache.Get(r[0].Addr.Hostname())...)
			}

			err = fm.Set(bypass, filenames.Bypass)

			return
		}

		_, err = clientlib.ValidateAndRecievePofs(fm)
		if err != nil {
			fmt.Println("could not validate pofs: ", err)

			// update servicekey
			if err := clientlib.UpdateServiceKey(fm, c.PofURL); err != nil {
				log.Fatalf("!!could not update servicekey: %s", err)
			}
		}

		// cache dns, sc and directory data if we can
		sks := clientlib.SKSource(fm, &c, cl)
		fmt.Println("BEFORE sks(false)")
		if _, err := sks(false); err == nil {
			// cache sc pubkey and directory contents
			if _, err := circuitf(); err == nil {
				circ = nil
				// cache all relay addresses just in case
				if rl != nil {
					for _, r := range rl.All() {
						err = cache.Cache(context.Background(), r.Addr.Hostname())
						if err != nil {
							log.Printf("could not cache %s: %s", r.Addr.Hostname(), err)
						}
					}
				}
				circuitf()
			}
		}

		if _, err := circuitf(); err != nil {
			fmt.Println("custom circuit error", err)
		}

		// maybe there's an upgrade available?
		if di.Channels != nil {
			if v, ok := di.Channels[version.Channel]; ok && v.GT(version.VERSION) {
				skipv := upgrade.NewConfig(fm, "mercury", false).SkippedVersion()
				if skipv != nil && skipv.EQ(v) {
					log.Printf("Upgrade available to %s, current version is %s. ", v, version.VERSION)
					log.Printf("Last upgrade attempt to %s failed! Keeping current version; please upgrade when possible.", skipv)
				} else {
					log.Fatalf(
						"Upgrade available to %s, current version is %s. Please run `mercury upgrade`.",
						v, version.VERSION,
					)
				}
			}
		}
		// set up local listening functions
		var (
			listening = []string{}
			dialer    = clientlib.CircuitDialer(clientlib.AlwaysFetch(sks), circuitf, dialf)
			errf      = func(e error) {
				if err != nil {
					if o := clientlib.TraceOrigin(err, circ); o != nil {
						if status.IsCircuitError(err) {
							// reset on circuit errors
							log.Printf(
								"relay-originated circuit error from %s: %s, resetting circuit",
								o.Pubkey,
								err,
							)
							mu.Lock()
							circ = nil
							mu.Unlock()
						} else {
							// not reset-worthy
							log.Printf("error from %s: %s", o.Pubkey, err)
						}
					} else {
						log.Printf("circuit dial error: %s", err)
					}
				}
			}
		)
		if c.Address.Socks != nil {
			err = clientlib.ListenSOCKS(*c.Address.Socks, dialer, errf)
			if err != nil {
				log.Fatalf("listening on socks5://%s and udp://%s failed: %s", *c.Address.Socks, *c.Address.Socks, err)
			}
			listening = append(listening, "socksv5://"+*c.Address.Socks, "udp://"+*c.Address.Socks)
		}
		if c.Address.H2C != nil {
			err = clientlib.ListenH2C(*c.Address.H2C, tt.TLSClientConfig, dialer, errf)
			if err != nil {
				log.Fatalf("listening on h2c://%s failed: %s", *c.Address.H2C, err)
			}
			listening = append(listening, "h2c://"+*c.Address.H2C)
		}
		log.Printf("listening on: %v", listening)

		time.Sleep(2 * time.Second)
		tuncmd.Start(fm, c)

		shutdown := func() bool {
			log.Println("gracefully shutting down...")
			fm.Del(filenames.Pid)

			// stop tun
			fmt.Println("Shutting down the tune")
			tuncmd.Stop(fm)
			return true
		}
		defer shutdown()
		cli.SignalLoop(cli.SignalMap{
			syscall.SIGUSR1: func() (_ bool) {
				log.Println("reloading config")
				mu.Lock()
				defer mu.Unlock()
				// reload config
				err = fm.Get(&c, filenames.Config)
				if err != nil {
					log.Printf(
						"could not reload config: %s, aborting reload",
						err,
					)
					return
				}
				// refresh contract info
				if err = syncinfo(); err != nil {
					log.Printf(
						"could not refresh contract info: %s, aborting reload",
						err,
					)
					return
				}
				// reset circuit
				circ = nil
				return
			},
			syscall.SIGINT:  shutdown,
			syscall.SIGTERM: shutdown,
			syscall.SIGQUIT: shutdown,
		})
	}

	r := startcmd.Cmd("mercury", run)
	r.Desc = fmt.Sprintf("%s %s", r.Desc, "(SOCKSv5/connection broker)")
	r.Sections = []cli.Section{
		{
			Title: "Signals",
			Entries: []cli.Entry{
				{
					Key:   "SIGUSR1\t(10)",
					Value: "Reload configuration, contract information and circuit",
				},
				{
					Key:   "SIGTERM\t(15)",
					Value: "Gracefully stop mercury daemon and exit",
				},
				{
					Key:   "SIGQUIT\t(3)",
					Value: "Gracefully stop mercury daemon and exit",
				},
				{
					Key:   "SIGINT\t(2)",
					Value: "Gracefully stop mercury daemon and exit",
				},
			},
		},
		{
			Title: "Environment",
			Entries: []cli.Entry{{
				Key:   "MERCURY_TARGET_PROTOCOL",
				Value: "Resolve target IP via tcp4, tcp6 or tcp (default)",
			}},
		},
	}
	return r
}
