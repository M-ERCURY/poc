package servicekeycmd

import (
	"errors"
	"flag"
	"io"
	"log"
	"os"
	"syscall"
	"time"

	"github.com/M-ERCURY/core/api/client"
	"github.com/M-ERCURY/core/api/pof"
	"github.com/M-ERCURY/core/api/servicekey"
	"github.com/M-ERCURY/core/cli"
	"github.com/M-ERCURY/core/cli/fsdir"
	"github.com/M-ERCURY/poc/clientcfg"
	"github.com/M-ERCURY/poc/clientlib"
	"github.com/M-ERCURY/poc/filenames"
)

func Cmd() *cli.Subcmd {
	fs := flag.NewFlagSet("servicekey", flag.ExitOnError)

	run := func(fm fsdir.T) {
		c := clientcfg.Defaults()
		err := fm.Get(&c, filenames.Config)

		if err != nil {
			log.Fatal(err)
		}

		switch {
		case c.Contract == nil:
			log.Fatal("contract has to be set")
		case c.Accesskey.UseOnDemand:
			log.Fatal("accesskey.use_on_demand is enabled in config.json; refusing to run")
		}

		var ps []*pof.T
		err = fm.Get(&ps, "pofs.json")

		if err != nil {
			log.Fatalf("could not read pofs from pofs.json: %s", err)
		}

		cl := client.New(nil, "Client")

		sk := &servicekey.T{}
		err = fm.Get(sk, filenames.Servicekey)

		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, os.ErrNotExist) {
				// this is fine
			} else {
				log.Fatalf(
					"error reading old %s",
					filenames.Servicekey,
				)
			}
		} else {
			if !sk.IsExpiredAt(time.Now().Unix()) {
				log.Fatalf(
					"refusing to replace non-expired servicekey: %s expires at %s",
					filenames.Servicekey,
					time.Unix(sk.Contract.SettlementOpen, 0).String(),
				)
			}
		}

		// discard old servicekey & get a new one
		sk, err = clientlib.RefreshSK(fm, c.PofURL, func(p *pof.T) (*servicekey.T, error) {
			return clientlib.NewSKFromPof(
				cl,
				c.Contract.String()+"/servicekey/activate",
				p,
			)
		})

		if err != nil {
			log.Fatalf(
				"error while activating servicekey with pof: %s",
				err,
			)
		}

		err = fm.Set(sk, filenames.Servicekey)

		if err != nil {
			log.Fatalf(
				"could not write new servicekey: %s",
				err,
			)
		}

		// reload mercury daemon if possible
		var pid int
		err = fm.Get(&pid, filenames.Pid)

		// if not, it's no big deal -- still let the user know
		if err != nil {
			log.Printf(
				"could not send SIGUSR1 to running mercury daemon: %s",
				err,
			)
		}

		syscall.Kill(pid, syscall.SIGUSR1)
	}

	r := &cli.Subcmd{
		FlagSet: fs,
		Desc:    "Trigger accesskey activation (accesskey.use_on_demand=false)",
		Run:     run,
	}

	r.SetMinimalUsage("")

	return r
}
