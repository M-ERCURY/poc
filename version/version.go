// The release version is defined here.
package version

import (
	"fmt"
	"log"

	"github.com/M-ERCURY/core/api/auth"
	"github.com/M-ERCURY/core/api/client"
	"github.com/M-ERCURY/core/api/consume"
	"github.com/M-ERCURY/core/cli"
	"github.com/M-ERCURY/core/cli/fsdir"
	"github.com/M-ERCURY/core/cli/upgrade"
	"github.com/M-ERCURY/poc/clientcfg"
	"github.com/M-ERCURY/poc/filenames"
	"github.com/blang/semver"
)

// old name compat
var GITREV string = "1.0.0"

// VERSION_STRING is the current version string, set by the linker via go build
// -X flag.
var VERSION_STRING = GITREV

// VERSION is the semver version struct of VERSION_STRING.
var VERSION = semver.MustParse(VERSION_STRING)

// Hardcoded (for now) channel value for mercury client.
const Channel = "client"

// Post-upgrade hook for superviseupgradecmd.
func PostUpgradeHook(f fsdir.T) (err error) {
	// force unpacking of files
	log.Println("unpacking new embedded files...")
	if err = cli.RunChild(f.Path("mercury"), "init", "--force-unpack-only"); err != nil {
		return
	}
	log.Println("stopping running mercury...")
	if err = cli.RunChild(f.Path("mercury"), "stop"); err != nil {
		return
	}
	fp := f.Path("mercury_tun")
	fmt.Println("===================================")
	fmt.Println("NOTE: to enable mercury_tun again:")
	fmt.Println("$ sudo chown root:root", fp)
	fmt.Println("$ sudo chmod u+s", fp)
	fmt.Println("===================================")
	fmt.Println("(to return to your shell prompt just press Return)")
	return
}

// Post-rollback hook for rollbackcmd.
func PostRollbackHook(f fsdir.T) (err error) {
	// do the same thing but with the old binary on rollback
	log.Println("unpacking old embedded files...")
	if err = cli.RunChild(f.Path("mercury"), "init", "--force-unpack-only"); err != nil {
		return
	}
	fp := f.Path("mercury_tun")
	fmt.Println("===================================")
	fmt.Println("NOTE: to enable mercury_tun again:")
	fmt.Println("$ sudo chown root:root", fp)
	fmt.Println("$ sudo chmod u+s", fp)
	fmt.Println("===================================")
	fmt.Println("(to return to your shell prompt just press Return)")
	return
}

// MIGRATIONS is the slice of versioned migrations.
var MIGRATIONS = []*upgrade.Migration{}

// LatestChannelVersion is a special function for mercury which will obtain
// the latest version supported by the currently configured update channel from
// the directory.
func LatestChannelVersion(f fsdir.T) (_ semver.Version, err error) {
	// check if running mercury or mercury_tun
	if err = cli.RunChild(f.Path("mercury"), "tun", "status"); err == nil {
		err = fmt.Errorf("mercury_tun appears to be running, please stop it to upgrade")
		return
	}
	if err = cli.RunChild(f.Path("mercury"), "status"); err == nil {
		err = fmt.Errorf("mercury appears to be running, please stop it to upgrade")
		return
	}
	c := clientcfg.Defaults()
	if err = f.Get(&c, filenames.Config); err != nil {
		return
	}
	if c.Contract == nil {
		err = fmt.Errorf("`contract` field in config is empty, setup a contract with `mercury import`")
		return
	}
	cl := client.New(nil, auth.Client)
	dinfo, err := consume.DirectoryInfo(cl, c.Contract)
	if err != nil {
		return
	}
	v, ok := dinfo.Channels[Channel]
	if !ok {
		err = fmt.Errorf("no version for channel '%s' is provided by directory", Channel)
		return
	}
	return v, nil
}
