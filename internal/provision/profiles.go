package provision

import (
	"errors"
	"granger/internal/config"
)

func IssueProfile(cfg *config.Config, userID, output string) error {
	user, exists := cfg.Users[userID]
	if exists == false || user.Disabled {
		return errors.New("active user is required")
	}
	out, exists := cfg.Outputs[output]
	if exists == false || out.IsEnabled() == false {
		return errors.New("active entrypoint is required")
	}
	for _, client := range out.Clients {
		if client.User == userID {
			return errors.New("user already has a profile on this entrypoint")
		}
	}
	primary := user
	user.Output = output
	user.Client = ""
	cfg.Users[userID] = user
	defer func() { cfg.Users[userID] = primary }()
	return IssueUser(cfg, userID)
}

func SetProfileDisabled(cfg *config.Config, output, clientName string, disabled bool) error {
	out, exists := cfg.Outputs[output]
	if exists == false {
		return errors.New("unknown entrypoint")
	}
	out.Clients = append([]config.Client(nil), out.Clients...)
	found := false
	for i := range out.Clients {
		if out.Clients[i].Name == clientName {
			out.Clients[i].Disabled = disabled
			found = true
		}
	}
	if found == false {
		return errors.New("unknown profile")
	}
	cfg.Outputs[output] = out
	return RenderProfiles(*cfg, output)
}

func RenderProfiles(cfg config.Config, name string) error {
	out := cfg.Outputs[name]
	out.Clients = cfg.EnabledClients(out)
	return renderOutput(name, out, cfg.Server.PublicIP)
}

func RuntimeOutput(cfg config.Config, out config.Output) config.Output {
	out.Clients = append([]config.Client(nil), out.Clients...)
	for i := range out.Clients {
		out.Clients[i].Disabled = cfg.ClientEnabled(out.Clients[i]) == false
	}
	return out
}
