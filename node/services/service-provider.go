package services

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"reflect"
	"runtime"
	"time"

	dclient "github.com/docker/docker/client"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/rocket-pool/node-manager-core/beacon/client"
	"github.com/rocket-pool/node-manager-core/config"
	"github.com/rocket-pool/node-manager-core/eth"
	"github.com/rocket-pool/node-manager-core/log"
	"github.com/rocket-pool/node-manager-core/node/wallet"
)

const (
	DockerApiVersion string = "1.40"
)

// ==================
// === Interfaces ===
// ==================

// Provides access to Ethereum client(s) via a fallback-enabled manager, along with utilities for querying the chain and executing transactions
type IEthClientProvider interface {
	// Gets the Execution Client manager
	GetEthClient() *ExecutionClientManager

	// Gets the Execution layer query manager
	GetQueryManager() *eth.QueryManager

	// Gets the Execution layer transaction manager
	GetTransactionManager() *eth.TransactionManager
}

// Provides access to Beacon client(s) via a fallback-enabled manager
type IBeaconClientProvider interface {
	// Gets the Beacon Client manager
	GetBeaconClient() *BeaconClientManager
}

// Provides access to a Docker client
type IDockerProvider interface {
	// Gets the Docker client
	GetDocker() dclient.APIClient
}

// Provides access to the node's loggers
type ILoggerProvider interface {
	// Gets the logger to use for the API server
	GetApiLogger() *log.Logger

	// Gets the logger to use for the automated tasks loop
	GetTasksLogger() *log.Logger
}

// Provides access to the node's wallet
type IWalletProvider interface {
	// Gets the node's wallet
	GetWallet() *wallet.Wallet
}

// Provides access to a context for cancelling long operations upon daemon shutdown
type IContextProvider interface {
	// Gets a base context for the daemon that all operations can derive from
	GetBaseContext() context.Context

	// Cancels the base context when the daemon is shutting down
	CancelContextOnShutdown()
}

// A container for all of the various services used by the node daemon
type IServiceProvider interface {
	IEthClientProvider
	IBeaconClientProvider
	IDockerProvider
	ILoggerProvider
	IWalletProvider
	IContextProvider
	io.Closer
}

// =======================
// === ServiceProvider ===
// =======================

// Options for configuring the service provider
type ServiceProviderOptions struct {
	ExecutionClientManager *ExecutionClientManager
	BeaconClientManager    *BeaconClientManager
	DockerClient           dclient.APIClient
	TxManager              *eth.TransactionManager
	QueryManager           *eth.QueryManager
	ApiLogger              *log.Logger
	TasksLogger            *log.Logger
	LoggerOpts             *log.LoggerOptions
	NodeWallet             *wallet.Wallet
}

// A container for all of the various services used by the node service
type serviceProvider struct {
	// Services
	nodeWallet *wallet.Wallet
	ecManager  *ExecutionClientManager
	bcManager  *BeaconClientManager
	docker     dclient.APIClient
	txMgr      *eth.TransactionManager
	queryMgr   *eth.QueryManager

	// Context for cancelling long operations
	ctx    context.Context
	cancel context.CancelFunc

	// Logging
	apiLogger   *log.Logger
	tasksLogger *log.Logger
}

// Creates a new ServiceProvider instance based on the given config, with the provided optional custom service implemnentations.
// Any services not provided will be created with default implementations.
// clientTimeout is only used to create the default implementations of the ExecutionClientManager and BeaconClientManager if they are not provided.
func NewServiceProvider(cfg config.IConfig, resources *config.NetworkResources, clientTimeout time.Duration, opts ServiceProviderOptions) (IServiceProvider, error) {
	// EC Manager
	if opts.ExecutionClientManager == nil {
		primaryEcUrl, fallbackEcUrl := cfg.GetExecutionClientUrls()
		primaryEc, err := ethclient.Dial(primaryEcUrl)
		if err != nil {
			return nil, fmt.Errorf("error connecting to primary EC at [%s]: %w", primaryEcUrl, err)
		}
		if fallbackEcUrl != "" {
			// Get the fallback EC url, if applicable
			fallbackEc, err := ethclient.Dial(fallbackEcUrl)
			if err != nil {
				return nil, fmt.Errorf("error connecting to fallback EC at [%s]: %w", fallbackEcUrl, err)
			}
			opts.ExecutionClientManager = NewExecutionClientManagerWithFallback(primaryEc, fallbackEc, resources.ChainID, clientTimeout)
		} else {
			opts.ExecutionClientManager = NewExecutionClientManager(primaryEc, resources.ChainID, clientTimeout)
		}

	}

	// Beacon manager
	if opts.BeaconClientManager == nil {
		primaryBnUrl, fallbackBnUrl := cfg.GetBeaconNodeUrls()
		primaryBc := client.NewStandardHttpClient(primaryBnUrl, clientTimeout)
		if fallbackBnUrl != "" {
			fallbackBc := client.NewStandardHttpClient(fallbackBnUrl, clientTimeout)
			opts.BeaconClientManager = NewBeaconClientManagerWithFallback(primaryBc, fallbackBc, resources.ChainID, clientTimeout)
		} else {
			opts.BeaconClientManager = NewBeaconClientManager(primaryBc, resources.ChainID, clientTimeout)
		}
	}

	// Make the API logger
	if opts.LoggerOpts == nil {
		loggerOpts := cfg.GetLoggerOptions()
		opts.LoggerOpts = &loggerOpts
	}
	if opts.ApiLogger == nil {
		apiLogger, err := log.NewLogger(cfg.GetApiLogFilePath(), *opts.LoggerOpts)
		if err != nil {
			return nil, fmt.Errorf("error creating API logger: %w", err)
		}
		opts.ApiLogger = apiLogger
	}

	// Make the tasks logger
	if opts.TasksLogger == nil {
		tasksLogger, err := log.NewLogger(cfg.GetTasksLogFilePath(), *opts.LoggerOpts)
		if err != nil {
			return nil, fmt.Errorf("error creating tasks logger: %w", err)
		}
		opts.TasksLogger = tasksLogger
	}

	// Docker client
	if opts.DockerClient == nil ||
		(reflect.ValueOf(opts.DockerClient).Kind() == reflect.Ptr && reflect.ValueOf(opts.DockerClient).IsNil()) {
		dockerClient, err := dclient.NewClientWithOpts(dclient.WithVersion(DockerApiVersion))
		if err != nil {
			return nil, fmt.Errorf("error creating Docker client: %w", err)
		}
		opts.DockerClient = dockerClient
	}

	// Wallet
	if opts.NodeWallet == nil {
		nodeAddressPath := filepath.Join(cfg.GetNodeAddressFilePath())
		walletDataPath := filepath.Join(cfg.GetWalletFilePath())
		passwordPath := filepath.Join(cfg.GetPasswordFilePath())
		nodeWallet, err := wallet.NewWallet(opts.TasksLogger.Logger, walletDataPath, nodeAddressPath, passwordPath, resources.ChainID)
		if err != nil {
			return nil, fmt.Errorf("error creating node wallet: %w", err)
		}
		opts.NodeWallet = nodeWallet
	}

	// TX Manager
	if opts.TxManager == nil {
		txMgr, err := eth.NewTransactionManager(opts.ExecutionClientManager, eth.DefaultSafeGasBuffer, eth.DefaultSafeGasMultiplier)
		if err != nil {
			return nil, fmt.Errorf("error creating transaction manager: %w", err)
		}
		opts.TxManager = txMgr
	}

	// Query Manager
	if opts.QueryManager == nil {
		// Set the default concurrent run limit to half the CPUs so the EC doesn't get overwhelmed
		concurrentCallLimit := runtime.NumCPU() / 2
		if concurrentCallLimit < 1 {
			concurrentCallLimit = 1
		}
		queryMgr := eth.NewQueryManager(opts.ExecutionClientManager, resources.MulticallAddress, concurrentCallLimit)
		opts.QueryManager = queryMgr
	}

	// Context for handling task cancellation during shutdown
	ctx, cancel := context.WithCancel(context.Background())

	// Log startup
	opts.ApiLogger.Info("Starting API logger.")
	opts.TasksLogger.Info("Starting Tasks logger.")

	// Create the provider
	provider := &serviceProvider{
		nodeWallet:  opts.NodeWallet,
		ecManager:   opts.ExecutionClientManager,
		bcManager:   opts.BeaconClientManager,
		docker:      opts.DockerClient,
		txMgr:       opts.TxManager,
		queryMgr:    opts.QueryManager,
		ctx:         ctx,
		cancel:      cancel,
		apiLogger:   opts.ApiLogger,
		tasksLogger: opts.ApiLogger,
	}
	return provider, nil
}

// Closes the service provider and its underlying services
func (p *serviceProvider) Close() error {
	p.apiLogger.Close()
	p.tasksLogger.Close()
	return nil
}

// ===============
// === Getters ===
// ===============

func (p *serviceProvider) GetWallet() *wallet.Wallet {
	return p.nodeWallet
}

func (p *serviceProvider) GetEthClient() *ExecutionClientManager {
	return p.ecManager
}

func (p *serviceProvider) GetBeaconClient() *BeaconClientManager {
	return p.bcManager
}

func (p *serviceProvider) GetDocker() dclient.APIClient {
	return p.docker
}

func (p *serviceProvider) GetTransactionManager() *eth.TransactionManager {
	return p.txMgr
}

func (p *serviceProvider) GetQueryManager() *eth.QueryManager {
	return p.queryMgr
}

func (p *serviceProvider) GetApiLogger() *log.Logger {
	return p.apiLogger
}

func (p *serviceProvider) GetTasksLogger() *log.Logger {
	return p.tasksLogger
}

func (p *serviceProvider) GetBaseContext() context.Context {
	return p.ctx
}

func (p *serviceProvider) CancelContextOnShutdown() {
	p.cancel()
}
