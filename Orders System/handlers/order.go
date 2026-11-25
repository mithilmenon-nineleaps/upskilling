package handlers

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lib/pq"

	"orders/data"
	"orders/db"
	"orders/notify"
)

func Create(ctx *gin.Context) {
	var newOrder data.Order

	if err := ctx.ShouldBindJSON(&newOrder); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := newOrder.Validate(); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := db.DB.QueryRowContext(
		ctx.Request.Context(),
		`WITH reserved AS (
		UPDATE products SET stock = stock - 1
		WHERE id = $2 AND stock > 0
		RETURNING id, name, price
	),
	new_order AS (
		INSERT INTO orders (user_id, product_id, cost)
		SELECT $1, id, price FROM reserved
		RETURNING id, user_id, product_id, cost
	)
	SELECT n.id, n.user_id, n.product_id, r.name, n.cost
	FROM new_order n
	JOIN reserved r ON r.id = n.product_id`,
		newOrder.UserID, newOrder.ProductID,
	).Scan(&newOrder.ID, &newOrder.UserID, &newOrder.ProductID, &newOrder.Item, &newOrder.Cost)

	if errors.Is(err, sql.ErrNoRows) {
		// Either the product doesn't exist, or it has no stock left.
		var stock int
		lookupErr := db.DB.QueryRowContext(
			ctx.Request.Context(),
			`SELECT stock FROM products WHERE id = $1`,
			newOrder.ProductID,
		).Scan(&stock)

		if errors.Is(lookupErr, sql.ErrNoRows) {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "product does not exist"})
			return
		}
		ctx.JSON(http.StatusConflict, gin.H{"error": "product is out of stock"})
		return
	}
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23503" {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "user does not exist"})
			return
		}
		log.Printf("insert order failed: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "could not create order"})
		return
	}

	go func(UserID int, item string, cost float64) {
		bg, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		var email string

		err := db.DB.QueryRowContext(ctx, `SELECT email FROM users WHERE id=$1`, UserID).Scan(&email)
		if err != nil {
			log.Printf("email lookup failed: %v", err)
			return
		}

		if err = notify.SendOrderConfirmation(bg, email, item, cost); err != nil {
			log.Printf("Could not send email: %v", err)
		}

	}(newOrder.UserID, newOrder.Item, newOrder.Cost)

	ctx.JSON(http.StatusCreated, newOrder)
}
func GetAll(ctx *gin.Context) {
	rows, err := db.DB.QueryContext(
		ctx.Request.Context(),
		`SELECT o.id, o.user_id, o.product_id, p.name, o.cost
	 FROM orders o
	 JOIN products p ON p.id = o.product_id
	 ORDER BY o.id`,
	)

	if err != nil {
		log.Printf("Query Order failed:%v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Could not fetch order"})
		return
	}
	defer rows.Close()
	orders := []data.Order{}

	for rows.Next() {
		var o data.Order
		if err := rows.Scan(&o.ID, &o.UserID, &o.ProductID, &o.Item, &o.Cost); err != nil {
			log.Printf("scan order failed: %v", err)
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "could not read orders"})
			return
		}
		orders = append(orders, o)
	}
	if err := rows.Err(); err != nil {
		log.Printf("iterating orders failed: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "could not fetch orders"})
		return
	}

	ctx.JSON(http.StatusOK, orders)
}

func Delete(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))

	if err != nil || id <= 0 {
		log.Printf("invalid ID: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid Order ID"})
		return
	}

	result, err := db.DB.ExecContext(
		ctx.Request.Context(),
		`WITH deleted AS (
		DELETE FROM orders WHERE id = $1 RETURNING product_id
	)
	UPDATE products SET stock = stock + 1
	WHERE id IN (SELECT product_id FROM deleted)`,
		id,
	)

	if err != nil {
		log.Printf("delete order failed: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "could not delete order"})
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("rows affected failed: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "could not delete order"})
		return
	}

	if rowsAffected == 0 {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Order deleted"})
}

func Update(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || id <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid order id"})
		return
	}

	var input data.Order
	if err := ctx.ShouldBindJSON(&input); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := input.Validate(); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var updated data.Order

	err = db.DB.QueryRowContext(
		ctx.Request.Context(),
		`WITH updated AS (
		UPDATE orders o
		SET user_id = $1, product_id = p.id, cost = p.price
		FROM products p
		WHERE o.id = $3 AND p.id = $2
		RETURNING o.id, o.user_id, o.product_id, o.cost
	)
	SELECT u.id, u.user_id, u.product_id, p.name, u.cost
	FROM updated u
	JOIN products p ON p.id = u.product_id`,
		input.UserID, input.ProductID, id,
	).Scan(&updated.ID, &updated.UserID, &updated.ProductID, &updated.Item, &updated.Cost)

	if err == sql.ErrNoRows {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
		return
	}
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23503" {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "user does not exist"})
			return
		}

		log.Printf("update order failed: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "could not update order"})
		return
	}

	ctx.JSON(http.StatusOK, updated)
}
