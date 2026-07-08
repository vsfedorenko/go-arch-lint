package application

import "example/ddd/internal/domain/order"

type OrderRepository interface {
	FindByID(id int) (*order.Order, error)
}

type OrderService struct {
	repo OrderRepository
}

func NewOrderService(repo OrderRepository) *OrderService {
	return &OrderService{repo: repo}
}

func (s *OrderService) GetByID(id int) (*order.Order, error) {
	return s.repo.FindByID(id)
}
