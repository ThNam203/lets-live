package main

import (
	"context"
	"fmt"

	// TODO: add swagger
	//_ "sen1or/lets-live/auth/docs"

	cfg "sen1or/lets-live/auth/config"
	"sen1or/lets-live/auth/utils"
	"sen1or/lets-live/pkg/discovery"
	"sen1or/lets-live/pkg/logger"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	ctx := context.Background()

	logger.Init(logger.LogLevel(logger.Debug))
	config := cfg.RetrieveConfig()
	utils.StartMigration(config.Database.ConnectionString, config.Database.MigrationPath)

	// for consul service discovery
	registry, err := discovery.NewConsulRegistry(config.Registry.RegistryService.Address)
	if err != nil {
		logger.Panicf("failed to start discovery mechanism: %s", err)
	}
	go RegisterToDiscoveryService(ctx, registry, config)

	dbConn := ConnectDB(ctx, config)
	defer dbConn.Close()

	server := NewAPIServer(dbConn, registry, *config)
	go server.ListenAndServe(false)
	select {}
}

func ConnectDB(ctx context.Context, config *cfg.Config) *pgxpool.Pool {
	dbConn, err := pgxpool.New(ctx, config.Database.ConnectionString)
	if err != nil {
		logger.Panicf("unable to connect to database: %v\n", "err", err)
	}

	return dbConn
}

func RegisterToDiscoveryService(ctx context.Context, registry discovery.Registry, config *cfg.Config) {
	serviceName := config.Service.Name
	serviceHostPort := fmt.Sprintf("%s:%d", config.Service.Hostname, config.Service.APIPort)
	serviceHealthCheckURL := fmt.Sprintf("http://%s/v1/auth/health", serviceHostPort)
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
