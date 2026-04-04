package domain

// ServiceConfig defines a managed service from .wtm.services.toml.
type ServiceConfig struct {
	Name string `toml:"name"`
	Cmd  string `toml:"cmd"`
	Stop string `toml:"stop"`
	Cwd  string `toml:"cwd"`
}

// ProfileConfig defines a named group of services.
type ProfileConfig struct {
	Name     string   `toml:"name"`
	Services []string `toml:"services"`
	Default  bool     `toml:"default"`
}

// ServicesConfig is the top-level structure of .wtm.services.toml.
type ServicesConfig struct {
	Services []ServiceConfig `toml:"services"`
	Profiles []ProfileConfig `toml:"profiles"`
}

// ServiceStatus represents the current state of a managed service.
type ServiceStatus string

const (
	ServiceStatusRunning ServiceStatus = "running"
	ServiceStatusStopped ServiceStatus = "stopped"
	ServiceStatusCrashed ServiceStatus = "crashed"
)

// ServicesFileName is the services config file name.
const ServicesFileName = ".wtm.services.toml"

// DefaultProfile returns the profile marked as default, or the first one.
func (c ServicesConfig) DefaultProfile() (ProfileConfig, bool) {
	for _, p := range c.Profiles {
		if p.Default {
			return p, true
		}
	}
	if len(c.Profiles) > 0 {
		return c.Profiles[0], true
	}
	return ProfileConfig{}, false
}

// FindProfile returns a profile by name.
func (c ServicesConfig) FindProfile(name string) (ProfileConfig, bool) {
	for _, p := range c.Profiles {
		if p.Name == name {
			return p, true
		}
	}
	return ProfileConfig{}, false
}

// FindService returns a service by name.
func (c ServicesConfig) FindService(name string) (ServiceConfig, bool) {
	for _, s := range c.Services {
		if s.Name == name {
			return s, true
		}
	}
	return ServiceConfig{}, false
}

// ProfileServices returns the ServiceConfig list for a profile.
func (c ServicesConfig) ProfileServices(profile ProfileConfig) []ServiceConfig {
	var services []ServiceConfig
	for _, name := range profile.Services {
		if svc, ok := c.FindService(name); ok {
			services = append(services, svc)
		}
	}
	return services
}
