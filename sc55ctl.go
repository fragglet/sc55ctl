// Package main implements a tool for controlling a Roland SC-55.
package main

import (
	"context"
	"flag"
	"fmt"
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
	midiDevice = flag.String("midi_device", "", "Name of output MIDI device")
	sc55DeviceID = flag.Int("sc55_device_id", sc55.DefaultDevice, "ID of SC-55 device to control")
)

var bitmap = [16][16]bool{
	{ o, X, o, X, o, o, o, o, o, X, o, X, o, o, o, o },
	{ o, X, o, X, o, o, X, o, o, X, o, X, o, o, X, o },
	{ o, X, X, X, o, X, o, X, o, X, o, X, o, X, o, X },
	{ o, X, o, X, o, X, X, o, o, X, o, X, o, X, o, X },
	{ o, X, o, X, o, o, X, X, o, X, o, X, o, o, X, o },
	{ o, o, o, o, o, o, o, o, o, o, o, o, o, o, o, o },
	{ X, o, X, o, o, o, o, o, o, o, o, X, o, o, o, X },
	{ X, o, X, o, o, X, o, o, o, X, o, X, o, o, o, X },
	{ X, X, X, o, X, o, X, o, X, o, o, X, o, o, X, X },
	{ X, X, X, o, X, o, X, o, X, o, o, X, o, X, o, X },
	{ X, o, X, o, o, X, o, o, X, o, o, X, o, X, X, X },
	{ o, o, o, o, o, o, o, o, o, o, o, o, o, o, o, o },
	{ o, o, X, o, o, o, X, o, o, o, X, o, o, o, o, o },
	{ o, o, o, o, X, o, o, o, o, o, o, X, o, o, o, o },
	{ o, o, X, o, o, o, X, o, o, X, X, X, o, o, o, o },
	{ o, o, o, X, X, X, o, o, o, o, o, o, o, o, o, o },
}

func deviceID() sc55.DeviceID {
	return sc55.DeviceID(*sc55DeviceID)
}

func openPortMidi() (*portmidi.Stream, error) {
	if err := portmidi.Initialize(); err != nil {
		return nil, err
	}
	for id := 0; id < portmidi.CountDevices(); id++ {
		fmt.Println(portmidi.Info(portmidi.DeviceID(id)))
	}
	id := portmidi.DeviceID(2)
	return portmidi.NewOutputStream(id, 1024, 0)
}

type cmd struct {
	name, synopsis string
	produceData func ([]string) ([]byte, error)
}

func (c *cmd) Name() string     { return c.name }
func (c *cmd) Synopsis() string { return c.synopsis }
func (*cmd) SetFlags(f *flag.FlagSet) {}
func (c *cmd) Usage() string {
	return fmt.Sprintf("%s [...]:\n%s\n", c.Name(), c.Synopsis())
}

func (c *cmd) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	if len(f.Args()) < 1 {
		log.Printf("parameter not provided for command %q", c.name)
		return subcommands.ExitFailure
	}
	msg, err := c.produceData(f.Args())
	if err != nil {
		return subcommands.ExitFailure
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

func setParameterCallback(f func(sc55.DeviceID, int) []byte) func ([]string) ([]byte, error) {
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
		name: "display-message",
		synopsis: "Show a message on the SC-55 front panel",
		produceData: func(args []string) ([]byte, error) {
			msg := strings.Join(args, " ")
			return sc55.DisplayMessage(deviceID(), msg), nil
		},
	},
	&cmd{
		name: "display-image",
		synopsis: "Show a picture on the SC-55 front panel",
		produceData: func(args []string) ([]byte, error) {
			return sc55.DisplayImage(deviceID(), bitmap), nil
		},
	},
	&cmd{
		name: "master-volume",
		synopsis: "Set the master volume",
		produceData: setParameterCallback(sc55.SetMasterVolume),
	},
	&cmd{
		name: "master-pan",
		synopsis: "Set the master pan",
		produceData: setParameterCallback(sc55.SetMasterPan),
	},
	&cmd{
		name: "master-tune",
		synopsis: "Set the master tune",
		produceData: setParameterCallback(sc55.SetMasterTune),
	},
	&cmd{
		name: "master-key-shift",
		synopsis: "Set the master key shift",
		produceData: setParameterCallback(sc55.SetMasterKeyShift),
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

