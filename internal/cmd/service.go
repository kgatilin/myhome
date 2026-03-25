package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kgatilin/myhome/internal/config"
	"github.com/kgatilin/myhome/internal/platform"
	"github.com/kgatilin/myhome/internal/service"
)

var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "Manage agent stack services (deskd, agents, adapters)",
}

var serviceStartCmd = &cobra.Command{
	Use:   "start [name]",
	Short: "Start services (all or by name)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, plat, err := loadServiceDeps()
		if err != nil {
			return err
		}
		if len(args) == 0 {
			return service.StartAll(cfg.Services, plat)
		}
		return service.StartOne(args[0], cfg.Services, plat)
	},
}

var serviceStopCmd = &cobra.Command{
	Use:   "stop [name]",
	Short: "Stop services (all or by name)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, plat, err := loadServiceDeps()
		if err != nil {
			return err
		}
		if len(args) == 0 {
			return service.StopAll(cfg.Services, plat)
		}
		return service.StopOne(args[0], cfg.Services, plat)
	},
}

var serviceStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show status of all services",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, plat, err := loadServiceDeps()
		if err != nil {
			return err
		}
		results, err := service.StatusAll(cfg.Services, plat)
		if err != nil {
			return err
		}
		for _, r := range results {
			state := "stopped"
			if r.Running {
				state = "running"
			}
			fmt.Printf("%-20s %s\n", r.Name, state)
		}
		return nil
	},
}

func loadServiceDeps() (*config.Config, platform.Platform, error) {
	cfgPath, err := config.DefaultConfigPath()
	if err != nil {
		return nil, nil, err
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, nil, err
	}
	plat, err := platform.Detect()
	if err != nil {
		return nil, nil, err
	}
	return cfg, plat, nil
}

func init() {
	serviceCmd.AddCommand(serviceStartCmd)
	serviceCmd.AddCommand(serviceStopCmd)
	serviceCmd.AddCommand(serviceStatusCmd)
}
