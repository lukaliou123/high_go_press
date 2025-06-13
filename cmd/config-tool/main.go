package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"high-go-press/pkg/config"
	"high-go-press/pkg/logger"

	"go.uber.org/zap"
)

var (
	consulAddr  = flag.String("consul", "localhost:8500", "Consul address")
	service     = flag.String("service", "", "Service name")
	environment = flag.String("env", "dev", "Environment")
	configFile  = flag.String("config", "", "Config file path")
	action      = flag.String("action", "get", "Action: get, put, delete, list, history, watch")
	version     = flag.String("version", "", "Config version for rollback")
)

func main() {
	flag.Parse()

	if *service == "" {
		fmt.Println("Service name is required")
		flag.Usage()
		os.Exit(1)
	}

	// 初始化日志
	logger, err := logger.NewLogger("info", "console")
	if err != nil {
		fmt.Printf("Failed to create logger: %v\n", err)
		os.Exit(1)
	}

	// 创建配置中心
	configCenter, err := config.NewConsulConfigCenter(*consulAddr, logger)
	if err != nil {
		logger.Fatal("Failed to create config center", zap.Error(err))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	switch *action {
	case "get":
		handleGet(ctx, configCenter, logger)
	case "put":
		handlePut(ctx, configCenter, logger)
	case "delete":
		handleDelete(ctx, configCenter, logger)
	case "list":
		handleList(ctx, configCenter, logger)
	case "history":
		handleHistory(ctx, configCenter, logger)
	case "watch":
		handleWatch(ctx, configCenter, logger)
	default:
		fmt.Printf("Unknown action: %s\n", *action)
		flag.Usage()
		os.Exit(1)
	}
}

func handleGet(ctx context.Context, configCenter *config.ConsulConfigCenter, logger *zap.Logger) {
	cfg, err := configCenter.GetConfig(ctx, *service, *environment)
	if err != nil {
		logger.Fatal("Failed to get config", zap.Error(err))
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		logger.Fatal("Failed to marshal config", zap.Error(err))
	}

	fmt.Println(string(data))
}

func handlePut(ctx context.Context, configCenter *config.ConsulConfigCenter, logger *zap.Logger) {
	if *configFile == "" {
		logger.Fatal("Config file is required for put action")
	}

	// 从文件加载配置
	manager := config.NewManager(logger)
	cfg, err := manager.Load(*configFile)
	if err != nil {
		logger.Fatal("Failed to load config from file", zap.Error(err))
	}

	// 推送到配置中心
	err = configCenter.PutConfig(ctx, *service, *environment, cfg)
	if err != nil {
		logger.Fatal("Failed to put config", zap.Error(err))
	}

	fmt.Printf("Config pushed successfully for service %s in environment %s\n", *service, *environment)
}

func handleDelete(ctx context.Context, configCenter *config.ConsulConfigCenter, logger *zap.Logger) {
	err := configCenter.DeleteConfig(ctx, *service, *environment)
	if err != nil {
		logger.Fatal("Failed to delete config", zap.Error(err))
	}

	fmt.Printf("Config deleted successfully for service %s in environment %s\n", *service, *environment)
}

func handleList(ctx context.Context, configCenter *config.ConsulConfigCenter, logger *zap.Logger) {
	// 这里实现列出所有服务的配置
	fmt.Printf("Listing configs is not implemented yet\n")
}

func handleHistory(ctx context.Context, configCenter *config.ConsulConfigCenter, logger *zap.Logger) {
	versions, err := configCenter.GetConfigHistory(ctx, *service, *environment)
	if err != nil {
		logger.Fatal("Failed to get config history", zap.Error(err))
	}

	fmt.Printf("Config history for service %s in environment %s:\n", *service, *environment)
	for _, version := range versions {
		fmt.Printf("  Version: %s, Timestamp: %s, Comment: %s\n",
			version.Version, version.Timestamp.Format(time.RFC3339), version.Comment)
	}
}

func handleWatch(ctx context.Context, configCenter *config.ConsulConfigCenter, logger *zap.Logger) {
	fmt.Printf("Watching config changes for service %s in environment %s...\n", *service, *environment)

	callback := func(oldConfig, newConfig *config.Config) error {
		fmt.Printf("\n=== Config Changed ===\n")
		fmt.Printf("Timestamp: %s\n", time.Now().Format(time.RFC3339))

		if oldConfig == nil {
			fmt.Printf("Event: Config Created\n")
		} else if newConfig == nil {
			fmt.Printf("Event: Config Deleted\n")
		} else {
			fmt.Printf("Event: Config Updated\n")
		}

		if newConfig != nil {
			data, _ := json.MarshalIndent(newConfig, "", "  ")
			fmt.Printf("New Config:\n%s\n", string(data))
		}

		return nil
	}

	err := configCenter.WatchConfig(ctx, *service, *environment, callback)
	if err != nil {
		logger.Fatal("Failed to watch config", zap.Error(err))
	}

	// 保持运行直到收到中断信号
	select {
	case <-ctx.Done():
		fmt.Println("Watch cancelled")
	}
}
