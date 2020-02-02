package server

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/luoyunpeng/go-fastdfs/internal/config"
)

// Start start the monitor api server
func Start(conf *config.Config) {
	app := gin.Default()
	//app.Use(cors)

	registerRoutes(app, conf)
	srv := &http.Server{
		Addr:              conf.Port(),
		Handler:           app,
		ReadTimeout:       time.Duration(conf.ReadTimeout()) * time.Second,
		ReadHeaderTimeout: time.Duration(conf.ReadHeaderTimeout()) * time.Second,
		WriteTimeout:      time.Duration(conf.WriteTimeout()) * time.Second,
		IdleTimeout:       time.Duration(conf.IdleTimeout()) * time.Second,
	}

	go func() {
		// service connections
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	signalListen(srv, conf)
}

func cors(c *gin.Context) {
	whiteList := map[string]int{
		// TODO
	}

	// request header
	origin := c.Request.Header.Get("Origin")
	if _, ok := whiteList[origin]; ok {
		c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
		// allow to access the origin
		c.Header("Access-Control-Allow-Origin", origin)
		//all method that server supports, in case of to many pre-checking
		c.Header("Access-Control-Allow-Methods", "POST, GET, PUT, DELETE")
		//  header type
		c.Header("Access-Control-Allow-Headers", "Authorization, Content-Length, X-CSRF-Token, Token,session,X_Requested_With,Accept, Origin, Host, Connection, Accept-Encoding, Accept-Language,DNT, X-CustomHeader, Keep-Alive, User-Agent, X-Requested-With, If-Modified-Since, Cache-Control, Content-Type, Pragma")
		// allow across origin setting return other sub fields
		c.Header("Access-Control-Expose-Headers", "Content-Length, Access-Control-Allow-Origin, Access-Control-Allow-Headers,Cache-Control,Content-Language,Content-Type,Expires,Last-Modified,Pragma,FooBar")
		c.Header("Access-Control-Max-Age", "172800")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Set("content-type", "application/json")
	} else if !ok && origin != "" {
		log.Println("forbid access from origin: ", origin)
	}

	// handle request
	c.Next()
}

func signalListen(srv *http.Server, conf *config.Config) {
	// Wait for interrupt signal to gracefully shutdown the server with
	// a timeout of 5 seconds.
	quit := make(chan os.Signal)
	signal.Notify(quit, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	log.Printf("receive %v, Exit", <-quit)
	log.Println("**** Graceful shutdown monitor server ****")

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	conf.RegisterExit()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("File Server shutdown:", err)
	}
	log.Println("**** File server exiting **** ")

}
