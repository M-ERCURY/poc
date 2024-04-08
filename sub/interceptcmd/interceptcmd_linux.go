package interceptcmd

import (
	"flag"
	"log"
	"os"
	"os/exec"
	"runtime"
	"syscall"

	"github.com/M-ERCURY/core/cli"
	"github.com/M-ERCURY/core/cli/fsdir"
	"github.com/M-ERCURY/poc/clientcfg"
	"github.com/M-ERCURY/poc/filenames"
)

func Cmd() *cli.Subcmd {
	fs := flag.NewFlagSet("intercept", flag.ExitOnError)

	run := func(fm fsdir.T) {
		c := clientcfg.Defaults()
		err := fm.Get(&c, filenames.Config)

		if err != nil {
			log.Fatal(err)
		}

		if fs.NArg() == 0 {
			fs.Usage()
		}

		switch runtime.GOOS {
		case "linux":
			lib := fm.Path("mercury_intercept.so")
			args := fs.Args()

			bin, err := exec.LookPath(args[0])

			if err != nil {
				log.Fatal(err)
			}

			err = syscall.Exec(
				bin,
				args,
				append([]string{
					"LD_PRELOAD=" + lib,
					"SOCKS5_PROXY=" + *c.Address.Socks,
				}, os.Environ()...),
			)

			if err != nil {
				log.Fatal(err)
			}
		default:
			log.Fatal("unsupported OS:", runtime.GOOS)
		}
	}

	r := &cli.Subcmd{
		FlagSet: fs,
		Desc:    "Run executable and redirect connections to mercury daemon",
		Run:     run,
	}

	r.SetMinimalUsage("[args]")
	return r
}
