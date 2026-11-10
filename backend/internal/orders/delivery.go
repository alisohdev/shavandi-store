package orders

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/shavandi/store/backend/internal/auth"
)

type Delivery struct {
	ShippingMethodID *string `json:"shipping_method_id"`
	ShippingVersion  *int64  `json:"shipping_version"`
	ShippingName     *string `json:"shipping_name"`
	ShippingFeeToman *int64  `json:"shipping_fee_toman"`
	Recipient        string  `json:"recipient"`
	Phone            string  `json:"phone"`
	Province         string  `json:"province"`
	City             string  `json:"city"`
	Address          string  `json:"address"`
	PostalCode       string  `json:"postal_code"`
	Version          int64   `json:"version"`
}

func NormalizeDelivery(input Delivery) (Delivery, error) {
	input.Recipient = strings.TrimSpace(input.Recipient)
	input.Province = strings.TrimSpace(input.Province)
	input.City = strings.TrimSpace(input.City)
	input.Address = strings.TrimSpace(input.Address)
	for _, field := range []struct {
		value    string
		min, max int
	}{{input.Recipient, 2, 100}, {input.Province, 2, 100}, {input.City, 2, 100}, {input.Address, 10, 500}} {
		if !utf8.ValidString(field.value) || utf8.RuneCountInString(field.value) < field.min || utf8.RuneCountInString(field.value) > field.max {
			return Delivery{}, ErrInvalid
		}
		for _, r := range field.value {
			if unicode.IsControl(r) && r != '\n' {
				return Delivery{}, ErrInvalid
			}
		}
	}
	phone, err := auth.NormalizePhone(input.Phone)
	if err != nil {
		return Delivery{}, ErrInvalid
	}
	input.Phone = phone
	var code strings.Builder
	for _, r := range strings.TrimSpace(input.PostalCode) {
		if r >= '۰' && r <= '۹' {
			r = '0' + r - '۰'
		}
		if r >= '٠' && r <= '٩' {
			r = '0' + r - '٠'
		}
		if r < '0' || r > '9' {
			return Delivery{}, ErrInvalid
		}
		code.WriteRune(r)
	}
	input.PostalCode = code.String()
	if len(input.PostalCode) != 10 || input.Version < 0 {
		return Delivery{}, ErrInvalid
	}
	return input, nil
}
