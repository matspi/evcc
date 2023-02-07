package charger

// LICENSE

// Copyright (c) 2022 matspi

// This module is NOT covered by the MIT license. All rights reserved.

// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

import (
	"encoding/binary"
	"fmt"

	"github.com/evcc-io/evcc/api"
	"github.com/evcc-io/evcc/util"
	"github.com/evcc-io/evcc/util/modbus"
)

// Amtron Professional charger implementation
type AmtronProfessional struct {
	conn *modbus.Connection
	curr uint16
}

const (
	amtronProfessionalRegEnergy     = 200
	amtronProfessionalRegCurrent    = 212
	amtronProfessionalRegPower      = 206
	amtronProfessionalRegStatus     = 122
	amtronProfessionalRegAmpsConfig = 1000
	amtronProfessionalRegEVCCID     = 741
)

func init() {
	registry.Add("amtron_professional", NewAmtronProfessionalFromConfig)
}

// NewAmtronProfessionalFromConfig creates a Mennekes Amtron Professional charger from generic config
func NewAmtronProfessionalFromConfig(other map[string]interface{}) (api.Charger, error) {
	cc := modbus.TcpSettings{
		ID: 2,
	}

	if err := util.DecodeOther(other, &cc); err != nil {
		return nil, err
	}

	return NewAmtronProfessional(cc.URI, cc.ID)
}

// NewAmtron creates Amtron charger
func NewAmtronProfessional(uri string, slaveID uint8) (api.Charger, error) {
	uri = util.DefaultPort(uri, 502)

	conn, err := modbus.NewConnection(uri, "", "", 0, modbus.Tcp, slaveID)
	if err != nil {
		return nil, err
	}

	log := util.NewLogger("amtron_professional")
	conn.Logger(log.TRACE)

	wb := &AmtronProfessional{
		conn: conn,
		curr: 6,
	}

	return wb, err
}

// Status implements the api.Charger interface
func (wb *AmtronProfessional) Status() (api.ChargeStatus, error) {
	b, err := wb.conn.ReadHoldingRegisters(amtronProfessionalRegStatus, 1)
	if err != nil {
		return api.StatusNone, err
	}

	switch b[1] {
	case 1:
		return api.StatusA, nil
	case 2:
		return api.StatusB, nil
	case 3:
		return api.StatusC, nil
	case 4:
		return api.StatusD, nil
	case 5:
		return api.StatusE, nil
	case 6:
		return api.StatusF, nil
	default:
		return api.StatusNone, fmt.Errorf("invalid status: %d", b)
	}
}

// Enabled implements the api.Charger interface
func (wb *AmtronProfessional) Enabled() (bool, error) {
	b, err := wb.conn.ReadHoldingRegisters(amtronProfessionalRegAmpsConfig, 1)
	if err != nil {
		return false, err
	}

	u := binary.BigEndian.Uint16(b)

	return u != 0, nil
}

// Enable implements the api.Charger interface
func (wb *AmtronProfessional) Enable(enable bool) error {
	var err error
	if enable {
		err = wb.MaxCurrent(int64(wb.curr))
	} else {
		err = wb.MaxCurrent(0)
	}

	return err
}

// MaxCurrent implements the api.Charger interface
func (wb *AmtronProfessional) MaxCurrent(current int64) error {
	cur := uint16(current)

	_, err := wb.conn.WriteSingleRegister(amtronProfessionalRegAmpsConfig, cur)
	if err == nil {
		if cur > 0 {
			wb.curr = cur
		}
	}

	return err
}

var _ api.Meter = (*AmtronProfessional)(nil)

// CurrentPower implements the api.Meter interface
func (wb *AmtronProfessional) CurrentPower() (float64, error) {
	l1, err := wb.conn.ReadHoldingRegisters(amtronProfessionalRegPower, 2)
	if err != nil {
		return 0, err
	}
	var l1Power uint32 = toUint32(l1)

	l2, err := wb.conn.ReadHoldingRegisters(amtronProfessionalRegPower+2, 2)
	if err != nil {
		return 0, err
	}
	var l2Power uint32 = toUint32(l2)

	l3, err := wb.conn.ReadHoldingRegisters(amtronProfessionalRegPower+4, 2)
	if err != nil {
		return 0, err
	}
	var l3Power uint32 = toUint32(l3)

	return float64(l1Power + l2Power + l3Power), err
}

func toUint32(b []byte) uint32 {
	if len(b) < 4 {
		return 0
	}
	return uint32(binary.LittleEndian.Uint16(b)*256) + uint32(binary.BigEndian.Uint16(b[2:]))
}

var _ api.PhaseCurrents = (*AmtronProfessional)(nil)

// Currents implements the api.MeterCurrent interface
func (wb *AmtronProfessional) Currents() (float64, float64, float64, error) {
	l1, err := wb.conn.ReadHoldingRegisters(amtronProfessionalRegCurrent, 2)
	var l1Curr = toUint32(l1)
	if err != nil {
		return 0, 0, 0, err
	}
	l2, err := wb.conn.ReadHoldingRegisters(amtronProfessionalRegCurrent+2, 2)
	var l2Curr = toUint32(l2)
	if err != nil {
		return 0, 0, 0, err
	}
	l3, err := wb.conn.ReadHoldingRegisters(amtronProfessionalRegCurrent+4, 2)
	var l3Curr = toUint32(l3)
	if err != nil {
		return 0, 0, 0, err
	}

	return float64(l1Curr) / 1e3, float64(l2Curr) / 1e3, float64(l3Curr) / 1e3, err
}
