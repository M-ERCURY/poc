package clientlib

import (
	"fmt"

	"github.com/M-ERCURY/core/api/client"
	"github.com/M-ERCURY/core/api/consume"
	"github.com/M-ERCURY/core/api/contractinfo"
	"github.com/M-ERCURY/core/api/relaylist"
	"github.com/M-ERCURY/core/api/texturl"
	"github.com/M-ERCURY/core/cli/fsdir"
	"github.com/M-ERCURY/poc/filenames"
)

func GetContractInfo(cl *client.Client, sc *texturl.URL) (info *contractinfo.T, rl relaylist.T, err error) {
	if info, err = consume.ContractInfo(cl, sc); err != nil {
		err = fmt.Errorf(
			"could not get contract info for %s: %s",
			sc.String(), err,
		)
		return
	}
	if rl, err = consume.ContractRelays(cl, sc); err != nil {
		err = fmt.Errorf(
			"could not get contract relays for %s: %s",
			sc.String(), err,
		)
	}
	return
}

func SaveContractInfo(fm fsdir.T, ci *contractinfo.T, rl relaylist.T) (err error) {
	if err = fm.Set(ci, filenames.Contract); err != nil {
		return fmt.Errorf("could not save contract info: %s", err)
	}
	if err = fm.Set(rl, filenames.Relays); err != nil {
		return fmt.Errorf("could not save contract relays: %s", err)
	}
	return
}
