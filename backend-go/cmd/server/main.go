package main

import (
	"log"
	"strings"

	"ftoz/internal/handler"

	"github.com/gin-gonic/gin"
)

func main() {
	gin.SetMode(gin.ReleaseMode)

	r := gin.Default()

	// CORS 中间件
	r.Use(corsMiddleware())

	// 创建处理器
	h := handler.New()

	// 路由注册
	r.POST("/migrate", h.Migrate)
	r.GET("/status", h.Status)
	r.GET("/dir", h.Dir)
	r.POST("/dir", h.Dir)
	r.GET("/read", h.Read)
	r.POST("/save", h.Save)

	// 通用分发路由 (通过 api-path 头或 _api 参数)
	r.Any("/*path", h.Dispatch)

	log.Println("Server running on :17746")
	if err := r.Run(":17746"); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == "" {
			origin = "*"
		}

		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, api-path")
		c.Header("Access-Control-Allow-Credentials", "true")

		if strings.ToUpper(c.Request.Method) == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
