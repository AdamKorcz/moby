package cgroups

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	systemdDbus "github.com/coreos/go-systemd/dbus"
	"github.com/godbus/dbus"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

const (
	SystemdDbus  Name = "systemd"
	defaultSlice      = "system.slice"
)

func Systemd() ([]Subsystem, error) {
	root, err := v1MountPoint()
	if err != nil {
		return nil, err
	}
	defaultSubsystems, err := defaults(root)
	if err != nil {
		return nil, err
	}
	s, err := NewSystemd(root)
	if err != nil {
		return nil, err
	}
	// make sure the systemd controller is added first
	return append([]Subsystem{s}, defaultSubsystems...), nil
}

func Slice(slice, name string) Path {
	if slice == "" {
		slice = defaultSlice
	}
	return func(subsystem Name) (string, error) {
		return filepath.Join(slice, unitName(name)), nil
	}
}

func NewSystemd(root string) (*SystemdController, error) {
	return &SystemdController{
		root: root,
	}, nil
}

type SystemdController struct {
	mu   sync.Mutex
	root string
}

func (s *SystemdController) Name() Name {
	return SystemdDbus
}

func (s *SystemdController) Create(path string, resources *specs.LinuxResources) error {
	conn, err := systemdDbus.New()
	if err != nil {
		return err
	}
	defer conn.Close()
	slice, name := splitName(path)
	properties := []systemdDbus.Property{
		systemdDbus.PropDescription(fmt.Sprintf("cgroup %s", name)),
		systemdDbus.PropWants(slice),
		newProperty("DefaultDependencies", false),
		newProperty("Delegate", true),
		newProperty("MemoryAccounting", true),
		newProperty("CPUAccounting", true),
		newProperty("BlockIOAccounting", true),
	}
	ch := make(chan string)
	_, err = conn.StartTransientUnit(name, "replace", properties, ch)
	if err != nil {
		return err
	}
	<-ch
	return nil
}

func (s *SystemdController) Delete(path string) error {
	conn, err := systemdDbus.New()
	if err != nil {
		return err
	}
	defer conn.Close()
	_, name := splitName(path)
	ch := make(chan string)
	_, err = conn.StopUnit(name, "replace", ch)
	if err != nil {
		return err
	}
	<-ch
	return nil
}

func newProperty(name string, units interface{}) systemdDbus.Property {
	return systemdDbus.Property{
		Name:  name,
		Value: dbus.MakeVariant(units),
	}
}

func unitName(name string) string {
	return fmt.Sprintf("%s.slice", name)
}

func splitName(path string) (slice string, unit string) {
	slice, unit = filepath.Split(path)
	return strings.TrimSuffix(slice, "/"), unit
}
