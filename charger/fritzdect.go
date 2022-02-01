package charger

import (
	"encoding/xml"
	"errors"
	"strconv"
	"strings"

	"github.com/evcc-io/evcc/api"
	"github.com/evcc-io/evcc/util"
	"github.com/evcc-io/evcc/util/fritzdect"
)

// AVM FritzBox AHA interface specifications:
// https://avm.de/fileadmin/user_upload/Global/Service/Schnittstellen/AHA-HTTP-Interface.pdf

// FritzDECT charger implementation
type FritzDECT struct {
	fritzdect    *fritzdect.Connection
	standbypower float64
}

func init() {
	registry.Add("fritzdect", NewFritzDECTFromConfig)
}

// NewFritzDECTFromConfig creates a fritzdect charger from generic config
func NewFritzDECTFromConfig(other map[string]interface{}) (api.Charger, error) {
	cc := struct {
		URI          string
		AIN          string
		User         string
		Password     string
		StandbyPower float64
	}{}
	if err := util.DecodeOther(other, &cc); err != nil {
		return nil, err
	}
	return NewFritzDECT(cc.URI, cc.AIN, cc.User, cc.Password, cc.StandbyPower)
}

// NewFritzDECT creates a new connection with standbypower for charger
func NewFritzDECT(uri, ain, user, password string, standbypower float64) (*FritzDECT, error) {
	fritzdect, err := fritzdect.NewConnection(uri, ain, user, password)

	fd := &FritzDECT{
		fritzdect:    fritzdect,
		standbypower: standbypower,
	}

	return fd, err
}

// Status implements the api.Charger interface
func (c *FritzDECT) Status() (api.ChargeStatus, error) {
	resp, err := c.fritzdect.ExecCmd("getswitchpresent")

	if err == nil {
		var present bool
		present, err = strconv.ParseBool(resp)
		if err == nil && !present {
			err = api.ErrNotAvailable
		}
	}
	if err != nil {
		return api.StatusNone, err
	}

	res := api.StatusB

	// static mode
	if c.standbypower < 0 {
		on, err := c.Enabled()
		if on {
			res = api.StatusC
		}

		return res, err
	}

	// standby power mode
	power, err := c.fritzdect.CurrentPower()
	if power > c.standbypower {
		res = api.StatusC
	}

	return res, err
}

// Enabled implements the api.Charger interface
func (c *FritzDECT) Enabled() (bool, error) {
	resp, err := c.fritzdect.ExecCmd("getswitchstate")
	if err != nil {
		return false, err
	}

	if resp == "inval" {
		return false, api.ErrNotAvailable
	}

	return strconv.ParseBool(resp)
}

// Enable implements the api.Charger interface
func (c *FritzDECT) Enable(enable bool) error {
	cmd := "setswitchoff"
	if enable {
		cmd = "setswitchon"
	}

	// on 0/1 - DECT Switch state off/on (empty if unknown or error)
	resp, err := c.fritzdect.ExecCmd(cmd)

	var on bool
	if err == nil {
		on, err = strconv.ParseBool(resp)
		if err == nil && enable != on {
			err = errors.New("switch failed")
		}
	}

	return err
}

// MaxCurrent implements the api.Charger interface
func (c *FritzDECT) MaxCurrent(current int64) error {
	return nil
}

var _ api.Meter = (*FritzDECT)(nil)

// CurrentPower implements the api.Meter interface
func (c *FritzDECT) CurrentPower() (float64, error) {
	power, err := c.fritzdect.CurrentPower()
	if power < c.standbypower {
		power = 0
	}

	return power, err
}

var _ api.ChargeRater = (*FritzDECT)(nil)

// ChargedEnergy implements the api.ChargeRater interface
func (c *FritzDECT) ChargedEnergy() (float64, error) {
	// fetch basicdevicestats
	resp, err := c.fritzdect.ExecCmd("getbasicdevicestats")
	if err != nil {
		return 0, err
	}

	// unmarshal devicestats
	var stats fritzdect.Devicestats
	if err = xml.Unmarshal([]byte(resp), &stats); err != nil {
		return 0, err
	}

	// select energy value of current day
	if len(stats.Energy.Values) == 0 {
		return 0, api.ErrNotAvailable
	}

	energylist := strings.Split(stats.Energy.Values[1], ",")
	energy, err := strconv.ParseFloat(energylist[0], 64)

	return energy / 1000, err
}
