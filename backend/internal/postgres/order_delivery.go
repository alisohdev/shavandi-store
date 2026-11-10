package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"

	"github.com/shavandi/store/backend/internal/orders"
)

// Delivery details belong to one order owner and never renew a stock reservation.
func (s *OrderStore) SaveDelivery(ctx context.Context, userID, id string, input orders.Delivery) (orders.Order, error) {
	if !validOrderID(id) {
		return orders.Order{}, orders.ErrNotFound
	}
	delivery, err := orders.NormalizeDelivery(input)
	if err != nil {
		return orders.Order{}, err
	}
	if err = s.Expire(ctx); err != nil {
		return orders.Order{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return orders.Order{}, err
	}
	defer tx.Rollback(ctx)
	if err = lockCatalogMutations(ctx, tx); err != nil {
		return orders.Order{}, err
	}
	order, err := readOrder(ctx, tx, userID, id)
	if err != nil {
		return order, err
	}
	if order.Status != "pending_confirmation" {
		return order, orders.ErrConflict
	}
	var unexpired bool
	if err = tx.QueryRow(ctx, `SELECT expires_at>clock_timestamp() FROM shavandi.orders WHERE id=$1::uuid FOR UPDATE`, id).Scan(&unexpired); err != nil {
		return order, err
	}
	if !unexpired {
		return order, orders.ErrConflict
	}
	current := int64(0)
	if order.Delivery != nil {
		current = order.Delivery.Version
	}
	if current != delivery.Version {
		return order, orders.ErrConflict
	}

	delivery.ShippingFeeToman = nil
	delivery.ShippingName = nil
	if delivery.ShippingMethodID != nil {
		if !validOrderID(*delivery.ShippingMethodID) || delivery.ShippingVersion == nil {
			return order, orders.ErrInvalid
		}
		var name string
		var fee, shippingVersion int64
		err = tx.QueryRow(ctx, `SELECT name,fee_toman,version FROM shavandi.shipping_methods WHERE id=$1::uuid AND active`, *delivery.ShippingMethodID).Scan(&name, &fee, &shippingVersion)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return order, err
		}
		if errors.Is(err, pgx.ErrNoRows) || shippingVersion != *delivery.ShippingVersion {
			return order, orders.ErrConflict
		}
		if fee > 9007199254740991-order.SubtotalToman {
			return order, orders.ErrInvalid
		}
		delivery.ShippingName = &name
		delivery.ShippingFeeToman = &fee
	} else {
		var configured bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM shavandi.shipping_methods WHERE active)`).Scan(&configured); err != nil {
			return order, err
		}
		if configured || delivery.ShippingVersion != nil {
			return order, orders.ErrInvalid
		}
	}
	delivery.Version = current + 1
	// An uncertain-response retry with the same expected version cannot silently overwrite a newer address.
	_, err = tx.Exec(ctx, `INSERT INTO shavandi.order_deliveries(order_id,recipient,phone,province,city,address,postal_code,version) VALUES($1::uuid,$2,$3,$4,$5,$6,$7,$8)
 ON CONFLICT(order_id) DO UPDATE SET recipient=excluded.recipient,phone=excluded.phone,province=excluded.province,city=excluded.city,address=excluded.address,postal_code=excluded.postal_code,version=excluded.version,updated_at=now()`, id, delivery.Recipient, delivery.Phone, delivery.Province, delivery.City, delivery.Address, delivery.PostalCode, delivery.Version)
	if err != nil {
		return order, err
	}
	_, err = tx.Exec(ctx, `UPDATE shavandi.order_deliveries SET shipping_method_id=$2::uuid,shipping_version=$3,shipping_name=$4,shipping_fee_toman=$5 WHERE order_id=$1::uuid`, id, delivery.ShippingMethodID, delivery.ShippingVersion, delivery.ShippingName, delivery.ShippingFeeToman)
	if err != nil {
		return order, err
	}
	order.Delivery = &delivery
	err = tx.Commit(ctx)
	return order, err
}

func deliveryJSON(value []byte) (*orders.Delivery, error) {
	if string(value) == "null" || len(value) == 0 {
		return nil, nil
	}
	var result orders.Delivery
	err := json.Unmarshal(value, &result)
	return &result, err
}
