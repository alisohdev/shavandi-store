package orders

import (
	"strings"
	"testing"
)

func TestNormalizeDelivery(t *testing.T) {
	input := Delivery{Recipient: " گیرنده آزمایشی ", Phone: "۰۹۱۲۳۴۵۶۷۸۹", Province: " تهران ", City: " تهران ", Address: " خیابان آزمایشی پلاک ۱۲ ", PostalCode: "۱۲۳۴۵۶۷۸۹۰"}
	value, err := NormalizeDelivery(input)
	if err != nil || value.Phone != "+989123456789" || value.PostalCode != "1234567890" || value.Recipient != "گیرنده آزمایشی" {
		t.Fatalf("normalization failed: %v", err)
	}
	for _, mutate := range []func(*Delivery){func(d *Delivery) { d.Recipient = " " }, func(d *Delivery) { d.Phone = "123" }, func(d *Delivery) { d.Province = "" }, func(d *Delivery) { d.City = "" }, func(d *Delivery) { d.Address = "short" }, func(d *Delivery) { d.Address = strings.Repeat("ا", 501) }, func(d *Delivery) { d.PostalCode = "123" }, func(d *Delivery) { d.Recipient = "name\x00" }, func(d *Delivery) { d.Version = -1 }} {
		invalid := input
		mutate(&invalid)
		if _, err = NormalizeDelivery(invalid); err != ErrInvalid {
			t.Fatal("invalid delivery accepted")
		}
	}
}
