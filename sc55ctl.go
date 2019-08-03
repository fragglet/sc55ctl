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

func deviceID() sc55.DeviceID {
	return sc55.DeviceID(sc55DeviceID)
}

// outPortForName returns the device ID of the port with the given name.
func outPortForName(name string) (portmidi.DeviceID, error) {
	portNames := []string{}
	for i := 0; i < portmidi.CountDevices(); i++ {
		id := portmidi.DeviceID(i)
		info := portmidi.Info(id)
		if !info.IsOutputAvailable {
			continue
		}
		if info.Name == name {
			return id, nil
		}
		portNames = append(portNames, fmt.Sprintf("%q", info.Name))
	}
	return portmidi.DeviceID(-1), fmt.Errorf("invalid port %q: valid ports: %v", name, strings.Join(portNames, "; "))
}

func openPortMidi() (*portmidi.Stream, error) {
	err := portmidi.Initialize()
	if err != nil {
		return nil, err
	}
	id := portmidi.DefaultOutputDeviceID()
	if midiDevice != "" {
		id, err = outPortForName(midiDevice)
		if err != nil {
			return nil, err
		}
	}
	return portmidi.NewOutputStream(id, 1024, 0)
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

type cmd struct {
	name, synopsis string
	minArgs        int
	produceData    func([]string) ([]byte, error)
}

func (c *cmd) Name() string     { return c.name }
func (c *cmd) Synopsis() string { return c.synopsis }
func (*cmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&midiDevice, "midi_device", "", "Name of output MIDI device")
	f.IntVar(&sc55DeviceID, "sc55_device_id", int(sc55.DefaultDevice), "ID of SC-55 device to control")
}
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
	out, err := openPortMidi()
	if err != nil {
		log.Printf("failed to open portmidi: %v", err)
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
	&cmd{
		name: "register-get",
		synopsis: "get the value of a register",
		minArgs: 1,
		produceData: func(args []string) ([]byte, error) {
			r, ok := sc55.RegisterByName(args[0])
			if !ok {

				return nil, fmt.Errorf("unknown register %q", args[0])
			}
			return r.Get(deviceID()), nil
		},
	},
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
	subcommands.Register(subcommands.HelpCommand(), "")
	subcommands.Register(subcommands.CommandsCommand(), "")
	for _, cmd := range commands {
		subcommands.Register(cmd, "")
	}
	ctx := context.Background()
	os.Exit(int(subcommands.Execute(ctx)))
}
