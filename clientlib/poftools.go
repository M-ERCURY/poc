package clientlib

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/M-ERCURY/core/api/accesskey"
	"github.com/M-ERCURY/core/api/client"
	"github.com/M-ERCURY/core/api/pof"
	"github.com/M-ERCURY/core/api/servicekey"
	"github.com/M-ERCURY/core/api/status"
	"github.com/M-ERCURY/core/cli/fsdir"
	"github.com/M-ERCURY/poc/clientcfg"
	"github.com/M-ERCURY/poc/filenames"
)

var (
	ErrThereIsNotPof = errors.New("there is no pof available")
)

const (
	maxPofRetryCount = 3
	maxSkRetryCount  = 3
)

type SKSourceFunc func(bool) (*servicekey.T, error)

// function to get a fresh sk if at all possible
func SKSource(fm fsdir.T, c *clientcfg.C, cl *client.Client) SKSourceFunc {
	var sk *servicekey.T
	return func(fetch bool) (r *servicekey.T, err error) {
		if sk == nil {
			fm.Get(&sk, "servicekey.json")
		}
		if sk != nil && sk.Contract != nil && !sk.IsExpiredAt(time.Now().Unix()) {
			log.Printf(
				"found existing servicekey %s",
				sk.PublicKey,
			)
			return sk, nil
		}
		if !c.Accesskey.UseOnDemand {
			return nil, fmt.Errorf("no fresh servicekey available and accesskey.use_on_demand is false")
		}
		if !fetch {
			return nil, fmt.Errorf("no activated servicekey available")
		}
		// discard old servicekey & get a new one
		sk, err = RefreshSK(fm, c.PofURL, func(p *pof.T) (*servicekey.T, error) {
			if c.Contract == nil {
				return nil, fmt.Errorf("no contract defined")
			}
			return NewSKFromPof(
				cl,
				c.Contract.String()+"/servicekey/activate",
				p,
			)
		})
		return sk, err
	}
}

type AlwaysFetchFunc func() (*servicekey.T, error)

func AlwaysFetch(f SKSourceFunc) AlwaysFetchFunc {
	return func() (*servicekey.T, error) { return f(true) }
}

func NewSKFromPof(cl *client.Client, skurl string, p *pof.T) (*servicekey.T, error) {
	_, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		return nil, err
	}

	sk := servicekey.New(priv)
	req := &pof.SKActivationRequest{Pubkey: sk.PublicKey, Pof: p}
	if err = cl.Perform(http.MethodPost, skurl, req, sk.Contract); err != nil {
		return nil, fmt.Errorf("error while performing SK activation request: %w", err)
	}

	return sk, nil
}

func PickPofs(pofs ...*pof.T) (r []*pof.T) {
	for _, p := range pofs {
		if !p.IsExpiredAt(time.Now().Unix()) {
			// this one has not expired yet
			r = append(r, p)
		}
	}

	return r
}

func PickSK(sks ...*servicekey.T) (sk *servicekey.T) {
	for _, k := range sks {
		if !k.IsExpiredAt(time.Now().Unix()) {
			// this one has not expired yet
			sk = k
			break
		}
	}

	return
}

type Activator func(*pof.T) (*servicekey.T, error)

func ValidateAndRecievePofs(fm fsdir.T) ([]*pof.T, error) {
	ps := []*pof.T{}
	if err := fm.Get(&ps, filenames.Pofs); err != nil {
		return ps, fmt.Errorf("could not open %s: %s; did you run `mercury import`?", filenames.Pofs, err)
	}

	ps = PickPofs(ps...)
	if len(ps) == 0 {
		return ps, fmt.Errorf("no fresh pofs available")
	}

	return ps, nil
}

func RefreshSK(fm fsdir.T, pofURL string, actf Activator) (*servicekey.T, error) {
	skRetryCount := 0
	var ps []*pof.T
	var sk *servicekey.T
	var err error

	for {
		skRetryCount++

		ps, err = receivePofs(fm, pofURL)
		if err != nil {
			return nil, err
		}

		sk, err = generateNewSk(fm, actf, ps)
		if errors.Is(err, ErrThereIsNotPof) {
			if skRetryCount >= maxSkRetryCount {
				fmt.Println("generateNewSk error", err)

				return nil, err
			}

			continue
		}

		return sk, err
	}
}

func receivePofs(fm fsdir.T, pofURL string) ([]*pof.T, error) {
	var ps []*pof.T

	var err error
	pofRetryCount := 0
	for {
		pofRetryCount++

		ps, err = ValidateAndRecievePofs(fm)
		if err != nil {
			if pofRetryCount >= maxPofRetryCount {
				fmt.Println("ValidateAndRecievePofs error", err)

				return nil, err
			}

			if err := UpdateServiceKey(fm, pofURL); err != nil {
				fmt.Println("RefreshSK UpdateServiceKey error", err)
			}

			continue
		}

		break
	}

	return ps, nil
}

func generateNewSk(fm fsdir.T, actf Activator, ps []*pof.T) (*servicekey.T, error) {
	var sk *servicekey.T
	var err error

	newps := []*pof.T{}
	// filter pofs & get sk
	for _, p := range ps {
		if sk == nil {
			log.Printf("generating new servicekey from pof %s...", p.Digest())

			sk, err = actf(p)
			if err != nil {
				log.Printf(
					"failed generating new servicekey from pof %s: %s",
					p.Digest(),
					err,
				)

				// TODO
				if errors.Is(err, status.ErrSneakyPof) {
					// need to remove this pof

					// skip already used pof
					continue
				}

				// TODO combine
				if errors.Is(err, status.SneakyPofErr) {
					// need to remove this pof

					// skip expired pof
					continue
				}

				// keep if other error
				newps = append(newps, p)
				continue
			}
			// skip successfully-used pof
			continue
		}

		// keep the rest untouched
		newps = append(newps, p)
	}

	// write new pofs
	if err = fm.Set(&newps, filenames.Pofs); err != nil {
		return nil, fmt.Errorf("could not write new %s: %s", filenames.Pofs, err)
	}

	if len(newps) == 0 && sk == nil {
		// generate new pof
		return nil, ErrThereIsNotPof
	}

	if sk == nil {
		return nil, fmt.Errorf("no servicekey available")
	}

	// write new servicekey
	if err = fm.Set(&sk, filenames.Servicekey); err != nil {
		return nil, fmt.Errorf("could not write new %s: %s", filenames.Servicekey, err)
	}

	return sk, nil
}

func UpdateServiceKey(fm fsdir.T, url string) error {
	fmt.Println("Refreshing servicekey...")
	c := clientcfg.Defaults()

	// TODO do we need to do this?
	err := fm.Get(&c, filenames.Config)
	if err != nil {
		return fmt.Errorf("UpdateServiceKey fm.Get(&c, filenames.Config), err: %w", err)
	}

	data, err := download(url)
	if err != nil {
		return fmt.Errorf("UpdateServiceKey download(url), err: %w", err)
	}

	ak := &accesskey.T{}
	if err = json.Unmarshal(data, &ak); err != nil {
		return fmt.Errorf("UpdateServiceKey json.Unmarshal(data, &ak), err: %w", err)
	}

	if ak.Contract == nil || ak.Pofs == nil || ak.Contract.Endpoint == nil || ak.Contract.PublicKey == nil {
		return fmt.Errorf("malformed accesskey file")
	}

	if c.Contract == nil {
		c.Contract = ak.Contract.Endpoint

		if err = fm.Set(&c, filenames.Config); err != nil {
			return fmt.Errorf("could not save config.json with Contract=%s: %w", c.Contract.String(), err)
		}
	}

	if *c.Contract != *ak.Contract.Endpoint {
		err := fmt.Errorf(
			"you are trying to import accesskeys for a contract %s different from the currently defined %s",
			ak.Contract.Endpoint,
			c.Contract,
		)

		return err
	}

	cl := client.New(nil, "Client")
	ci, d, err := GetContractInfo(cl, ak.Contract.Endpoint)
	if err != nil {
		return fmt.Errorf("could not get contract info for %s: %w", ak.Contract.Endpoint, err)
	}

	if !bytes.Equal(ak.Contract.PublicKey, ci.Pubkey) {
		return fmt.Errorf("contract public key mismatch; expecting %s from accesskey file, got %s from live contract", ak.Contract.PublicKey, base64.RawURLEncoding.EncodeToString(ci.Pubkey))
	}

	if err = SaveContractInfo(fm, ci, d); err != nil {
		return fmt.Errorf("could not save contract info for %s: %w", ak.Contract.Endpoint, err)
	}

	pofs := []*pof.T{}
	if err = fm.Get(&pofs, filenames.Pofs); errors.Is(err, io.EOF) || errors.Is(err, os.ErrNotExist) {
		// this is fine
		err = nil
	}

	if err != nil {
		return fmt.Errorf("could not get previous pofs for %s: %w", c.Contract.String(), err)
	}

	for _, p := range ak.Pofs {
		if p.Expiration <= time.Now().Unix() {
			log.Printf("skipping expired accesskey %s", p.Digest())

			continue
		}

		dup := false
		for _, p0 := range pofs {
			if p0.Digest() == p.Digest() {
				log.Printf("skipping duplicate accesskey %s", p.Digest())
				dup = true

				break
			}
		}

		if !dup {
			pofs = append(pofs, p)
		}
	}

	if err = fm.Set(pofs, filenames.Pofs); err != nil {
		return fmt.Errorf("could not save new pofs for %s: %w", c.Contract.String(), err)
	}

	return nil
}

func download(url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	client := http.Client{Timeout: 10 * time.Second}

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"%s download request returned code %d: %s",
			url, res.StatusCode, res.Status,
		)
	}

	log.Printf("Downloading %s...", url)

	return io.ReadAll(res.Body)
}
