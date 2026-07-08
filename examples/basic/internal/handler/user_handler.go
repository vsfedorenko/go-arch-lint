package handler

import (
	"fmt"

	"example/basic/internal/service"
)

type UserHandler struct {
	svc *service.UserService
}

func NewUserHandler(svc *service.UserService) *UserHandler {
	return &UserHandler{svc: svc}
}

func (h *UserHandler) Handle() {
	user, _ := h.svc.GetUser(1)
	fmt.Printf("User: %s (%s)\n", user.Name, user.Email)
}
