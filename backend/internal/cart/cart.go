package cart

import "errors"

var ErrInvalid = errors.New("invalid cart input")
var ErrUnavailable = errors.New("cart variant unavailable")
var ErrLimit = errors.New("cart line limit reached")

type Line struct {
	AvailableQuantity int     `json:"available_quantity"`
	VariantID         string  `json:"variant_id"`
	ProductSlug       string  `json:"product_slug"`
	Name              string  `json:"name"`
	Size              *string `json:"size"`
	Color             *string `json:"color"`
	Quantity          int     `json:"quantity"`
	Available         bool    `json:"available"`
	UnitPriceToman    *int64  `json:"unit_price_toman"`
	LineTotalToman    *int64  `json:"line_total_toman"`
}
type Snapshot struct {
	Lines          []Line `json:"lines"`
	SubtotalToman  int64  `json:"subtotal_toman"`
	HasUnavailable bool   `json:"has_unavailable"`
}
