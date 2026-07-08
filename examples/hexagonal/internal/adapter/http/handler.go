package http

import (
	"fmt"

	"example/hexagonal/internal/core"
)

type OrderHandler struct {
	svc  *core.OrderService
	repo core.OrderRepository
}

func NewOrderHandler(svc *core.OrderService, repo core.OrderRepository) *OrderHandler {
	return &OrderHandler{svc: svc, repo: repo}
}

func (h *OrderHandler) CreateOrder() {
	order := h.svc.PlaceOrder()
	h.repo.Save(order)
	fmt.Printf("Created order #%d\n", order.ID)
}
