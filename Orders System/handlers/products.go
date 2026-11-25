package handlers

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lib/pq"

	"orders/data"
	"orders/db"
)

func CreateProduct(ctx *gin.Context) {
	var p data.Product
	if err := ctx.ShouldBindJSON(&p); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := p.Validate(); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	err := db.DB.QueryRowContext(
		ctx.Request.Context(),
		`INSERT INTO products (name, price, stock) VALUES ($1, $2, $3) RETURNING id`,
		p.Name, p.Price, p.Stock,
	).Scan(&p.ID)

	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			ctx.JSON(http.StatusConflict, gin.H{"error": "product already exists"})
			return
		}
		log.Printf("insert product failed: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "could not create product"})
		return
	}

	ctx.JSON(http.StatusCreated, p)
}

func GetAllProducts(ctx *gin.Context) {
	rows, err := db.DB.QueryContext(
		ctx.Request.Context(),
		`SELECT id, name, price, stock FROM products ORDER BY id`,
	)
	if err != nil {
		log.Printf("query products failed: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "could not fetch products"})
		return
	}
	defer rows.Close()

	products := []data.Product{}
	for rows.Next() {
		var p data.Product
		if err := rows.Scan(&p.ID, &p.Name, &p.Price, &p.Stock); err != nil {
			log.Printf("scan product failed: %v", err)
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "could not read products"})
			return
		}
		products = append(products, p)
	}
	if err := rows.Err(); err != nil {
		log.Printf("iterating products failed: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "could not fetch products"})
		return
	}

	ctx.JSON(http.StatusOK, products)
}

func AddStock(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || id <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid product id"})
		return
	}

	var req struct {
		Quantity int `json:"quantity"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil || req.Quantity <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "quantity must be greater than zero"})
		return
	}

	var p data.Product
	err = db.DB.QueryRowContext(
		ctx.Request.Context(),
		`UPDATE products SET stock = stock + $1 WHERE id = $2
		 RETURNING id, name, price, stock`,
		req.Quantity, id,
	).Scan(&p.ID, &p.Name, &p.Price, &p.Stock)

	if errors.Is(err, sql.ErrNoRows) {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
		return
	}
	if err != nil {
		log.Printf("add stock failed: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "could not update stock"})
		return
	}

	ctx.JSON(http.StatusOK, p)
}
