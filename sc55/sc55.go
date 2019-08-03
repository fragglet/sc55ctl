// Package sc55 is a library for generating SC-55 SysEx messages.
package sc55

type DeviceID byte

// Register represents a SoundCanvas memory register.
type Register struct {
	Address, Size int
	Min, Max      int
	Zero          int
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
	MasterTune     = &Register{0x400000, 4, 0x18, 0x7e8, 0x400}
	MasterVolume   = &Register{0x400004, 1, 0x00, 0x7f, 0}
	MasterKeyShift = &Register{0x400005, 1, 0x28, 0x58, 0x40}
	MasterPan      = &Register{0x400006, 1, 0x01, 0x7f, 0x40}
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
