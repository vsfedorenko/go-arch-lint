package infrastructure

import "example/ddd/internal/domain/order"

type OrderRepository struct{}

func NewOrderRepository() *OrderRepository {
	return &OrderRepository{}
}

func (r *OrderRepository) FindByID(id int) (*order.Order, error) {
	return &order.Order{ID: id, UserID: 1}, nil
}
