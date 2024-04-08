package initcmd

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/M-ERCURY/core/cli/fsdir"
)

func TestInitRun(t *testing.T) {
	tmpd, err := ioutil.TempDir("", "mrtest.*")

	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { os.RemoveAll(tmpd) })

	fm, err := fsdir.New(tmpd)

	if err != nil {
		t.Fatal(err)
	}

	Cmd().Run(fm)

	f := fm.Path("config.json")
	_, err = os.Stat(f)

	if err != nil {
		t.Errorf(
			"error while looking for file %s: %s",
			f,
			err,
		)
	}
}
