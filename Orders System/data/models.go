package data

import "errors"

type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type Product struct {
	ID    int     `json:"id"`
	Name  string  `json:"name"`
	Price float64 `json:"price"`
	Stock int     `json:"stock"`
}

// Item and Cost are filled in by the server, not sent by the client.
type Order struct {
	ID        int     `json:"id"`
	UserID    int     `json:"user_id"`
	ProductID int     `json:"product_id"`
	Item      string  `json:"item"`
	Cost      float64 `json:"cost"`
}

func (o Order) Validate() error {
	var errs []error

	if o.UserID <= 0 {
		errs = append(errs, errors.New("user_id is required"))
	}
	if o.ProductID <= 0 {
		errs = append(errs, errors.New("product_id is required"))
	}

	return errors.Join(errs...)
}

func (p Product) Validate() error {
	var errs []error

	if p.Name == "" {
		errs = append(errs, errors.New("name is required"))
	}
	if p.Price <= 0 {
		errs = append(errs, errors.New("price must be greater than zero"))
	}
	if p.Stock < 0 {
		errs = append(errs, errors.New("stock cannot be negative"))
	}

	return errors.Join(errs...)
}
