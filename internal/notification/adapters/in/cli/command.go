package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"hexa/internal/notification/ports/inbound"
)

// Services lists notification read use cases consumed by the CLI adapter.
type Services struct {
	List   inbound.ListNotificationsUseCase
	Get    inbound.GetNotificationUseCase
	Delete inbound.DeleteNotificationUseCase
}

// Provider returns one scoped set of services plus cleanup callback.
type Provider func() (Services, func() error, error)

// NewNotificationCommand builds the notification CLI command tree.
func NewNotificationCommand(provider Provider) *cobra.Command {
	notificationCommand := &cobra.Command{
		Use:   "notification",
		Short: "Manage notifications from the command line",
	}

	notificationCommand.AddCommand(newListCommand(provider))
	notificationCommand.AddCommand(newGetCommand(provider))
	notificationCommand.AddCommand(newDeleteCommand(provider))
	return notificationCommand
}

func newListCommand(provider Provider) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List notifications",
		RunE: func(command *cobra.Command, _ []string) error {
			services, cleanup, err := provider()
			if err != nil {
				return err
			}
			defer cleanup()

			notifications, err := services.List.ListNotifications(command.Context())
			if err != nil {
				return err
			}

			return printJSON(command, notifications)
		},
	}
}

func newGetCommand(provider Provider) *cobra.Command {
	var notificationID string

	command := &cobra.Command{
		Use:   "get",
		Short: "Get one notification",
		RunE: func(command *cobra.Command, _ []string) error {
			services, cleanup, err := provider()
			if err != nil {
				return err
			}
			defer cleanup()

			notification, err := services.Get.GetNotification(command.Context(), notificationID)
			if err != nil {
				return err
			}

			return printJSON(command, notification)
		},
	}

	command.Flags().StringVar(&notificationID, "id", "", "Notification ID")
	command.MarkFlagRequired("id")

	return command
}

func newDeleteCommand(provider Provider) *cobra.Command {
	var notificationID string

	command := &cobra.Command{
		Use:   "delete",
		Short: "Delete one notification",
		RunE: func(command *cobra.Command, _ []string) error {
			services, cleanup, err := provider()
			if err != nil {
				return err
			}
			defer cleanup()

			if err := services.Delete.DeleteNotification(command.Context(), notificationID); err != nil {
				return err
			}

			fmt.Fprintln(command.OutOrStdout(), "Notification deleted successfully")
			return nil
		},
	}

	command.Flags().StringVar(&notificationID, "id", "", "Notification ID")
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
