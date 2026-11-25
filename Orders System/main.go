package main

import (
	"orders/auth"
	"orders/db"
	"orders/handlers"

	"github.com/gin-gonic/gin"
)

func main() {

	db.Init(".env")
	defer db.DB.Close()

	auth.Init()
	router := gin.Default()

	router.POST("/login", auth.LoginHandler)
	router.POST("/register", auth.RegisterHandler)

	protected := router.Group("/")
	protected.Use(auth.AuthMiddleware())
	{

		protected.GET("/orders", handlers.GetAll)
		protected.POST("/create", handlers.Create)
		protected.PUT("/update/:id", handlers.Update)

	}

	protected.POST("/products", auth.RequireRole("admin"), handlers.CreateProduct)
	protected.DELETE("/orders/:id", auth.RequireRole("admin"), handlers.Delete)
	protected.POST("/products/:id/stock", auth.RequireRole("admin"), handlers.AddStock)

	router.Run()
}
