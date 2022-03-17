package charger

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/evcc-io/evcc/api"
	"github.com/evcc-io/evcc/util"
	"github.com/evcc-io/evcc/util/modbus"
)

// AmtronProfessional Professional charger implementation
type AmtronProfessional struct {
	log     *util.Logger
	conn    *modbus.Connection
	current int64
}

const (
	amtronProfRegEnergy     = 200
	amtronProfRegCurrent    = 212
	amtronProfRegPower      = 206
	amtronProfRegStatus     = 122
	amtronProfRegAmpsConfig = 1000
	amtronProfRegEVCCID     = 741
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
	conn, err := modbus.NewConnection(uri, device, comset, baudrate, modbus.Tcp, slaveID)
	if err != nil {
		return nil, err
	}

	log := util.NewLogger("amtron_prof")
	conn.Logger(log.TRACE)

	wb := &AmtronProfessional{
		log:     log,
		conn:    conn,
		current: 6,
	}

	return wb, err
}

// Status implements the api.Charger interface
func (wb *AmtronProfessional) Status() (api.ChargeStatus, error) {
	wb.log.DEBUG.Println("Status")
	b, err := wb.conn.ReadHoldingRegisters(amtronProfRegStatus, 1)
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
	wb.log.DEBUG.Println("Enabled")
	b, err := wb.conn.ReadHoldingRegisters(amtronProfRegAmpsConfig, 1)
	if err != nil {
		return false, err
	}

	var value uint16 = binary.BigEndian.Uint16(b)

	wb.log.DEBUG.Println("Read from box: ", value)

	return value != 0, nil
}

// Enable implements the api.Charger interface
 func (wb *AmtronProfessional) Enable(enable bool) error {
 	var u uint16
 	if enable {
 		u = 0x04
 		u = wb.current
 	}
	 
 	_, err := wb.conn.WriteSingleRegister(amtronProfRegCurrent, u)
 	return err
 }

// MaxCurrent implements the api.Charger interface
func (wb *AmtronProfessional) MaxCurrent(current int64) error {
	wb.log.DEBUG.Println("MaxCurrent")
	_, err := wb.conn.WriteSingleRegister(amtronProfRegAmpsConfig, uint16(current))
	if err == nil {
		wb.current = current
	}
	return err
}

var _ api.MeterEnergy = (*AmtronProfessional)(nil)

// TotalEnergy implements the api.MeterEnergy interface
func (wb *AmtronProfessional) TotalEnergy() (float64, error) {
	l1, err := wb.conn.ReadHoldingRegisters(amtronProfRegEnergy, 2)
	if err != nil {
		return 0, err
	}
	var l1Energy = toUint32(l1)

	l2, err := wb.conn.ReadHoldingRegisters(amtronProfRegEnergy+2, 2)
	if err != nil {
		return 0, err
	}
	var l2Energy = toUint32(l2)
	if bytes.Equal(l2, []byte{0xff, 0xff, 0xff, 0xff}) {
		l2Energy = 0
	}

	l3, err := wb.conn.ReadHoldingRegisters(amtronProfRegEnergy+4, 2)
	if err != nil {
		return 0, err
	}
	var l3Energy = toUint32(l3)
	if bytes.Equal(l3, []byte{0xff, 0xff, 0xff, 0xff}) {
		l3Energy = 0
	}

	return float64(l1Energy+l2Energy+l3Energy) / 1e3, err
}

func toUint32(b []byte) uint32 {
	if len(b) < 4 {
		return 0
	}
	return uint32(binary.LittleEndian.Uint16(b)*256) + uint32(binary.BigEndian.Uint16(b[2:]))
}

var _ api.MeterCurrent = (*AmtronProfessional)(nil)

// Currents implements the api.MeterCurrent interface
func (wb *AmtronProfessional) Currents() (float64, float64, float64, error) {
	l1, err := wb.conn.ReadHoldingRegisters(amtronProfRegCurrent, 2)
	var l1Curr = toUint32(l1)
	if err != nil {
		return 0, 0, 0, err
	}
	l2, err := wb.conn.ReadHoldingRegisters(amtronProfRegCurrent+2, 2)
	var l2Curr = toUint32(l2)
	if err != nil {
		return 0, 0, 0, err
	}
	l3, err := wb.conn.ReadHoldingRegisters(amtronProfRegCurrent+4, 2)
	var l3Curr = toUint32(l3)
	if err != nil {
		return 0, 0, 0, err
	}

	return float64(l1Curr) / 1e3, float64(l2Curr) / 1e3, float64(l3Curr) / 1e3, err
}

var _ api.Meter = (*AmtronProfessional)(nil)

func (wb *AmtronProfessional) CurrentPower() (float64, error) {
	l1, err := wb.conn.ReadHoldingRegisters(amtronProfRegPower, 2)
	if err != nil {
		return 0, err
	}
	var l1Power uint32 = toUint32(l1)

	l2, err := wb.conn.ReadHoldingRegisters(amtronProfRegPower+2, 2)
	if err != nil {
		return 0, err
	}
	var l2Power uint32 = toUint32(l2)

	l3, err := wb.conn.ReadHoldingRegisters(amtronProfRegPower+4, 2)
	if err != nil {
		return 0, err
	}
	var l3Power uint32 = toUint32(l3)

	return float64(l1Power + l2Power + l3Power), err
}
