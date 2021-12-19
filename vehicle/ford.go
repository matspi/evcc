package vehicle

import (
	"fmt"
	"strings"
	"time"

	"github.com/evcc-io/evcc/api"
	"github.com/evcc-io/evcc/util"
	"github.com/evcc-io/evcc/util/logx"
	"github.com/evcc-io/evcc/vehicle/ford"
)

// https://github.com/d4v3y0rk/ffpass-module

// Ford is an api.Vehicle implementation for Ford cars
type Ford struct {
	*embed
	// *request.Helper
	// log         *util.Logger
	// vin         string
	// tokenSource oauth2.TokenSource
	// statusG     func() (interface{}, error)
	// refreshId   string
	// refreshTime time.Time
	*ford.Provider
}

func init() {
	registry.Add("ford", NewFordFromConfig)
}

// NewFordFromConfig creates a new vehicle
func NewFordFromConfig(other map[string]interface{}) (api.Vehicle, error) {
	cc := struct {
		embed               `mapstructure:",squash"`
		User, Password, VIN string
		Expiry              time.Duration
		Cache               time.Duration
	}{
		Expiry: expiry,
		Cache:  interval,
	}

	if err := util.DecodeOther(other, &cc); err != nil {
		return nil, err
	}

	if cc.User == "" || cc.Password == "" {
		return nil, api.ErrMissingCredentials
	}

	v := &Ford{
		embed: &cc.embed,
	}

	log := logx.Redact(logx.NewModule("audi"), cc.User, cc.Password, cc.VIN)
	identity := ford.NewIdentity(log, cc.User, cc.Password)

	err := identity.Login()
	if err != nil {
		return nil, fmt.Errorf("login failed: %w", err)
	}

	api := ford.NewAPI(log, identity)

	if cc.VIN == "" {
		cc.VIN, err = findVehicle(api.Vehicles())
		if err == nil {
			logx.Debug(log, "msg", "found vehicle", "vin", cc.VIN)
		}
	}

	v.Provider = ford.NewProvider(api, strings.ToUpper(cc.VIN), cc.Expiry, cc.Cache)

	return v, err
}
