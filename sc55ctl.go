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

func onlyImportant(regs []*sc55.Register) []*sc55.Register {
	important := []*sc55.Register{}
	for _, r := range regs {
		if r.Important() {
			important = append(important, r)
		}
	}
	return important
}

type listRegistersCommand struct {
	all bool
}

func (*listRegistersCommand) Name() string     { return "register-list" }
func (*listRegistersCommand) Synopsis() string { return "list all registers on the SoundCanvas" }
func (*listRegistersCommand) Usage() string    { return "" }

func (c *listRegistersCommand) SetFlags(f *flag.FlagSet) {
	f.BoolVar(&c.all, "all", false, "list all registers")
}

func (c *listRegistersCommand) Execute(context.Context, *flag.FlagSet, ...interface{}) subcommands.ExitStatus {
	regs := sc55.AllRegisters()
	if !c.all {
		regs = onlyImportant(regs)
	}
	for _, r := range regs {
		fmt.Printf("% 8x  %s\n", r.Address, r.Name())
	}
	return subcommands.ExitSuccess
}

type getRegisterCommand struct {
	timeout time.Duration
	all     bool
}

func (*getRegisterCommand) Name() string     { return "register-get" }
func (*getRegisterCommand) Synopsis() string { return "get the value of a register" }
func (*getRegisterCommand) Usage() string    { return "" }

func (c *getRegisterCommand) SetFlags(f *flag.FlagSet) {
	setCommonFlags(f)
	f.DurationVar(&c.timeout, "timeout", 100*time.Millisecond, "how long to wait for a reply from the SoundCanvas before timing out")
	f.BoolVar(&c.all, "all", false, "fetch values of all registers")
}

func (c *getRegisterCommand) queryRegister(in, out *portmidi.Stream, r *sc55.Register) (int, error) {
	msg := r.Get(deviceID())
	if err := out.WriteSysExBytes(portmidi.Time(), msg); err != nil {
		return 0, err
	}
	timeoutTime := time.Now().Add(c.timeout)
	for {
		reply, err := in.ReadSysExBytes(1000)
		if err != nil {
			return 0, err
		}
		if len(reply) == 0 {
			if time.Now().After(timeoutTime) {
				return 0, fmt.Errorf("timeout waiting for reply fetching register %q value", r.Name())
			}
			time.Sleep(time.Millisecond)
			continue
		}
		for len(reply) > 0 && reply[len(reply)-1] == 0 {
			reply = reply[:len(reply)-1]
		}
		dev, value, err := r.Unmarshal(reply)
		if err == nil && dev == deviceID() {
			return value, nil
		}
	}
}

func (c *getRegisterCommand) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	var registers []*sc55.Register
	if len(f.Args()) > 0 {
		regName := f.Args()[0]
		r, ok := sc55.RegisterByName(regName)
		if !ok {
			log.Printf("unknown register %q", regName)
			return subcommands.ExitUsageError
		}
		registers = append(registers, r)
	} else {
		registers = sc55.AllRegisters()
		if !c.all {
			registers = onlyImportant(registers)
		}
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
	result := subcommands.ExitSuccess
	for _, r := range registers {
		value, err := c.queryRegister(in, out, r)
		if err != nil {
			log.Printf("error querying register %q: %v", r.Name(), err)
			result = subcommands.ExitFailure
			continue
		}
		fmt.Printf("%10x  %32s  %d\n", r.Address, r.Name(), value)
	}
	return result
}

type cmd struct {
	name, synopsis string
	minArgs        int
	produceData    func([]string) ([]byte, error)
}

func (c *cmd) Name() string           { return c.name }
func (c *cmd) Synopsis() string       { return c.synopsis }
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
		minArgs:  1,
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
	&listRegistersCommand{},
	&getRegisterCommand{},
	&cmd{
		name:     "register-set",
		synopsis: "set the value of a register",
		minArgs:  2,
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
