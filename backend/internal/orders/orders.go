package orders

import (
	"errors"
	"time"
)

var ErrConflict = errors.New("order quote or reservation conflict")
var ErrInvalid = errors.New("invalid order request")
var ErrNotFound = errors.New("order not found")

type Line struct {
	VariantID      string  `json:"variant_id"`
	Name           string  `json:"name"`
	Size           *string `json:"size"`
	Color          *string `json:"color"`
	Quantity       int     `json:"quantity"`
	UnitPriceToman int64   `json:"unit_price_toman"`
}
type Order struct {
	ID            string    `json:"id"`
	Status        string    `json:"status"`
	SubtotalToman int64     `json:"subtotal_toman"`
	CreatedAt     time.Time `json:"created_at"`
	ExpiresAt     time.Time `json:"expires_at"`
	Lines         []Line    `json:"lines"`
}
