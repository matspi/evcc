package charger

import (
	"encoding/binary"
	"fmt"

	"github.com/evcc-io/evcc/api"
	"github.com/evcc-io/evcc/util"
	"github.com/evcc-io/evcc/util/modbus"
)

// AmtronProfessional Professional charger implementation
type AmtronProfessional struct {
	conn *modbus.Connection
}

const (
	amtronRegEnergy     = 200
	amtronRegCurrent    = 212
	amtronRegPower      = 206
	amtronRegStatus     = 122
	amtronRegAmpsConfig = 1000
	amtronRegEVCCID     = 741
)

func init() {
	registry.Add("amtronprof", NewAmtronProfessionalFromConfig)
}

// NewAmtronProfessionalFromConfig creates a Mennekes Amtron Professional charger from generic config
func NewAmtronProfessionalFromConfig(other map[string]interface{}) (api.Charger, error) {
	cc := modbus.Settings{
		ID: 2,
	}

	if err := util.DecodeOther(other, &cc); err != nil {
		return nil, err
	}

	return NewAmtronProfessional(cc.URI, cc.Device, cc.Comset, cc.Baudrate, cc.ID)
}

// NewAmtronProfessional creates Amtron charger
func NewAmtronProfessional(uri, device, comset string, baudrate int, slaveID uint8) (api.Charger, error) {
	conn, err := modbus.NewConnection(uri, device, comset, baudrate, modbus.TcpFormat, slaveID)
	if err != nil {
		return nil, err
	}

	log := util.NewLogger("amtron_prof")
	conn.Logger(log.TRACE)

	wb := &AmtronProfessional{
		conn: conn,
	}

	return wb, err
}

// Status implements the api.Charger interface
func (wb *AmtronProfessional) Status() (api.ChargeStatus, error) {
	b, err := wb.conn.ReadHoldingRegisters(amtronRegStatus, 1)
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
	b, err := wb.conn.ReadHoldingRegisters(amtronRegAmpsConfig, 1)
	if err != nil {
		return false, err
	}

	var value uint16 = binary.LittleEndian.Uint16(b)

	return value > 0, nil
}

// Enable implements the api.Charger interface
func (wb *AmtronProfessional) Enable(enable bool) error {
	var err error
	if !enable {
		err = wb.MaxCurrent(0)
	} else {
		err = wb.MaxCurrent(16)
	}

	return err
}

// MaxCurrent implements the api.Charger interface
func (wb *AmtronProfessional) MaxCurrent(current int64) error {
	_, err := wb.conn.WriteSingleRegister(amtronRegAmpsConfig, uint16(current))

	return err
}

var _ api.MeterEnergy = (*AmtronProfessional)(nil)

// TotalEnergy implements the api.MeterEnergy interface
func (wb *AmtronProfessional) TotalEnergy() (float64, error) {
	l1, err := wb.conn.ReadHoldingRegisters(amtronRegEnergy, 2)
	if err != nil {
		return 0, err
	}
	var l1Energy = toUint32(l1)

	l2, err := wb.conn.ReadHoldingRegisters(amtronRegEnergy+2, 2)
	if err != nil {
		return 0, err
	}
	var l2Energy = toUint32(l2)
	if l2Energy == 0xffffffff {
		l2Energy = 0
	}

	l3, err := wb.conn.ReadHoldingRegisters(amtronRegEnergy+4, 2)
	if err != nil {
		return 0, err
	}
	var l3Energy = toUint32(l3)
	if l3Energy == 0xffffffff {
		l3Energy = 0
	}

	return float64(l1Energy + l2Energy + l3Energy), err
}

func toUint32(b []byte) uint32 {
	return uint32(binary.LittleEndian.Uint16(b)*256) + uint32(binary.BigEndian.Uint16(b[2:]))
}

var _ api.MeterCurrent = (*AmtronProfessional)(nil)

// Currents implements the api.MeterCurrent interface
func (wb *AmtronProfessional) Currents() (float64, float64, float64, error) {
	l1, err := wb.conn.ReadHoldingRegisters(amtronRegCurrent, 2)
	var l1Curr = toUint32(l1)
	if err != nil {
		return 0, 0, 0, err
	}
	l2, err := wb.conn.ReadHoldingRegisters(amtronRegCurrent+2, 2)
	var l2Curr = toUint32(l2)
	if err != nil {
		return 0, 0, 0, err
	}
	l3, err := wb.conn.ReadHoldingRegisters(amtronRegCurrent+4, 2)
	var l3Curr = toUint32(l3)
	if err != nil {
		return 0, 0, 0, err
	}

	return float64(l1Curr) / 1e3, float64(l2Curr) / 1e3, float64(l3Curr) / 1e3, err
}

var _ api.Meter = (*AmtronProfessional)(nil)

func (wb *AmtronProfessional) CurrentPower() (float64, error) {
	l1, err := wb.conn.ReadHoldingRegisters(amtronRegPower, 2)
	if err != nil {
		return 0, err
	}
	var l1Power uint32 = toUint32(l1)

	l2, err := wb.conn.ReadHoldingRegisters(amtronRegPower+2, 2)
	if err != nil {
		return 0, err
	}
	var l2Power uint32 = toUint32(l2)

	l3, err := wb.conn.ReadHoldingRegisters(amtronRegPower+4, 2)
	if err != nil {
		return 0, err
	}
	var l3Power uint32 = toUint32(l3)

	return float64(l1Power + l2Power + l3Power), err
}
