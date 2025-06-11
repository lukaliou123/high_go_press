package pprof

import (
	"context"
	"net/http"
	_ "net/http/pprof" // 导入pprof

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// PprofServer pprof性能分析服务器
type PprofServer struct {
	server *http.Server
	logger *zap.Logger
}

// NewPprofServer 创建pprof服务器
func NewPprofServer(port string, logger *zap.Logger) *PprofServer {
	mux := http.NewServeMux()

	// 注册pprof路由
	mux.HandleFunc("/debug/pprof/", http.DefaultServeMux.ServeHTTP)
	mux.HandleFunc("/debug/pprof/cmdline", http.DefaultServeMux.ServeHTTP)
	mux.HandleFunc("/debug/pprof/profile", http.DefaultServeMux.ServeHTTP)
	mux.HandleFunc("/debug/pprof/symbol", http.DefaultServeMux.ServeHTTP)
	mux.HandleFunc("/debug/pprof/trace", http.DefaultServeMux.ServeHTTP)

	// 添加健康检查
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("pprof server is healthy"))
	})

	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	return &PprofServer{
		server: server,
		logger: logger,
	}
}

// Start 启动pprof服务器
func (p *PprofServer) Start() error {
	p.logger.Info("Starting pprof server", zap.String("addr", p.server.Addr))

	go func() {
		if err := p.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			p.logger.Error("Pprof server failed", zap.Error(err))
		}
	}()

	return nil
}

// Stop 停止pprof服务器
func (p *PprofServer) Stop(ctx context.Context) error {
	p.logger.Info("Stopping pprof server")
	return p.server.Shutdown(ctx)
}

// AddPprofRoutes 为Gin添加pprof路由
func AddPprofRoutes(router *gin.Engine) {
	// 添加pprof路由组
	pprofGroup := router.Group("/debug/pprof")
	{
		pprofGroup.GET("/", gin.WrapH(http.DefaultServeMux))
		pprofGroup.GET("/cmdline", gin.WrapH(http.DefaultServeMux))
		pprofGroup.GET("/profile", gin.WrapH(http.DefaultServeMux))
		pprofGroup.POST("/symbol", gin.WrapH(http.DefaultServeMux))
		pprofGroup.GET("/symbol", gin.WrapH(http.DefaultServeMux))
		pprofGroup.GET("/trace", gin.WrapH(http.DefaultServeMux))
		pprofGroup.GET("/allocs", gin.WrapH(http.DefaultServeMux))
		pprofGroup.GET("/block", gin.WrapH(http.DefaultServeMux))
		pprofGroup.GET("/goroutine", gin.WrapH(http.DefaultServeMux))
		pprofGroup.GET("/heap", gin.WrapH(http.DefaultServeMux))
		pprofGroup.GET("/mutex", gin.WrapH(http.DefaultServeMux))
		pprofGroup.GET("/threadcreate", gin.WrapH(http.DefaultServeMux))
	}
}

// ProfileConfig 性能分析配置
type ProfileConfig struct {
	EnableCPU    bool   `yaml:"enable_cpu"`
	EnableMemory bool   `yaml:"enable_memory"`
	EnableBlock  bool   `yaml:"enable_block"`
	EnableMutex  bool   `yaml:"enable_mutex"`
	Port         string `yaml:"port"`
}

// DefaultProfileConfig 默认配置
func DefaultProfileConfig() *ProfileConfig {
	return &ProfileConfig{
		EnableCPU:    true,
		EnableMemory: true,
		EnableBlock:  true,
		EnableMutex:  true,
		Port:         "6060",
	}
}
