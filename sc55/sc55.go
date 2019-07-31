// Package sc55 is a library for generating SC-55 SysEx messages.
package sc55

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

	AddrMasterTune     = 0x400000
	AddrMasterVolume   = 0x400004
	AddrMasterKeyShift = 0x400005
	AddrMasterPan      = 0x400006
)

type DeviceID byte

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
	if addr < AddrMasterTune {
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

// SetXxx returns an SC-55 SysEx command that
func SetXxx(device DeviceID, param int) []byte {
	return nil
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

// SetMasterVolume returns an SC-55 SysEx command that sets the master volume
// on the SC-55 to a value in the range 0-127.
func SetMasterVolume(device DeviceID, volume int) []byte {
	volume = clamp(volume, 0, 127)
	return DataSet(device, AddrMasterVolume, byte(volume))
}

// SetMasterPan returns an SC-55 SysEx command that sets the master pan
// on the SC-55 to a value in the range 1..127.
func SetMasterPan(device DeviceID, pan int) []byte {
	pan = clamp(pan, -63, 63) + 64
	return DataSet(device, AddrMasterPan, byte(pan))
}

// SetMasterTune returns an SC-55 SysEx command that sets the master tuning
// to a value in the range -1000 - +1000.
func SetMasterTune(device DeviceID, tune int) []byte {
	tune = clamp(tune, -1000, 1000)
	tune = tune + 1000 + 0x18
	return DataSet(device, AddrMasterTune,
		byte(tune & 0xff),
		byte((tune >> 8) & 0xff),
		0,
		0,
	)
}

// SetMasterKeyShift returns an SC-55 SysEx command that sets the master key
// shift to a value in the range -24 - +24 semitones.
func SetMasterKeyShift(device DeviceID, keyShift int) []byte {
	keyShift = clamp(keyShift, -24, 24)
	return DataSet(device, AddrMasterKeyShift, byte(keyShift+0x40))
}

