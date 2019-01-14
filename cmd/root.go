package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/Azure/azure-amqp-common-go/conn"
	"github.com/Azure/go-autorest/autorest/azure"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.PersistentFlags().StringVar(&namespace, "namespace", "", "namespace of the Service Bus")
	rootCmd.PersistentFlags().StringVar(&entityPath, "sb", "", "path to Service Bus entity")
	rootCmd.PersistentFlags().StringVar(&sasKeyName, "key-name", "", "SAS key name")
	rootCmd.PersistentFlags().StringVar(&sasKey, "key", "", "SAS key")
	rootCmd.PersistentFlags().StringVar(&connStr, "conn-str", "", "Connection string for Service Bus")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "debug level logging")
	log.SetFormatter(&log.TextFormatter{TimestampFormat: "2006-01-02 15:04:05", FullTimestamp: true})
}

var (
	namespace, suffix, entityPath, sasKeyName, sasKey, connStr string
	debug                                                      bool

	rootCmd = &cobra.Command{
		Use:              "testbus",
		Short:            "testbus is a simple command line testing tool for the Service Bus library",
		TraverseChildren: true,
	}
)

// Execute kicks off the command line
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func checkAuthFlags() error {
	if connStr != "" {
		parsed, err := conn.ParsedConnectionFromStr(connStr)
		if err != nil {
			return err
		}
		namespace = parsed.Namespace
		entityPath = parsed.HubName
		suffix = parsed.Suffix
		sasKeyName = parsed.KeyName
		sasKey = parsed.Key
		return nil
	}

	if namespace == "" {
		return errors.New("namespace is required")
	}

	if entityPath == "" {
		return errors.New("entityPath is required")
	}

	if sasKey == "" {
		return errors.New("key is required")
	}

	if sasKeyName == "" {
		return errors.New("key-name is required")
	}

	if connStr == "" {
		connStr = fmt.Sprintf("Endpoint=sb://%s.servicebus.windows.net/;SharedAccessKeyName=%s;SharedAccessKey=%s;EntityPath=%s", namespace, sasKeyName, sasKey, entityPath)
	}
	return nil
}

func environment() azure.Environment {
	env := azure.PublicCloud
	if suffix != "" {
		env.ServiceBusEndpointSuffix = suffix
	}
	return env
}
