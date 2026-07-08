package db

import (
	"example/hexagonal/internal/core"
	"example/hexagonal/internal/domain"
)

type OrderRepository struct{}

func NewOrderRepository() *OrderRepository {
	return &OrderRepository{}
}

func (r *OrderRepository) Save(order *domain.Order) error {
	return nil
}

var _ core.OrderRepository = (*OrderRepository)(nil)
