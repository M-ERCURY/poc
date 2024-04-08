package filenames

const (
	Config     = "config.json"
	Pid        = "mercury.pid"
	Servicekey = "servicekey.json"
	Pofs       = "pofs.json"
	Log        = "mercury.log"
	Bypass     = "bypass.json"
	Contract   = "contract.json"
	Relays     = "relays.json"
)

var InitFiles = [...]string{Config, Servicekey, Pofs}
