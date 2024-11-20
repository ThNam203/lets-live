package main

import (
	"context"
	"fmt"
	"net"

	"sen1or/lets-live/pkg/discovery"
	"sen1or/lets-live/pkg/logger"
	cfg "sen1or/lets-live/user/config"
	"sen1or/lets-live/user/migrations"

	"github.com/jackc/pgx/v5"
)

func main() {
	ctx := context.Background()

	logger.Init()
	config := cfg.RetrieveConfig()
	migrations.StartMigration(config.Database.ConnectionString)

	// for consul service discovery
	go StartDiscovery(ctx, config)

	dbConn := ConnectDB(ctx, config)
	defer dbConn.Close(ctx)

	listenAddr := net.JoinHostPort(config.Service.APIBindAddress, string(config.Service.APIPort))

	server := NewAPIServer(dbConn, listenAddr, *config)
	go server.ListenAndServe(false)
	select {}
}

func ConnectDB(ctx context.Context, config *cfg.Config) *pgx.Conn {
	dbConn, err := pgx.Connect(ctx, config.Database.ConnectionString)
	if err != nil {
		logger.Panicf("unable to connect to database: %v\n", "err", err)
	}

	return dbConn
}

func StartDiscovery(ctx context.Context, config *cfg.Config) {
	registry, err := discovery.NewConsulRegistry(config.Registry.RegistryService.Address)
	if err != nil {
		logger.Panicf("failed to start discovery mechanism: %s", err)
	}

	serviceName := config.Service.Name
	serviceHostPort := fmt.Sprintf("%s:%d", config.Service.Hostname, config.Service.APIPort)
	serviceHealthCheckURL := fmt.Sprintf("http://%s/v1/health", serviceHostPort)
	instanceID := discovery.GenerateInstanceID(serviceName)
	if err := registry.Register(ctx, serviceHostPort, serviceHealthCheckURL, serviceName, instanceID, config.Registry.RegistryService.Tags); err != nil {
		logger.Panicf("failed to register server: %s", err)
	}

	ctx, cancel := context.WithCancel(ctx)

	<-ctx.Done()

	if err := registry.Deregister(ctx, serviceName, instanceID); err != nil {
		logger.Errorf("failed to deregister service: %s", err)
	}

	cancel()
}
