package execcmd

import (
	"flag"
	"log"
	"net"
	"os"
	"syscall"
	"time"

	"github.com/M-ERCURY/core/cli"
	"github.com/M-ERCURY/core/cli/fsdir"
	"github.com/M-ERCURY/poc/clientcfg"
	"github.com/M-ERCURY/poc/filenames"
)

func Cmd() *cli.Subcmd {
	fs := flag.NewFlagSet("exec", flag.ExitOnError)
	r := &cli.Subcmd{
		FlagSet: fs,
		Desc:    "Execute script from scripts directory",
	}
	r.SetMinimalUsage("FILENAME")
	r.Run = func(fm fsdir.T) {
		c := clientcfg.Defaults()
		err := fm.Get(&c, filenames.Config)

		if err != nil {
			log.Fatal(err)
		}

		if fs.NArg() < 1 {
			r.Usage()
		}

		p := fm.Path("scripts", fs.Arg(0))
		fi, err := os.Stat(p)

		if err != nil {
			p0 := fm.Path("scripts", "default", fs.Arg(0))
			fi, err = os.Stat(p0)
			if err != nil {
				log.Fatalf("could not stat %s or %s: %s", p, p0, err)
			}
			p = p0
		}

		if fi.Mode()&0111 == 0 {
			log.Fatalf("could not execute %s: file is not executable (did you `chmod +x`?)", p)
		}

		var pid int
		err = fm.Get(&pid, filenames.Pid)

		if err != nil {
			log.Fatalf("it appears mercury is not running: could not get mercury PID from %s: %s", fm.Path(filenames.Pid), err)
		}

		err = syscall.Kill(pid, 0)

		if err != nil {
			log.Fatalf("it appears mercury is not running: %s", err)
		}

		conn, err := net.DialTimeout("tcp", *c.Address.Socks, time.Second)

		if err != nil {
			log.Fatalf("could not connect to mercury at address.socks %s: %s", *c.Address.Socks, err)
		}

		conn.Close()

		err = syscall.Exec(
			p,
			fs.Args(),
			append([]string{
				"MERCURY_SOCKS=" + *c.Address.Socks,
			}, os.Environ()...),
		)

		hint := ""

		if os.IsPermission(err) {
			hint = ", check permissions (ownership and executable bit/+x)?"
		}

		if err != nil {
			log.Fatalf("could not execute %s: %s%s", p, err, hint)
		}
	}
	return r
}
