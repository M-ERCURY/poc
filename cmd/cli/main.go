package main

import (
	"os"

	"github.com/M-ERCURY/core/cli"
	"github.com/M-ERCURY/core/cli/commonsub/logcmd"

	"github.com/M-ERCURY/core/cli/commonsub/reloadcmd"
	"github.com/M-ERCURY/core/cli/commonsub/restartcmd"
	"github.com/M-ERCURY/core/cli/commonsub/statuscmd"
	"github.com/M-ERCURY/core/cli/commonsub/stopcmd"
	"github.com/M-ERCURY/poc/sub/configcmd"
	"github.com/M-ERCURY/poc/sub/execcmd"
	"github.com/M-ERCURY/poc/sub/infocmd"
	"github.com/M-ERCURY/poc/sub/interceptcmd"
	"github.com/M-ERCURY/poc/sub/startcmd"
	"github.com/M-ERCURY/poc/sub/tuncmd"
)

const binname = "mercury"

func main() {
	fm := cli.Home()

	cli.CLI{
		Subcmds: []*cli.Subcmd{
			configcmd.Cmd(fm),
			startcmd.Cmd(),
			statuscmd.Cmd(binname),
			reloadcmd.Cmd(binname),
			restartcmd.Cmd(binname, startcmd.Cmd().Run, stopcmd.Cmd(binname).Run),
			stopcmd.Cmd(binname),
			execcmd.Cmd(),
			interceptcmd.Cmd(),
			tuncmd.Cmd(),
			infocmd.Cmd(),
			logcmd.Cmd(binname),
		},
	}.Parse(os.Args).Run(fm)
}
