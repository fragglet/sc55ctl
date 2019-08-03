// Package sc55 is a library for generating SC-55 SysEx messages.
package sc55

import (
	"reflect"
)

type DeviceID byte

// Register represents a SoundCanvas memory register.
type Register struct {
	Address, Size int
	Min, Max      int
	Zero          int
}

// Part represents the set of registers associated with a part.
type Part struct {
	ToneNumberCC        Register
	ToneNumberPC        Register
	RxChannel           Register
	RxPitchBend         Register
	RxChPressure        Register
	RxProgramChange     Register
	RxControlChange     Register
	RxPolyPressure      Register
	RxNoteMessage       Register
	RxRPN               Register
	RxNRPN              Register
	RxModulation        Register
	RxVolume            Register
	RxPanPot            Register
	RxExpression        Register
	RxHoldi             Register
	RxPortamento        Register
	RxSostenuto         Register
	RxSoft              Register
	MonoPolyMode        Register
	AssignMode          Register
	UseForRhythm        Register
	PitchKeyShift       Register
	PitchOffsetFine     Register
	PartLevel           Register
	VelocitySenseDepth  Register
	VelocitySenseOffset Register
	PanPot              Register
	KeyRangeLow         Register
	KeyRangeHigh        Register
	CC1Controller       Register
	CC2Controller       Register
	ChorusSendLevel     Register
	ReverbSendLevel     Register
	RxBankSelect        Register
	ToneModify1         Register
	ToneModify2         Register
	ToneModify3         Register
	ToneModify4         Register
	ToneModify5         Register
	ToneModify6         Register
	ToneModify7         Register
	ToneModify8         Register
	ScaleTuningC        Register
	ScaleTuningCSharp   Register
	ScaleTuningD        Register
	ScaleTuningDSharp   Register
	ScaleTuningE        Register
	ScaleTuningF        Register
	ScaleTuningFSharp   Register
	ScaleTuningG        Register
	ScaleTuningGSharp   Register
	ScaleTuningA        Register
	ScaleTuningASharp   Register
	ScaleTuningB        Register
}

const (
	// DefaultDevice is the default device ID unless otherwise configured.
	DefaultDevice = 0x10

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

	// Parts contains the set of registers associated with each part.
	Parts [16]Part
)

func checksum(data []byte) byte {
	sum := 0
	for _, b := range data {
		sum += int(b)
	}
	return byte(128 - (sum % 128))
}

// DataSet returns an SC-55 DT1 command that sets the value of a range
// of memory in the SC-55.
func DataSet(device DeviceID, addr int, data ...byte) []byte {
	// A different model ID is used for different address ranges:
	modelID := byte(0x42)
	if addr < MasterTune.Address {
		modelID = 0x45
	}
	body := []byte{
		// Address:
		byte((addr >> 16) & 0xff),
		byte((addr >> 8) & 0xff),
		byte(addr & 0xff),
	}
	body = append(body, data...)
	msg := []byte{sysExStart, manufacturerID, byte(device), modelID, cmdDT1}
	msg = append(msg, body...)
	msg = append(msg, checksum(body))
	msg = append(msg, sysExEnd)
	return msg
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
func DisplayImage(device DeviceID, bmp [16][16]bool) []byte {
	buf := make([]byte, 64)
	for y, row := range bmp {
		for x, val := range row {
			bytenum := (x/5)*16 + y
			bitnum := uint(4 - (x % 5))
			if val {
				buf[bytenum] |= 1 << bitnum
			}
		}
	}
	return DataSet(device, AddrDisplayImage, buf...)
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

// Set returns a SC-55 SysEx command to set the given register to the given value.
func (r *Register) Set(device DeviceID, value int) []byte {
	value = clamp(value, r.Min, r.Max) + r.Zero
	bytes := []byte{
		byte(value & 0xff),
		byte((value >> 8) & 0xff),
		byte((value >> 16) & 0xff),
		byte((value >> 24) & 0xff),
	}
	return DataSet(device, r.Address, bytes[:r.Size]...)
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

func (p *Part) init(addr int) {
	*p = templatePart
	v := reflect.ValueOf(p).Elem()
	for i := 0; i < v.NumField(); i++ {
		reg := v.Field(i).Addr().Interface().(*Register)
		reg.Address += addr
	}
}

func init() {
	for i := range Parts {
		Parts[i].init(0x401000 + i*0x100)
	}
}
