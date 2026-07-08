package interfaces

import (
	"fmt"

	"example/ddd/internal/application"
)

type Handler struct {
	userSvc  *application.UserService
	orderSvc *application.OrderService
}

func NewHandler(userSvc *application.UserService, orderSvc *application.OrderService) *Handler {
	return &Handler{userSvc: userSvc, orderSvc: orderSvc}
}

func (h *Handler) GetUser(id int) string {
	u, _ := h.userSvc.GetByID(id)
	return fmt.Sprintf("User: %s", u.Name)
}

func (h *Handler) GetOrder(id int) string {
	o, _ := h.orderSvc.GetByID(id)
	return fmt.Sprintf("Order: #%d", o.ID)
}
