package service

import "github.com/VitalyMars/delivery_go/services/order/internal/servicepolicy"

func CanChangeStatus(from, to string) bool { return servicepolicy.CanChangeStatus(from, to) }
