// Package sc55 is a library for generating SC-55 SysEx messages.
package sc55

import (
	"fmt"
	"image"
	"reflect"
	"sort"
)

// DeviceID represents the address of an SC-55 so that multiple can be
// present on the same MIDI bus. Usually "DefaultDevice" should be used.
type DeviceID byte

// Register represents a SoundCanvas memory register.
type Register struct {
	Address, Size int
	Min, Max      int
	Zero          int
}

// Part represents the set of registers associated with a part.
type Part struct {
	ToneNumberCC        Register `name:"tone-number-cc"`
	ToneNumberPC        Register `name:"tone-number-cc"`
	RxChannel           Register `name:"rx-channel"`
	RxPitchBend         Register `name:"rx-pitch-bend"`
	RxChPressure        Register `name:"rx-ch-pressure"`
	RxProgramChange     Register `name:"rx-program-change"`
	RxControlChange     Register `name:"rx-control-change"`
	RxPolyPressure      Register `name:"rx-poly-pressure"`
	RxNoteMessage       Register `name:"rx-note-message"`
	RxRPN               Register `name:"rx-rpn"`
	RxNRPN              Register `name:"rx-nrpn"`
	RxModulation        Register `name:"rx-modulation"`
	RxVolume            Register `name:"rx-volume"`
	RxPanPot            Register `name:"rx-pan-pot"`
	RxExpression        Register `name:"rx-expression"`
	RxHoldi             Register `name:"rx-holdi"`
	RxPortamento        Register `name:"rx-portamento"`
	RxSostenuto         Register `name:"rx-sostenuto"`
	RxSoft              Register `name:"rx-soft"`
	MonoPolyMode        Register `name:"mono-poly-mode"`
	AssignMode          Register `name:"assign-mode"`
	UseForRhythm        Register `name:"use-for-rhythm"`
	PitchKeyShift       Register `name:"pitch-key-shift"`
	PitchOffsetFine     Register `name:"pitch-offset-fine"`
	PartLevel           Register `name:"part-level"`
	VelocitySenseDepth  Register `name:"velocity-sense-depth"`
	VelocitySenseOffset Register `name:"velocity-sense-offset"`
	PanPot              Register `name:"pan-pot"`
	KeyRangeLow         Register `name:"key-range-low"`
	KeyRangeHigh        Register `name:"key-range-high"`
	CC1Controller       Register `name:"cc-1-controller"`
	CC2Controller       Register `name:"cc-2-controller"`
	ChorusSendLevel     Register `name:"chorus-send-level"`
	ReverbSendLevel     Register `name:"reverb-send-level"`
	RxBankSelect        Register `name:"rx-bank-select"`
	ToneModify1         Register `name:"tone-modify-1"`
	ToneModify2         Register `name:"tone-modify-2"`
	ToneModify3         Register `name:"tone-modify-3"`
	ToneModify4         Register `name:"tone-modify-4"`
	ToneModify5         Register `name:"tone-modify-5"`
	ToneModify6         Register `name:"tone-modify-6"`
	ToneModify7         Register `name:"tone-modify-7"`
	ToneModify8         Register `name:"tone-modify-8"`
	ScaleTuningC        Register `name:"scale-tuning-c"`
	ScaleTuningCSharp   Register `name:"scale-tuning-cs"`
	ScaleTuningD        Register `name:"scale-tuning-d"`
	ScaleTuningDSharp   Register `name:"scale-tuning-ds"`
	ScaleTuningE        Register `name:"scale-tuning-e"`
	ScaleTuningF        Register `name:"scale-tuning-f"`
	ScaleTuningFSharp   Register `name:"scale-tuning-fs"`
	ScaleTuningG        Register `name:"scale-tuning-g"`
	ScaleTuningGSharp   Register `name:"scale-tuning-gs"`
	ScaleTuningA        Register `name:"scale-tuning-a"`
	ScaleTuningASharp   Register `name:"scale-tuning-as"`
	ScaleTuningB        Register `name:"scale-tuning-b"`
}

const (
	// DefaultDevice is the default device ID unless otherwise configured.
	DefaultDevice = DeviceID(0x10)

	manufacturerID = 0x41

	sysExStart = 0xf0
	sysExEnd   = 0xf7
)

const (
	cmdRQ1 = 0x11
	cmdDT1 = 0x12
)

const (
	AddrDisplayMessage = 0x100000
	AddrDisplayImage   = 0x100100

	AddrModeSet = 0x40007F
)

var (
	MasterTune     = Register{0x400000, 4, 0x18, 0x7e8, 0x400}
	MasterVolume   = Register{0x400004, 1, 0x00, 0x7f, 0}
	MasterKeyShift = Register{0x400005, 1, 0x28, 0x58, 0x40}
	MasterPan      = Register{0x400006, 1, 0x01, 0x7f, 0x40}

	parts [16]Part
	registersByAddress map[int]*Register
	registersByName map[string]*Register
	registerName map[*Register]string
)

func addRegister(name string, r *Register) {
	registersByName[name] = r
	registersByAddress[r.Address] = r
	registerName[r] = name
}

func checksum(data []byte) byte {
	sum := 0
	for _, b := range data {
		sum += int(b)
	}
	return byte(128 - (sum % 128))
}

func modelID(addr int) byte {
	if addr < MasterTune.Address {
		return 0x45
	}
	return 0x42
}

func marshalInt24(val int) []byte {
	return []byte{
		// Address:
		byte((val >> 16) & 0xff),
		byte((val >> 8) & 0xff),
		byte(val & 0xff),
	}
}

func unmarshalInt24(data []byte) int {
	return (int(data[0]) << 16) | (int(data[1]) << 8) | int(data[2])
}

// DataSet returns an SC-55 DT1 command that sets the value of a range
// of memory in the SC-55.
func DataSet(device DeviceID, addr int, data ...byte) []byte {
	// A different model ID is used for different address ranges:
	body := marshalInt24(addr)
	body = append(body, data...)
	msg := []byte{sysExStart, manufacturerID, byte(device), modelID(addr), cmdDT1}
	msg = append(msg, body...)
	msg = append(msg, checksum(body))
	msg = append(msg, sysExEnd)
	return msg
}

// DataGet returns an SC-55 RQ1 command that requests the contents of a range
// of memory in the SC-55.
func DataGet(device DeviceID, addr, size int) []byte {
	body := marshalInt24(addr)
	body = append(body, marshalInt24(size)...)
	msg := []byte{sysExStart, manufacturerID, byte(device), modelID(addr), cmdRQ1}
	msg = append(msg, body...)
	msg = append(msg, checksum(body))
	msg = append(msg, sysExEnd)
	return msg
}

// UnmarshalSet decodes a DT1 command, returning the device ID of the device that
// sent it, the address, and value.
func UnmarshalSet(msg []byte) (DeviceID, int, []byte, error) {
	switch {
	case msg[0] != sysExStart || msg[len(msg) - 1] != sysExEnd:
		return 0, 0, nil, fmt.Errorf("failed to unmarshal: not a SysEx command")
	case msg[1] != manufacturerID:
		return 0, 0, nil, fmt.Errorf("wrong manufacturer: want %02x, got %02x", manufacturerID, msg[1])
	case msg[3] != 0x42 && msg[3] != 0x45:
		return 0, 0, nil, fmt.Errorf("wrong device: want 0x42 or 0x45, got %02x", msg[3])
	case msg[4] != cmdDT1:
		return 0, 0, nil, fmt.Errorf("wrong command type, want %02x, got %02x", cmdDT1, msg[4])
	case len(msg) < 10:
		return 0, 0, nil, fmt.Errorf("DT1 command too short: len=%d", len(msg))
	}
	wantChecksum := checksum(msg[5:len(msg)-2])
	gotChecksum := msg[len(msg)-2]
	if wantChecksum != gotChecksum {
		return 0, 0, nil, fmt.Errorf("wrong checksum: calculated=%02x, got=%02x", wantChecksum, gotChecksum)
	}
	return DeviceID(msg[2]), unmarshalInt24(msg[5:8]), msg[8:len(msg)-2], nil
}

// DisplayMessage returns an SC-55 SysEx command that displays a message on the
// SC-55 front console.
func DisplayMessage(device DeviceID, msg string) []byte {
	// The data sheet says the maximum is 32, but I found that a message of
	// length 32 causes some weird screen corruption like a buffer is being
	// overflowed.
	if len(msg) > 31 {
		msg = msg[:31]
	}
	return DataSet(device, AddrDisplayMessage, []byte(msg)...)
}

// DisplayImage returns an SC-55 SysEx command that displays an image on the
// SC-55 front console. The image must be a 16x16 monochrome bitmap.
func DisplayImage(device DeviceID, img image.Image) ([]byte, error) {
	if img.Bounds() != image.Rect(0, 0, 16, 16) {
		return nil, fmt.Errorf("image to display must be 16x16 bitmap")
	}
	buf := make([]byte, 64)
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			bytenum := (x/5)*16 + y
			bitnum := uint(4 - (x % 5))
			r, g, b, _ := img.At(x, y).RGBA()
			if (r+g+b) / 3 > 0x8000 {
				buf[bytenum] |= 1 << bitnum
			}
		}
	}
	return DataSet(device, AddrDisplayImage, buf...), nil
}

// ResetGM returns an SC-55 SysEx command that sets the SC-55 into GM mode.
func ResetGM(device DeviceID) []byte {
	return []byte{
		sysExStart,
		manufacturerID,
		byte(device),
		0x09, // General MIDI message
		0x01, // General MIDI on
		sysExEnd,
	}
}

// ResetGS returns an SC-55 SysEx command that sets the SC-55 into GS mode.
func ResetGS(device DeviceID) []byte {
	return DataSet(device, AddrModeSet, 0)
}

func clamp(x, min, max int) int {
	switch {
	case x < min:
		return min
	case x > max:
		return max
	default:
		return x
	}
}

// Get returns an SC-55 SysEx command to get the value of the given register.
func (r *Register) Get(device DeviceID) []byte {
	return DataGet(device, r.Address, r.Size)
}

// Set returns an SC-55 SysEx command to set the given register to the given value.
func (r *Register) Set(device DeviceID, value int) []byte {
	value = clamp(value + r.Zero, r.Min, r.Max)
	bytes := []byte{
		byte(value & 0xff),
		byte((value >> 8) & 0xff),
		byte((value >> 16) & 0xff),
		byte((value >> 24) & 0xff),
	}
	return DataSet(device, r.Address, bytes[:r.Size]...)
}

// Unmarshal decodes an SC-55 SysEx DT1 command (typically received from the SC-55
// in reply to an RQ1 message generated by Set()) and returns the value of the
// field.
func (r *Register) Unmarshal(msg []byte) (DeviceID, int, error) {
	dev, addr, payload, err := UnmarshalSet(msg)
	switch {
	case err != nil:
		return 0, 0, err
	case addr != r.Address:
		return 0, 0, fmt.Errorf("wrong register: want address %x, got %x", r.Address, addr)
	case len(payload) != r.Size:
		return 0, 0, fmt.Errorf("wrong size: want %d bytes, got %d", r.Size, len(payload))
	}
	result := 0
	for i, b := range payload {
		result |= int(b) << uint(i * 8)
	}
	if result < r.Min || result > r.Max {
		return 0, 0, fmt.Errorf("register value out of range, want %d <= x <= %d, got x=%d", r.Min, r.Max, result)
	}
	return dev, result - r.Zero, nil
}

// Name returns the name of the given register.
func (r *Register) Name() string {
	return registerName[r]
}

// RegisterByName looks up a register by name, returning register, true if it
// exists or nil, false if there is no such register.
func RegisterByName(name string) (*Register, bool) {
	r, ok := registersByName[name]
	return r, ok
}

// RegisterByAddress looks up a register by address, returning register, true
// if it exists or nil, false if there is no such register.
func RegisterByAddress(addr int) (*Register, bool) {
	r, ok := registersByAddress[addr]
	return r, ok
}

// AllRegisters returns a slice containing all known SC-55 registers, sorted
// by address.
func AllRegisters() []*Register {
	addrs := []int{}
	for a := range registersByAddress {
		addrs = append(addrs, a)
	}
	sort.IntSlice(addrs).Sort()
	result := []*Register{}
	for _, a := range addrs {
		result = append(result, registersByAddress[a])
	}
	return result
}

var templatePart = Part{
	ToneNumberCC:        Register{0x00, 1, 0x00, 0x7f, 0},
	ToneNumberPC:        Register{0x01, 1, 0x00, 0x7f, 0},
	RxChannel:           Register{0x02, 1, 0x00, 0x10, 0},
	RxPitchBend:         Register{0x03, 1, 0x00, 0x01, 0},
	RxChPressure:        Register{0x04, 1, 0x00, 0x01, 0},
	RxProgramChange:     Register{0x05, 1, 0x00, 0x01, 0},
	RxControlChange:     Register{0x06, 1, 0x00, 0x01, 0},
	RxPolyPressure:      Register{0x07, 1, 0x00, 0x01, 0},
	RxNoteMessage:       Register{0x08, 1, 0x00, 0x01, 0},
	RxRPN:               Register{0x09, 1, 0x00, 0x01, 0},
	RxNRPN:              Register{0x0a, 1, 0x00, 0x01, 0},
	RxModulation:        Register{0x0b, 1, 0x00, 0x01, 0},
	RxVolume:            Register{0x0c, 1, 0x00, 0x01, 0},
	RxPanPot:            Register{0x0d, 1, 0x00, 0x01, 0},
	RxExpression:        Register{0x0e, 1, 0x00, 0x01, 0},
	RxHoldi:             Register{0x0f, 1, 0x00, 0x01, 0},
	RxPortamento:        Register{0x10, 1, 0x00, 0x01, 0},
	RxSostenuto:         Register{0x11, 1, 0x00, 0x01, 0},
	RxSoft:              Register{0x12, 1, 0x00, 0x01, 0},
	MonoPolyMode:        Register{0x13, 1, 0x00, 0x01, 0},
	AssignMode:          Register{0x14, 1, 0x00, 0x02, 0},
	UseForRhythm:        Register{0x15, 1, 0x00, 0x02, 0},
	PitchKeyShift:       Register{0x16, 1, 0x28, 0x58, 0x40},
	PitchOffsetFine:     Register{0x17, 2, 0x08, 0xf8, 0x800},
	PartLevel:           Register{0x19, 1, 0x00, 0x7f, 0},
	VelocitySenseDepth:  Register{0x1a, 1, 0x00, 0x7f, 0},
	VelocitySenseOffset: Register{0x1b, 1, 0x00, 0x7f, 0},
	PanPot:              Register{0x1c, 1, 0x00, 0x7f, 0x40},
	KeyRangeLow:         Register{0x1d, 1, 0x00, 0x7f, 0},
	KeyRangeHigh:        Register{0x1e, 1, 0x00, 0x7f, 0},
	CC1Controller:       Register{0x1f, 1, 0x00, 0x5f, 0},
	CC2Controller:       Register{0x20, 1, 0x00, 0x5f, 0},
	ChorusSendLevel:     Register{0x21, 1, 0x00, 0x7f, 0},
	ReverbSendLevel:     Register{0x22, 1, 0x00, 0x7f, 0},
	RxBankSelect:        Register{0x23, 1, 0x00, 0x01, 0},
	ToneModify1:         Register{0x30, 1, 0x0e, 0x72, 0x40},
	ToneModify2:         Register{0x31, 1, 0x0e, 0x72, 0x40},
	ToneModify3:         Register{0x32, 1, 0x0e, 0x72, 0x40},
	ToneModify4:         Register{0x33, 1, 0x0e, 0x72, 0x40},
	ToneModify5:         Register{0x34, 1, 0x0e, 0x72, 0x40},
	ToneModify6:         Register{0x35, 1, 0x0e, 0x72, 0x40},
	ToneModify7:         Register{0x36, 1, 0x0e, 0x72, 0x40},
	ToneModify8:         Register{0x37, 1, 0x0e, 0x72, 0x40},
	ScaleTuningC:        Register{0x40, 1, 0x00, 0x7f, 0x40},
	ScaleTuningCSharp:   Register{0x41, 1, 0x00, 0x7f, 0x40},
	ScaleTuningD:        Register{0x42, 1, 0x00, 0x7f, 0x40},
	ScaleTuningDSharp:   Register{0x43, 1, 0x00, 0x7f, 0x40},
	ScaleTuningE:        Register{0x44, 1, 0x00, 0x7f, 0x40},
	ScaleTuningF:        Register{0x45, 1, 0x00, 0x7f, 0x40},
	ScaleTuningFSharp:   Register{0x46, 1, 0x00, 0x7f, 0x40},
	ScaleTuningG:        Register{0x47, 1, 0x00, 0x7f, 0x40},
	ScaleTuningGSharp:   Register{0x48, 1, 0x00, 0x7f, 0x40},
	ScaleTuningA:        Register{0x49, 1, 0x00, 0x7f, 0x40},
	ScaleTuningASharp:   Register{0x4a, 1, 0x00, 0x7f, 0x40},
	ScaleTuningB:        Register{0x4b, 1, 0x00, 0x7f, 0x40},
}

func (p *Part) init(prefix string, addr int) {
	*p = templatePart
	v := reflect.ValueOf(p).Elem()
	for i := 0; i < v.NumField(); i++ {
		name := v.Type().Field(i).Tag.Get("name")
		r := v.Field(i).Addr().Interface().(*Register)
		r.Address += addr
		addRegister(prefix + name, r)
	}
}

// PartByNumber returns the given part, looked up by number in the
// range 1-16. This corresponds to the number shown on the front panel.
func PartByNumber(i int) *Part {
	if i < 1 || i > 16 {
		return nil
	}
	return &parts[i - 1]
}

func init() {
	registersByAddress = make(map[int]*Register)
	registersByName = make(map[string]*Register)
	registerName = make(map[*Register]string)

	addRegister("master-tune", &MasterTune)
	addRegister("master-volume", &MasterVolume)
	addRegister("master-key-shift", &MasterKeyShift)
	addRegister("master-pan", &MasterPan)

	for i := range parts {
		// As per the SC-55 manual ... (yes this is silly)
		// i  #0 -> partNumber  1 -> partIndex 1
		// i  #1 -> partNumber  2 -> partIndex 2
		// ...
		// i  #9 -> partNumber 10 -> partIndex 0
		// i #10 -> partNumber 11 -> partIndex A
		// i #11 -> partNumber 12 -> partIndex B
		// ...
		// i #15 -> partNumber 16 -> partIndex F
		partNumber := i + 1
		prefix := fmt.Sprintf("part-%d.", partNumber)
		partIndex := (partNumber % 10)
		if partNumber > 10 {
			partIndex = partNumber - 1
		}
		parts[i].init(prefix, 0x401000 + partIndex*0x100)
	}
}
