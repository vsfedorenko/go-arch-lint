package core

import "example/hexagonal/internal/domain"

type OrderRepository interface {
	Save(order *domain.Order) error
}

type OrderService struct{}

func NewOrderService() *OrderService {
	return &OrderService{}
}

func (s *OrderService) PlaceOrder() *domain.Order {
	return &domain.Order{ID: 1, Status: "pending"}
}
