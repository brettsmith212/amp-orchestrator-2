package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/brettsmith212/amp-orchestrator-2/internal/worker"
	"github.com/spf13/cobra"
)

func old() {
	var rootCmd = &cobra.Command{
		Use:   "amp-orchestrator",
		Short: "Orchestrate amp CLI instances",
		Long:  "A tool to manage and orchestrate multiple amp CLI worker instances",
	}

	// Add commands
	rootCmd.AddCommand(startCmd())
	rootCmd.AddCommand(stopCmd())
	rootCmd.AddCommand(continueCmd())
	rootCmd.AddCommand(listCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func startCmd() *cobra.Command {
	var message string
	var logDir string

	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start a new amp worker instance",
		RunE: func(cmd *cobra.Command, args []string) error {
			wm := worker.NewManager(logDir)
			return wm.StartWorker(message)
		},
	}

	cmd.Flags().StringVarP(&message, "message", "m", "", "Initial message for the worker")
	cmd.Flags().StringVarP(&logDir, "log-dir", "l", "./logs", "Directory for log files")
	cmd.MarkFlagRequired("message")

	return cmd
}

func stopCmd() *cobra.Command {
	var workerID string

	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop an amp worker instance",
		RunE: func(cmd *cobra.Command, args []string) error {
			wm := worker.NewManager("")
			return wm.StopWorker(workerID)
		},
	}

	cmd.Flags().StringVarP(&workerID, "worker", "w", "", "Worker ID to stop")
	cmd.MarkFlagRequired("worker")

	return cmd
}

func continueCmd() *cobra.Command {
	var workerID string
	var message string

	cmd := &cobra.Command{
		Use:   "continue",
		Short: "Send a message to an existing amp worker",
		RunE: func(cmd *cobra.Command, args []string) error {
			wm := worker.NewManager("")
			return wm.ContinueWorker(workerID, message)
		},
	}

	cmd.Flags().StringVarP(&workerID, "worker", "w", "", "Worker ID to continue")
	cmd.Flags().StringVarP(&message, "message", "m", "", "Message to send to the worker")
	cmd.MarkFlagRequired("worker")
	cmd.MarkFlagRequired("message")

	return cmd
}

func listCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all active amp workers",
		RunE: func(cmd *cobra.Command, args []string) error {
			wm := worker.NewManager("")
			workers, err := wm.ListWorkers()
			if err != nil {
				return err
			}

			if len(workers) == 0 {
				fmt.Println("No workers found")
				return nil
			}

			fmt.Printf("%-10s %-12s %-8s %-10s %-20s %s\n", "ID", "THREAD", "PID", "STATUS", "STARTED", "LOG")
			fmt.Println(strings.Repeat("-", 90))

			for _, w := range workers {
				fmt.Printf("%-10s %-12s %-8d %-10s %-20s %s\n",
					w.ID,
					w.ThreadID[:12]+"...",
					w.PID,
					w.Status,
					w.Started.Format("2006-01-02 15:04:05"),
					w.LogFile,
				)
			}

			return nil
		},
	}
}
