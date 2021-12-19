package vehicle

import (
	"fmt"
	"strings"
	"time"

	"github.com/evcc-io/evcc/api"
	"github.com/evcc-io/evcc/util"
	"github.com/evcc-io/evcc/util/logx"
	"github.com/evcc-io/evcc/vehicle/nissan"
)

// Credits to
//   https://github.com/Tobiaswk/dartnissanconnect
//   https://github.com/mitchellrj/kamereon-python
//   https://gitlab.com/tobiaswkjeldsen/carwingsflutter

// OAuth base url
// 	 https://prod.eu.auth.kamereon.org/kauth/oauth2/a-ncb-prod/.well-known/openid-configuration

// Nissan is an api.Vehicle implementation for Nissan cars
type Nissan struct {
	*embed
	*nissan.Provider
}

func init() {
	registry.Add("nissan", NewNissanFromConfig)
}

// NewNissanFromConfig creates a new vehicle
func NewNissanFromConfig(other map[string]interface{}) (api.Vehicle, error) {
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

	v := &Nissan{
		embed: &cc.embed,
	}

	log := logx.Redact(logx.NewModule("nissan"), cc.User, cc.Password, cc.VIN)
	identity := nissan.NewIdentity(log)

	if err := identity.Login(cc.User, cc.Password); err != nil {
		return v, fmt.Errorf("login failed: %w", err)
	}

	api := nissan.NewAPI(log, identity)

	var err error
	if cc.VIN == "" {
		cc.VIN, err = findVehicle(api.Vehicles())
		if err == nil {
			logx.Debug(log, "msg", "found vehicle", "vin", cc.VIN)
		}
	}

	v.Provider = nissan.NewProvider(api, strings.ToUpper(cc.VIN), cc.Expiry, cc.Cache)

	return v, err
}
