package simple_network

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
	"net/http"
)

var server *http.Server = nil

var upgrader = &websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type AcceptCallback = func(session *websocket.Conn)

func Startup(addr string, onConnect AcceptCallback) {
	engine := gin.Default()

	// 以release模式运行gin，请将下面代码的注释去掉
	//gin.SetMode(gin.ReleaseMode)

	engine.GET("/", func(context *gin.Context) {
		conn, err := upgrader.Upgrade(context.Writer, context.Request, nil) // error ignored for sake of simplicity

		if err != nil {
			log.Error("websocket upgrade failed:", err.Error())
			return
		}

		onConnect(conn)
	})

	//创建HTTP服务器
	server = &http.Server{
		Addr:    addr,
		Handler: engine,
	}

	//启动HTTP服务器
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("http server listen error:", err)
		}
	}()
}

func StartupWss(addr string, onConnect AcceptCallback, key string, pem string) {

	http.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		conn, err := upgrader.Upgrade(writer, request, nil)
		if err != nil {
			log.Error("websocket upgrade failed:", err.Error())
			return
		}

		onConnect(conn)
	})

	// 使用 HTTPS 启动 Web 服务器，并处理 WSS 协议
	go func() {
		err := http.ListenAndServeTLS(addr, pem, key, nil)
		if err != nil {
			log.Error("ListenAndServeTLS failed:", err)
		}
	}()
}

func Stop(ctx context.Context) {
	//停止HTTP服务器
	if err := server.Shutdown(ctx); err != nil {
		log.Error("http server Shutdown error:", err)
	}

	log.Info("http server exiting")
}

func Join() {
	// todo:需要添加等待http服务器退出的机制
}
