package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"hexa/internal/task/ports/inbound"
)

// Services lists inbound task use cases consumed by the CLI adapter.
type Services struct {
	Create   inbound.CreateTaskUseCase
	List     inbound.ListTasksUseCase
	Get      inbound.GetTaskUseCase
	Update   inbound.UpdateTaskUseCase
	Delete   inbound.DeleteTaskUseCase
	Start    inbound.StartTaskUseCase
	Complete inbound.CompleteTaskUseCase
}

// Provider returns one scoped set of services plus a cleanup callback.
type Provider func() (Services, func() error, error)

// NewTaskCommand builds the task CLI adapter command tree.
func NewTaskCommand(provider Provider) *cobra.Command {
	taskCommand := &cobra.Command{
		Use:   "task",
		Short: "Manage tasks from the command line",
	}

	taskCommand.AddCommand(newCreateCommand(provider))
	taskCommand.AddCommand(newListCommand(provider))
	taskCommand.AddCommand(newGetCommand(provider))
	taskCommand.AddCommand(newUpdateCommand(provider))
	taskCommand.AddCommand(newDeleteCommand(provider))
	taskCommand.AddCommand(newStartCommand(provider))
	taskCommand.AddCommand(newCompleteCommand(provider))

	return taskCommand
}

func newCreateCommand(provider Provider) *cobra.Command {
	var title string
	var description string

	command := &cobra.Command{
		Use:   "create",
		Short: "Create a task",
		RunE: func(command *cobra.Command, _ []string) error {
			services, cleanup, err := provider()
			if err != nil {
				return err
			}
			defer cleanup()

			task, err := services.Create.CreateTask(command.Context(), inbound.CreateTaskCommand{
				Title:       title,
				Description: description,
			})
			if err != nil {
				return err
			}

			return printJSON(command, task)
		},
	}

	command.Flags().StringVar(&title, "title", "", "Task title")
	command.Flags().StringVar(&description, "description", "", "Task description")
	command.MarkFlagRequired("title")

	return command
}

func newListCommand(provider Provider) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List tasks",
		RunE: func(command *cobra.Command, _ []string) error {
			services, cleanup, err := provider()
			if err != nil {
				return err
			}
			defer cleanup()

			tasks, err := services.List.ListTasks(command.Context())
			if err != nil {
				return err
			}

			return printJSON(command, tasks)
		},
	}
}

func newGetCommand(provider Provider) *cobra.Command {
	var taskID string

	command := &cobra.Command{
		Use:   "get",
		Short: "Get one task",
		RunE: func(command *cobra.Command, _ []string) error {
			services, cleanup, err := provider()
			if err != nil {
				return err
			}
			defer cleanup()

			task, err := services.Get.GetTask(command.Context(), taskID)
			if err != nil {
				return err
			}

			return printJSON(command, task)
		},
	}

	command.Flags().StringVar(&taskID, "id", "", "Task ID")
	command.MarkFlagRequired("id")

	return command
}

func newUpdateCommand(provider Provider) *cobra.Command {
	var taskID string
	var title string
	var description string

	command := &cobra.Command{
		Use:   "update",
		Short: "Update task title/description",
		RunE: func(command *cobra.Command, _ []string) error {
			services, cleanup, err := provider()
			if err != nil {
				return err
			}
			defer cleanup()

			task, err := services.Update.UpdateTask(command.Context(), taskID, inbound.UpdateTaskCommand{
				Title:       title,
				Description: description,
			})
			if err != nil {
				return err
			}

			return printJSON(command, task)
		},
	}

	command.Flags().StringVar(&taskID, "id", "", "Task ID")
	command.Flags().StringVar(&title, "title", "", "Task title")
	command.Flags().StringVar(&description, "description", "", "Task description")
	command.MarkFlagRequired("id")
	command.MarkFlagRequired("title")

	return command
}

func newDeleteCommand(provider Provider) *cobra.Command {
	var taskID string

	command := &cobra.Command{
		Use:   "delete",
		Short: "Delete a task",
		RunE: func(command *cobra.Command, _ []string) error {
			services, cleanup, err := provider()
			if err != nil {
				return err
			}
			defer cleanup()

			if err := services.Delete.DeleteTask(command.Context(), taskID); err != nil {
				return err
			}

			return printJSON(command, map[string]string{"deleted": taskID})
		},
	}

	command.Flags().StringVar(&taskID, "id", "", "Task ID")
	command.MarkFlagRequired("id")

	return command
}

func newStartCommand(provider Provider) *cobra.Command {
	var taskID string

	command := &cobra.Command{
		Use:   "start",
		Short: "Mark a task as in progress",
		RunE: func(command *cobra.Command, _ []string) error {
			services, cleanup, err := provider()
			if err != nil {
				return err
			}
			defer cleanup()

			task, err := services.Start.StartTask(command.Context(), taskID)
			if err != nil {
				return err
			}

			return printJSON(command, task)
		},
	}

	command.Flags().StringVar(&taskID, "id", "", "Task ID")
	command.MarkFlagRequired("id")

	return command
}

func newCompleteCommand(provider Provider) *cobra.Command {
	var taskID string

	command := &cobra.Command{
		Use:   "complete",
		Short: "Mark a task as completed",
		RunE: func(command *cobra.Command, _ []string) error {
			services, cleanup, err := provider()
			if err != nil {
				return err
			}
			defer cleanup()

			task, err := services.Complete.CompleteTask(command.Context(), taskID)
			if err != nil {
				return err
			}

			return printJSON(command, task)
		},
	}

	command.Flags().StringVar(&taskID, "id", "", "Task ID")
	command.MarkFlagRequired("id")

	return command
}

func printJSON(command *cobra.Command, payload any) error {
	encoder := json.NewEncoder(command.OutOrStdout())
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(payload); err != nil {
		return fmt.Errorf("encode output: %w", err)
	}

	return nil
}
