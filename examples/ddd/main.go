package main

import (
	"fmt"

	"example/ddd/internal/application"
	"example/ddd/internal/infrastructure"
	"example/ddd/internal/interfaces"
)

func main() {
	userRepo := infrastructure.NewUserRepository()
	orderRepo := infrastructure.NewOrderRepository()
	userSvc := application.NewUserService(userRepo)
	orderSvc := application.NewOrderService(orderRepo)
	handler := interfaces.NewHandler(userSvc, orderSvc)

	fmt.Println(handler.GetUser(1))
	fmt.Println(handler.GetOrder(10))
}
