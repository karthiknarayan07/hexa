package cli

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"hexa/internal/platform/bootstrap"
	taskCLI "hexa/internal/task/adapters/in/cli"
)

// NewRootCommand builds the single-entrypoint command tree.
func NewRootCommand() *cobra.Command {
	rootCommand := &cobra.Command{
		Use:   "app",
		Short: "Run transports and manage tasks",
	}

	rootCommand.AddCommand(newRunCommand())
	rootCommand.AddCommand(newTaskCommand())

	return rootCommand
}

func newRunCommand() *cobra.Command {
	var modes []string
	var httpAddress string
	var grpcAddress string

	command := &cobra.Command{
		Use:   "run",
		Short: "Run one or more runtime components",
		RunE: func(command *cobra.Command, _ []string) error {
			selectedModes, err := bootstrap.ParseModes(modes)
			if err != nil {
				return err
			}

			container, err := bootstrap.BuildForServer(bootstrap.BuildOptions{})
			if err != nil {
				return err
			}
			defer container.Close()

			processContext, stopSignals := signal.NotifyContext(command.Context(), os.Interrupt, syscall.SIGTERM)
			defer stopSignals()

			return bootstrap.RunSelected(processContext, container, selectedModes, bootstrap.RuntimeAddresses{
				HTTP: httpAddress,
				GRPC: grpcAddress,
			})
		},
	}

	command.Flags().StringSliceVar(&modes, "run", nil, "Modes to run (http,grpc,worker,cron). Default is all modes")
	command.Flags().StringVar(&httpAddress, "http-addr", ":8080", "HTTP listen address")
	command.Flags().StringVar(&grpcAddress, "grpc-addr", ":9090", "gRPC listen address")

	return command
}

func newTaskCommand() *cobra.Command {
	provider := func() (taskCLI.Services, func() error, error) {
		container, err := bootstrap.BuildForCLI(bootstrap.BuildOptions{})
		if err != nil {
			return taskCLI.Services{}, nil, err
		}

		cleanup := func() error {
			if err := container.Close(); err != nil {
				return fmt.Errorf("close container: %w", err)
			}
			return nil
		}

		services := taskCLI.Services{
			Create:   container.TaskService,
			List:     container.TaskService,
			Get:      container.TaskService,
			Update:   container.TaskService,
			Delete:   container.TaskService,
			Start:    container.TaskService,
			Complete: container.TaskService,
		}

		return services, cleanup, nil
	}

	return taskCLI.NewTaskCommand(provider)
}
