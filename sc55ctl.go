// Package main implements a tool for controlling a Roland SC-55.
package main

import (
	"context"
	"flag"
	"fmt"
	"image/png"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/fragglet/sc55ctl/sc55"
	"github.com/google/subcommands"
	"github.com/rakyll/portmidi"
)

const (
	o = false
	X = true
)

var (
	midiDevice   string
	sc55DeviceID int
)

func setCommonFlags(f *flag.FlagSet) {
	f.StringVar(&midiDevice, "midi_device", "", "Name of output MIDI device")
	f.IntVar(&sc55DeviceID, "sc55_device_id", int(sc55.DefaultDevice), "ID of SC-55 device to control")
}
func deviceID() sc55.DeviceID {
	return sc55.DeviceID(sc55DeviceID)
}

// portForName returns the device ID of the port with the given name.
func portForName(name string, output bool) (portmidi.DeviceID, error) {
	portNames := []string{}
	for i := 0; i < portmidi.CountDevices(); i++ {
		id := portmidi.DeviceID(i)
		info := portmidi.Info(id)
		switch {
		case output && !info.IsOutputAvailable:
			continue
		case !output && !info.IsInputAvailable:
			continue
		}
		if info.Name == name {
			return id, nil
		}
		portNames = append(portNames, fmt.Sprintf("%q", info.Name))
	}
	return portmidi.DeviceID(-1), fmt.Errorf("invalid port %q: valid ports: %v", name, strings.Join(portNames, "; "))
}

func openOutputStream() (*portmidi.Stream, error) {
	id := portmidi.DefaultOutputDeviceID()
	if midiDevice != "" {
		var err error
		id, err = portForName(midiDevice, true)
		if err != nil {
			return nil, err
		}
	}
	return portmidi.NewOutputStream(id, 1024, 0)
}

func openInputStream() (*portmidi.Stream, error) {
	id := portmidi.DefaultInputDeviceID()
	if midiDevice != "" {
		var err error
		id, err = portForName(midiDevice, false)
		if err != nil {
			return nil, err
		}
	}
	return portmidi.NewInputStream(id, 1024)
}

type listRegistersCommand struct {}
func (*listRegistersCommand) Name() string     { return "register-list" }
func (*listRegistersCommand) Synopsis() string { return "list all registers on the SoundCanvas" }
func (*listRegistersCommand) SetFlags(f *flag.FlagSet) { }
func (*listRegistersCommand) Usage() string { return "" }
func (*listRegistersCommand) Execute(context.Context, *flag.FlagSet, ...interface{}) subcommands.ExitStatus {
	for _, r := range sc55.AllRegisters() {
		fmt.Printf("% 8x  %s\n", r.Address, r.Name())
	}
	return subcommands.ExitSuccess
}

type getRegisterCommand struct {}
func (*getRegisterCommand) Name() string     { return "register-get" }
func (*getRegisterCommand) Synopsis() string { return "get the value of a register" }
func (*getRegisterCommand) SetFlags(f *flag.FlagSet) { setCommonFlags(f) }
func (*getRegisterCommand) Usage() string { return "" }

func queryRegister(in, out *portmidi.Stream, r *sc55.Register) ([]byte, error) {
	msg := r.Get(deviceID())
	if err := out.WriteSysExBytes(portmidi.Time(), msg); err != nil {
		return nil, err
	}
	for {
		reply, err := in.ReadSysExBytes(1000)
		if err != nil {
			return nil, err
		}
		if len(reply) == 0 {
			time.Sleep(time.Millisecond)
			continue
		}
		for len(reply) > 0 && reply[len(reply) - 1] == 0 {
			reply = reply[:len(reply)-1]
		}
		dev, addr, value, err := sc55.UnmarshalSet(reply)
		if err == nil && dev == deviceID() && addr == r.Address {
			return value, nil
		}
	}
}

func (*getRegisterCommand) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	if len(f.Args()) != 1 {
		log.Printf("register name not supplied")
		return subcommands.ExitUsageError
	}
	regName := f.Args()[0]
	r, ok := sc55.RegisterByName(regName)
	if !ok {
		log.Printf("unknown register %q", regName)
		return subcommands.ExitUsageError
	}
	in, err := openInputStream()
	if err != nil {
		log.Printf("failed to open input stream: %v", err)
		return subcommands.ExitFailure
	}
	out, err := openOutputStream()
	if err != nil {
		log.Printf("failed to open output stream: %v", err)
		return subcommands.ExitFailure
	}
	result, err := queryRegister(in, out, r)
	if err != nil {
		log.Printf("error querying register %q: %v", r.Name(), err)
		return subcommands.ExitFailure
	}
	fmt.Println(result)
	return subcommands.ExitSuccess
}

type cmd struct {
	name, synopsis string
	minArgs        int
	produceData    func([]string) ([]byte, error)
}

func (c *cmd) Name() string     { return c.name }
func (c *cmd) Synopsis() string { return c.synopsis }
func (*cmd) SetFlags(f *flag.FlagSet) { setCommonFlags(f) }
func (c *cmd) Usage() string {
	return fmt.Sprintf("%s [...]:\n%s\n", c.Name(), c.Synopsis())
}

func (c *cmd) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	if len(f.Args()) < c.minArgs {
		log.Printf("parameter not provided for command %q", c.name)
		return subcommands.ExitUsageError
	}
	msg, err := c.produceData(f.Args())
	if err != nil {
		return subcommands.ExitUsageError
	}
	out, err := openOutputStream()
	if err != nil {
		log.Printf("failed to open output stream: %v", err)
		return subcommands.ExitFailure
	}
	if err := out.WriteSysExBytes(portmidi.Time(), msg); err != nil {
		log.Printf("failed to write message to output: %v", err)
		return subcommands.ExitFailure
	}
	return subcommands.ExitSuccess
}

func setParameterCallback(f func(sc55.DeviceID, int) []byte) func([]string) ([]byte, error) {
	return func(args []string) ([]byte, error) {
		val, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return nil, err
		}
		return f(deviceID(), int(val)), nil
	}
}

var commands = []subcommands.Command{
	&cmd{
		name:     "reset-gm",
		synopsis: "Reset the SoundCanvas into General MIDI mode",
		produceData: func([]string) ([]byte, error) {
			return sc55.ResetGM(deviceID()), nil
		},
	},
	&cmd{
		name:     "reset-gs",
		synopsis: "Reset the SoundCanvas into GS mode",
		produceData: func([]string) ([]byte, error) {
			return sc55.ResetGS(deviceID()), nil
		},
	},
	&cmd{
		name:     "display-message",
		synopsis: "Show a message on the SC-55 front panel",
		minArgs:  1,
		produceData: func(args []string) ([]byte, error) {
			msg := strings.Join(args, " ")
			return sc55.DisplayMessage(deviceID(), msg), nil
		},
	},
	&cmd{
		name:     "display-image",
		synopsis: "Show a picture on the SC-55 front panel",
		minArgs: 1,
		produceData: func(args []string) ([]byte, error) {
			in, err := os.Open(args[0])
			if err != nil {
				return nil, err
			}
			defer in.Close()
			img, err := png.Decode(in)
			if err != nil {
				return nil, err
			}
			return sc55.DisplayImage(deviceID(), img)
		},
	},
	&cmd{
		name:        "master-volume",
		synopsis:    "Set the master volume",
		minArgs:     1,
		produceData: setParameterCallback(sc55.MasterVolume.Set),
	},
	&cmd{
		name:        "master-pan",
		synopsis:    "Set the master pan",
		minArgs:     1,
		produceData: setParameterCallback(sc55.MasterPan.Set),
	},
	&cmd{
		name:        "master-tune",
		synopsis:    "Set the master tune",
		minArgs:     1,
		produceData: setParameterCallback(sc55.MasterTune.Set),
	},
	&cmd{
		name:        "master-key-shift",
		synopsis:    "Set the master key shift",
		minArgs:     1,
		produceData: setParameterCallback(sc55.MasterKeyShift.Set),
	},
	&listRegistersCommand{},
	&getRegisterCommand{},
	&cmd{
		name: "register-set",
		synopsis: "set the value of a register",
		minArgs: 2,
		produceData: func(args []string) ([]byte, error) {
			r, ok := sc55.RegisterByName(args[0])
			if !ok {

				return nil, fmt.Errorf("unknown register %q", args[0])
			}
			val, err := strconv.ParseInt(args[1], 10, 32)
			if err != nil {
				return nil, err
			}
			return r.Set(deviceID(), int(val)), nil
		},
	},
}

func main() {
	flag.Parse()
	if err := portmidi.Initialize(); err != nil {
		log.Fatalf("failed to initialize portmidi: %v", err)
	}
	subcommands.Register(subcommands.HelpCommand(), "")
	subcommands.Register(subcommands.CommandsCommand(), "")
	for _, cmd := range commands {
		subcommands.Register(cmd, "")
	}
	ctx := context.Background()
	os.Exit(int(subcommands.Execute(ctx)))
}
